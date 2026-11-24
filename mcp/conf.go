package mcp

import (
	"net/http"

	"github.com/czcorpus/cnc-gokit/logging"
)

type ServerConf struct {
	ListenAddress string `json:"listenAddress"`
}

type Conf struct {
	APIUrl     string              `json:"apiUrl"`
	APIHeaders map[string]string   `json:"apiHeaders"`
	Server     ServerConf          `json:"server"`
	Logging    logging.LoggingConf `json:"logging"`

	// HTTPClient is used for API requests (nil = http.DefaultClient).
	// It allows an embedding application to share e.g. a cookie jar
	// with an authenticated session.
	HTTPClient *http.Client `json:"-"`
}

func (conf *Conf) httpClient() *http.Client {
	if conf.HTTPClient != nil {
		return conf.HTTPClient
	}
	return http.DefaultClient
}
