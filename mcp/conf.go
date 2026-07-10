package mcp

type Conf struct {
	APIUrl     string            `json:"apiUrl"`
	APIHeaders map[string]string `json:"apiHeaders"`
}
