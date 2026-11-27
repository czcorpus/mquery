package agent

import (
	"context"
	"fmt"
	"strings"

	"mquery/cmd/mymquery/llm"
)

// ValidateToolName is the CQLizer tool used for automatic validation of
// queries before they are searched (see Session.validateCall)
const ValidateToolName = "cql_validate"

// RejectionInstructions start the part of a rejection message (following
// the reason) which tells the model what to do
const RejectionInstructions = "The query was not run."

// cqlArgs are arguments of the search tools containing CQL queries
var cqlArgs = []string{"q", "coll"}

type validationVerdict int

const (
	// validationUnavailable: no validator or it failed (the query is run)
	validationUnavailable validationVerdict = iota
	validationOK
	// validationWarning: the query is run, the validator has a remark
	validationWarning
	// validationRejected: the query is invalid, it is not run
	validationRejected
)

// AutoValidates tells whether queries of the search tools are validated
// automatically (i.e. cql_validate is available). Then the validation tool
// is not offered to the model.
func (tb *Toolbox) AutoValidates() bool {
	_, ok := tb.helpers[ValidateToolName]
	return ok
}

// ModelDefs returns definitions of the tools offered to the model
func (tb *Toolbox) ModelDefs() []llm.ToolDef {
	if !tb.AutoValidates() {
		return tb.defs
	}
	ans := make([]llm.ToolDef, 0, len(tb.defs))
	for _, d := range tb.defs {
		if d.Name != ValidateToolName {
			ans = append(ans, d)
		}
	}
	return ans
}

// validateCQL validates a query for a corpus via cql_validate
func (tb *Toolbox) validateCQL(ctx context.Context, corpusID, q string) (validationVerdict, string) {
	srv, ok := tb.helpers[ValidateToolName]
	if !ok {
		return validationUnavailable, ""
	}
	text, isError, err := callMCPRaw(ctx, srv.client, ValidateToolName, map[string]any{
		"corpus_id": corpusID, "q": q,
	})
	text = strings.TrimSpace(text)
	switch {
	case err != nil:
		return validationUnavailable, err.Error()
	case isError:
		return validationRejected, text
	// "query OK" or "query syntax OK, but corpus ... is unknown to CQLizer ..."
	// (the latter is not interesting for the model)
	case text == "query OK" || strings.HasPrefix(text, "query syntax OK"):
		return validationOK, text
	default:
		return validationWarning, text
	}
}

// validationKey identifies a query of a tool call (see Session.rejected)
func validationKey(tool, corpusID, arg, q string) string {
	return strings.Join([]string{tool, corpusID, arg, q}, "\x00")
}

// validateCall validates CQL arguments of a search tool call. If a query is
// rejected, it returns a message for the model (and the call must not be run).
// Otherwise, it returns validator's remarks (if any) for the model.
// A query rejected before is not validated again - the model may insist
// on it (the validator may be wrong).
func (s *Session) validateCall(
	ctx context.Context, tool string, args map[string]any,
) (rejection, remarks string) {
	if !s.tools.AutoValidates() || s.tools.IsHelper(tool) {
		return "", ""
	}
	corpusID, _ := args["corpus_id"].(string)
	if corpusID == "" {
		return "", ""
	}
	var notes []string
	for _, arg := range cqlArgs {
		q, _ := args[arg].(string)
		if strings.TrimSpace(q) == "" {
			continue
		}
		key := validationKey(tool, corpusID, arg, q)
		if s.rejected[key] {
			continue
		}
		verdict, msg := s.tools.validateCQL(ctx, corpusID, q)
		switch verdict {
		case validationRejected:
			if s.rejected == nil {
				s.rejected = make(map[string]bool)
			}
			s.rejected[key] = true
			// (the first line is shown in the UI, so it starts with the reason;
			// the whole message - e.g. a list of tagset issues - is for the model)
			return fmt.Sprintf(
				"error: rejected by CQL validation (`%s`): %s\n"+
					RejectionInstructions+" Fix it and call the tool again. If you are sure the query "+
					"is correct (the validator may be wrong), call the tool again with exactly the same "+
					"arguments to run it without validation.", arg, strings.TrimSpace(msg)), ""
		case validationWarning:
			notes = append(notes, fmt.Sprintf("CQL validation of `%s`: %s", arg, msg))
		}
	}
	return "", strings.Join(notes, "\n")
}
