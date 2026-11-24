package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

func TestToGenAIContents(t *testing.T) {
	sig := []byte("signature")
	contents, err := toGenAIContents([]Message{
		{Role: RoleUser, Content: "question"},
		{Role: RoleAssistant, Content: "let me look", ToolCalls: []ToolCall{
			{Name: "a", Args: `{"x":1}`, Opaque: sig},
			{Name: "b", Args: `{}`},
		}},
		{Role: RoleTool, ToolResult: ToolResult{ToolName: "a", Content: "ra"}},
		{Role: RoleTool, ToolResult: ToolResult{ToolName: "b", Content: "rb"}},
		{Role: RoleAssistant, Content: "answer"},
	})
	require.NoError(t, err)
	require.Len(t, contents, 4)

	model := contents[1]
	assert.Equal(t, string(genai.RoleModel), model.Role)
	require.Len(t, model.Parts, 3)
	assert.Equal(t, "let me look", model.Parts[0].Text)
	assert.Equal(t, "a", model.Parts[1].FunctionCall.Name)
	assert.Equal(t, map[string]any{"x": float64(1)}, model.Parts[1].FunctionCall.Args)
	assert.Equal(t, sig, model.Parts[1].ThoughtSignature)
	assert.Nil(t, model.Parts[2].ThoughtSignature)

	// responses to parallel calls are merged
	resps := contents[2]
	assert.Equal(t, string(genai.RoleUser), resps.Role)
	require.Len(t, resps.Parts, 2)
	assert.Equal(t, "a", resps.Parts[0].FunctionResponse.Name)
	assert.Equal(t, map[string]any{"output": "rb"}, resps.Parts[1].FunctionResponse.Response)

	assert.Equal(t, "answer", contents[3].Parts[0].Text)
}
