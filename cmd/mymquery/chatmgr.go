package main

import (
	"context"
	"fmt"
	"strings"

	"mquery/cmd/mymquery/agent"
	"mquery/cmd/mymquery/api"
	"mquery/cmd/mymquery/llm"
	"mquery/cmd/mymquery/storage"
	"mquery/cmd/mymquery/ui"
)

// llmKeyNames are names of the API key variables of the backends
var llmKeyNames = map[string]string{
	llmBackendGemini:  "GEMINI_API_KEY",
	llmBackendMistral: "MISTRAL_API_KEY",
	llmBackendOpenAI:  "OPENAI_API_KEY",
}

// chatBackends describes the backends for the UI
func chatBackends() []ui.ChatBackend {
	return []ui.ChatBackend{
		{ID: llmBackendMistral, Label: "Mistral AI", DefaultModels: defaultLLMModels[llmBackendMistral],
			KeyName: llmKeyNames[llmBackendMistral]},
		{ID: llmBackendGemini, Label: "Google Gemini", DefaultModels: defaultLLMModels[llmBackendGemini],
			KeyName: llmKeyNames[llmBackendGemini]},
		{ID: llmBackendOpenAI, Label: "OpenAI-compatible (llama-server)",
			DefaultModels: "the server's model", KeyName: llmKeyNames[llmBackendOpenAI],
			KeyOptional: true, HasURL: true, DefaultURL: llm.DefaultOpenAIAPIURL},
	}
}

// chatManager creates AI chat sessions - at startup from the configuration
// and later as configured by the user in the UI (a different backend, model
// or API key)
type chatManager struct {
	client  *api.Client
	lang    string
	db      *storage.DB
	llmConf llmConf
	// cqlizerSpec is the CQLizer MCP server ("" = none); it is connected once
	cqlizerSpec   string
	helperEnv     []string
	cqlizer       *agent.ExternalServer
	cqlizerTried  bool
	cqlizerNotice string
	// userConfig is the user's configuration file (for saving the setup)
	userConfig string

	setup llmSetup
	tools *agent.Toolbox
}

// current returns the current backend and models (without the key)
func (cm *chatManager) current() ui.ChatSetup {
	return ui.ChatSetup{
		Backend: cm.setup.backend, Models: strings.Join(cm.setup.models, ","), URL: cm.setup.url}
}

// connect creates a new session (with new tools; the previous ones are closed)
func (cm *chatManager) connect(setup llmSetup) (*agent.Session, string, error) {
	ctx := context.Background()
	backend, err := newLLMBackend(ctx, setup)
	if err != nil {
		return nil, "", err
	}
	tools, err := agent.NewToolbox(ctx, cm.client, cm.lang, "mymquery")
	if err != nil {
		return nil, "", err
	}
	notice := cm.addHelperTools(ctx, tools)
	if cm.tools != nil {
		cm.tools.Close()
	}
	cm.tools = tools
	cm.setup = setup
	sess := agent.NewSession(backend, tools, agent.NewStore())
	if cm.db != nil {
		sess.Recorder = cm.db
	}
	return sess, notice, nil
}

// addHelperTools adds CQLizer's tools (if configured). The chat works
// without them, so a failure is just reported in the returned notice.
func (cm *chatManager) addHelperTools(ctx context.Context, tools *agent.Toolbox) string {
	if cm.cqlizerSpec == "" {
		return ""
	}
	if !cm.cqlizerTried {
		cm.cqlizerTried = true
		srv, err := agent.ConnectMCP(ctx, cm.cqlizerSpec, "mymquery", cm.helperEnv)
		if err != nil {
			cm.cqlizerNotice = "CQLizer is not available (CQL helper tools are disabled): " + err.Error()
		}
		cm.cqlizer = srv
	}
	if cm.cqlizer == nil {
		return cm.cqlizerNotice
	}
	added, err := tools.AddHelperTools(ctx, cm.cqlizer, agent.CQLizerTools)
	switch {
	case err != nil:
		return "CQLizer is not available (CQL helper tools are disabled): " + err.Error()
	case len(added) == 0:
		return "CQLizer provides none of the expected tools (" +
			strings.Join(agent.CQLizerTools, ", ") + "), CQL helper tools are disabled"
	}
	return "CQL helper tools (CQLizer): " + strings.Join(added, ", ")
}

// configure creates a session for a setup chosen in the UI. An empty API key
// means the current (configured) one of the backend. With setup.Save,
// the setup is saved to the user's configuration file.
func (cm *chatManager) configure(setup ui.ChatSetup) (*agent.Session, string, error) {
	conf := cm.llmConf
	conf.backend = setup.Backend
	conf.models = setup.Models
	if setup.Backend == llmBackendOpenAI {
		conf.openaiURL = strings.TrimSpace(setup.URL)
	}
	key := strings.TrimSpace(setup.APIKey)
	if key != "" {
		switch setup.Backend {
		case llmBackendGemini:
			conf.geminiKey = key
		case llmBackendMistral:
			conf.mistralKey = key
		case llmBackendOpenAI:
			conf.openaiKey = key
		}
	}
	resolved, unavailable, err := conf.resolve()
	if err != nil {
		return nil, "", err
	}
	if unavailable != "" {
		return nil, "", fmt.Errorf("an API key (%s) is required", llmKeyNames[setup.Backend])
	}
	sess, notice, err := cm.connect(resolved)
	if err != nil {
		return nil, "", err
	}
	// the key is kept for this run (e.g. for a later change of the model)
	cm.llmConf = conf
	var notices []string
	if setup.Save {
		if msg, err := cm.save(setup, key); err != nil {
			notices = append(notices, "failed to save the AI chat setup: "+err.Error())
		} else {
			notices = append(notices, msg)
		}
	}
	if notice != "" {
		notices = append(notices, notice)
	}
	return sess, strings.Join(notices, "\n"), nil
}

// save writes the setup to the user's configuration file
func (cm *chatManager) save(setup ui.ChatSetup, key string) (string, error) {
	if cm.userConfig == "" {
		return "", fmt.Errorf("unknown configuration directory (set HOME or XDG_CONFIG_HOME)")
	}
	values := map[string]string{"LLM_BACKEND": setup.Backend, "LLM_MODEL": strings.TrimSpace(setup.Models)}
	if key != "" {
		values[llmKeyNames[setup.Backend]] = key
	}
	if setup.Backend == llmBackendOpenAI {
		values["OPENAI_API_URL"] = strings.TrimSpace(setup.URL)
	}
	if err := updateConfigFile(cm.userConfig, values); err != nil {
		return "", err
	}
	return fmt.Sprintf("the AI chat setup was saved to %s (readable only by you)", cm.userConfig), nil
}

// Close closes the tools and helper servers
func (cm *chatManager) Close() {
	if cm.tools != nil {
		cm.tools.Close()
	}
	if cm.cqlizer != nil {
		cm.cqlizer.Close()
	}
}
