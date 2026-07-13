package mcp

type ServerConf struct {
	ListenAddress string `json:"listenAddress"`
}

type Conf struct {
	APIUrl     string            `json:"apiUrl"`
	APIHeaders map[string]string `json:"apiHeaders"`
	Server     ServerConf        `json:"server"`
}
