package mcp

import "github.com/czcorpus/cnc-gokit/logging"

type ServerConf struct {
	ListenAddress string `json:"listenAddress"`
}

type Conf struct {
	APIUrl     string              `json:"apiUrl"`
	APIHeaders map[string]string   `json:"apiHeaders"`
	Server     ServerConf          `json:"server"`
	Logging    logging.LoggingConf `json:"logging"`
}
