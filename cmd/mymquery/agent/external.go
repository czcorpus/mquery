package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	"mquery/cmd/mymquery/llm"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// CQLizerTools are tools of the CQLizer MCP server used by the chat
// (others, e.g. get_corpus_structure duplicating corpus_info or the DQD
// tools for LCP corpora, are not offered to the model)
var CQLizerTools = []string{
	"cql_validate", "build_tag_condition", "tagset_reference", "cql_eval_complexity",
}

const (
	externalConnectTimeout = 15 * time.Second
	// stderrTailLines is the number of the last stderr lines of a stdio
	// MCP server kept for error messages
	stderrTailLines = 5
)

// ExternalServer is a connection to an external MCP server providing
// helper tools (e.g. CQLizer)
type ExternalServer struct {
	// Spec is the server URL or the command line (as configured)
	Spec   string
	client *client.Client
	stderr *tailBuffer
}

// ConnectMCP connects to an MCP server. A spec starting with http:// or
// https:// is an URL of a server with the Streamable HTTP transport, anything
// else is a command line (split on whitespace, no quoting) of a server using
// the stdio transport. Such a server inherits the environment, `env`
// ("NAME=value" items) is added to it.
func ConnectMCP(ctx context.Context, spec, version string, env []string) (*ExternalServer, error) {
	ctx, cancel := context.WithTimeout(ctx, externalConnectTimeout)
	defer cancel()
	srv := &ExternalServer{Spec: spec}
	if strings.HasPrefix(spec, "http://") || strings.HasPrefix(spec, "https://") {
		cl, err := client.NewStreamableHttpClient(spec)
		if err != nil {
			return nil, fmt.Errorf("failed to create MCP client for %s: %w", spec, err)
		}
		if err := cl.Start(ctx); err != nil {
			return nil, fmt.Errorf("failed to connect to %s: %w", spec, err)
		}
		srv.client = cl

	} else {
		fields := strings.Fields(spec)
		if len(fields) == 0 {
			return nil, fmt.Errorf("empty MCP server command")
		}
		cl, err := client.NewStdioMCPClient(fields[0], env, fields[1:]...)
		if err != nil {
			return nil, fmt.Errorf("failed to start %s: %w", fields[0], err)
		}
		srv.client = cl
		// the subprocess would block once the stderr pipe is full
		if r, ok := client.GetStderr(cl); ok {
			srv.stderr = newTailBuffer(stderrTailLines)
			go srv.stderr.consume(r)
		}
	}
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "mymquery", Version: version}
	if _, err := srv.client.Initialize(ctx, initReq); err != nil {
		srv.client.Close()
		return nil, srv.wrapErr(fmt.Errorf("failed to initialize MCP server %s: %w", spec, err))
	}
	return srv, nil
}

// wrapErr adds the last stderr lines of a stdio server (if any) to an error
func (srv *ExternalServer) wrapErr(err error) error {
	if srv.stderr == nil {
		return err
	}
	// a failed process may still be writing its error message
	select {
	case <-srv.stderr.done:
	case <-time.After(200 * time.Millisecond):
	}
	if tail := srv.stderr.String(); tail != "" {
		return fmt.Errorf("%w (stderr: %s)", err, tail)
	}
	return err
}

// Close disconnects from the server (a stdio server process is terminated)
func (srv *ExternalServer) Close() error {
	return srv.client.Close()
}

// AddHelperTools adds tools of an external server (only those in `allow`)
// as helper tools: their outputs are passed to the model, but they are not
// stored as results (they are no data the user would like to see in a table).
// It returns names of the added tools. Tools with names already used are
// skipped.
func (tb *Toolbox) AddHelperTools(ctx context.Context, srv *ExternalServer, allow []string) ([]string, error) {
	list, err := srv.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, srv.wrapErr(fmt.Errorf("failed to list tools of %s: %w", srv.Spec, err))
	}
	var added []string
	for _, t := range list.Tools {
		if !slices.Contains(allow, t.Name) || tb.Has(t.Name) {
			continue
		}
		schema, err := json.Marshal(t.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to encode schema for tool %q: %w", t.Name, err)
		}
		tb.defs = append(tb.defs, llm.ToolDef{Name: t.Name, Description: t.Description, Parameters: schema})
		if tb.helpers == nil {
			tb.helpers = make(map[string]*ExternalServer)
		}
		tb.helpers[t.Name] = srv
		added = append(added, t.Name)
	}
	return added, nil
}

// Has tells whether a tool is available
func (tb *Toolbox) Has(name string) bool {
	for _, d := range tb.defs {
		if d.Name == name {
			return true
		}
	}
	return false
}

// HelperNames returns names of the helper tools
func (tb *Toolbox) HelperNames() []string {
	ans := make([]string, 0, len(tb.helpers))
	for name := range tb.helpers {
		ans = append(ans, name)
	}
	slices.Sort(ans)
	return ans
}

// IsHelper tells whether a tool is a helper tool (see AddHelperTools)
func (tb *Toolbox) IsHelper(name string) bool {
	_, ok := tb.helpers[name]
	return ok
}

// callMCP calls a tool of an MCP server; failures are reported as text
// for the model with ok == false
func callMCP(ctx context.Context, cl *client.Client, name string, args map[string]any) (string, bool) {
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := cl.CallTool(ctx, req)
	if err != nil {
		return fmt.Sprintf("tool call failed: %v", err), false
	}
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	if res.IsError {
		return "error: " + sb.String(), false
	}
	return sb.String(), true
}

// tailBuffer keeps the last lines of a stream
type tailBuffer struct {
	mu    sync.Mutex
	max   int
	lines []string
	// done is closed once the stream ends
	done chan struct{}
}

func newTailBuffer(maxLines int) *tailBuffer {
	return &tailBuffer{max: maxLines, done: make(chan struct{})}
}

func (b *tailBuffer) consume(r io.Reader) {
	defer close(b.done)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 4096), 1024*1024)
	defer io.Copy(io.Discard, r) // (e.g. after a too long line, keep draining)
	for sc.Scan() {
		b.mu.Lock()
		b.lines = append(b.lines, sc.Text())
		if len(b.lines) > b.max {
			b.lines = b.lines[len(b.lines)-b.max:]
		}
		b.mu.Unlock()
	}
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(strings.Join(b.lines, " | "))
}
