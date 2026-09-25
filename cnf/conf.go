// Copyright 2019 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2019 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//   This file is part of MQUERY.
//
//  MQUERY is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  MQUERY is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with MQUERY.  If not, see <https://www.gnu.org/licenses/>.

package cnf

import (
	"encoding/json"
	"fmt"
	"mquery/corpus"
	"mquery/general"
	"mquery/monitoring"
	"mquery/rdb"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/logging"
	"github.com/rs/zerolog/log"
)

const (
	dfltServerWriteTimeoutSecs = 30
	dfltLanguage               = "en"
	dfltMaxNumConcurrentJobs   = 4
	dfltVertMaxNumErrors       = 100
	dfltTimeZone               = "Europe/Prague"
)

type LocaleConf struct {
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

type LocalesConf []LocaleConf

func (conf LocalesConf) SupportsLocale(name string) bool {
	var elms []string
	if strings.Contains(name, "-") {
		elms = strings.Split(name, "-")

	} else if strings.Contains(name, "_") {
		elms = strings.Split(name, "_")

	} else {
		elms = []string{name}
	}
	for _, locConf := range conf {
		if locConf.Name == elms[0] {
			return true
		}
	}
	return false
}

func (conf LocalesConf) DefaultLocale() string {
	for _, v := range conf {
		if v.IsDefault {
			return v.Name
		}
	}
	return "en"
}

type PrivacyPolicy struct {
	LastUpdate string   `json:"lastUpdate"`
	Contents   []string `json:"contents"`
}

type AuthConf struct {
	TokenHeaderName string   `json:"TokenHeaderName"`
	Tokens          []string `json:"tokens"`
	// KnownProxies lists IP addresses of reverse proxies in front of MQuery.
	// Requests originating from these IPs are always subject to auth token
	// checks, even if the IP matches listenAddress (for an exception, see
	// ProxiesSetForwardingHeaders).
	KnownProxies []string `json:"knownProxies"`
	// LocalNetworks lists CIDR ranges (e.g. "192.168.1.0/24") whose traffic
	// is considered local and exempt from auth token checks, provided the
	// source IP is not also listed in knownProxies. If empty, only the
	// exact listenAddress is treated as local.
	LocalNetworks []string `json:"localNetworks"`

	// ProxiesSetForwardingHeaders if true, then a request coming from
	// a known proxy IP is considered as proxied only if it contains one of
	// the X-Forwarded-For, X-Real-IP, Forwarded headers. Otherwise it is
	// considered as a direct request from a client running on the proxy's
	// host (and it is evaluated just like any other direct request).
	// Enable this only if ALL the proxies listed in KnownProxies always set
	// at least one of the headers (e.g. Nginx does not do this by default!).
	// Otherwise, external requests may be considered internal.
	//
	// This is useful e.g. if some internal applications/script querying
	// MQuery are on the same IP as the proxy. In such case it would
	// be otherwise impossible to distinguish between a proxy request and
	// the script.
	ProxiesSetForwardingHeaders bool `json:"proxiesSetForwardingHeaders"`

	// ApplyToAdminActionsOnly if true then only specific "administration"
	// actions are protected by authentication tokens.
	// In case there are no tokens defined (tokenHeaderName, tokens),
	// those actions are blocked completely.
	ApplyToAdminActionsOnly bool `json:"applyToAdminActionsOnly"`
}

func (ac *AuthConf) IsDefined() bool {
	return ac != nil && ac.TokenHeaderName != "" && len(ac.Tokens) > 0
}

// IsLocalNetwork tests whether the provided IP belongs to one of
// configured local networks. If no networks are configured (or there
// is no auth configuration at all), only the listenAddr is considered local.
func (ac *AuthConf) IsLocalNetwork(ip, listenAddr string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	if ac == nil || len(ac.LocalNetworks) == 0 {
		return ip == listenAddr
	}
	for _, cidr := range ac.LocalNetworks {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			log.Error().Err(err).Str("cidr", cidr).Msg("invalid localNetworks entry")
			continue
		}
		if network.Contains(parsed) {
			return true
		}
	}
	return false
}

func (ac *AuthConf) IsKnownProxy(ip string) bool {
	if ac == nil {
		return false
	}
	for _, p := range ac.KnownProxies {
		if p == ip {
			return true
		}
	}
	return false
}

func hasForwardingHeader(req *http.Request) bool {
	return req.Header.Get("X-Forwarded-For") != "" ||
		req.Header.Get("X-Real-IP") != "" ||
		req.Header.Get("Forwarded") != ""
}

// isProxiedRequest tests whether the request comes via a known proxy.
// If the proxies are configured to always set forwarding headers
// (see ProxiesSetForwardingHeaders), a request from a known proxy IP without
// such headers is a direct request from a client on the proxy's host.
func (ac *AuthConf) isProxiedRequest(req *http.Request, remoteIP string) bool {
	if !ac.IsKnownProxy(remoteIP) {
		return false
	}
	return !ac.ProxiesSetForwardingHeaders || hasForwardingHeader(req)
}

// IsInternalRequest tests whether the request comes directly from
// an internal network. Such requests are exempt from auth token checks
// and can access "internal network access only" corpora.
// Requests via known proxies and requests with the general.PublicClientHeader
// set (e.g. from the MCP server) are never considered internal.
func (ac *AuthConf) IsInternalRequest(req *http.Request, listenAddr string) bool {
	if req.Header.Get(general.PublicClientHeader) != "" {
		return false
	}
	remoteIP, _, err := net.SplitHostPort(req.RemoteAddr)
	return err == nil && ac.IsLocalNetwork(remoteIP, listenAddr) && !ac.isProxiedRequest(req, remoteIP)
}

// --------

// Conf is a global configuration of the app
type Conf struct {
	ListenAddress string `json:"listenAddress"`

	// PublicURLs specifies which URLs are used to access the server.
	// For more flexibility, we allow for more public URLs where
	// a concrete variant is inferred based on client's request.
	PublicURLs             []string             `json:"publicUrls"`
	APIDocsURLPath         string               `json:"apiDocsUrlPath"`
	ListenPort             int                  `json:"listenPort"`
	ServerReadTimeoutSecs  int                  `json:"serverReadTimeoutSecs"`
	ServerWriteTimeoutSecs int                  `json:"serverWriteTimeoutSecs"`
	CorsAllowedOrigins     []string             `json:"corsAllowedOrigins"`
	CorporaSetup           *corpus.CorporaSetup `json:"corpora"`
	CQLTranslatorURL       string               `json:"cqlTranslatorURL"`
	Redis                  *rdb.Conf            `json:"redis"`
	Logging                logging.LoggingConf  `json:"logging"`
	Locales                LocalesConf          `json:"locales"`
	TimeZone               string               `json:"timeZone"`
	PrivacyPolicy          PrivacyPolicy        `json:"privacyPolicy"`

	Monitoring *monitoring.Conf `json:"monitoring"`
	Auth       *AuthConf        `json:"auth"`
	srcPath    string
}

func (conf *Conf) LoadSubconfigs() error {
	if conf.CorporaSetup.ConfFilesDir != "" {
		if err := conf.CorporaSetup.Resources.Load(conf.CorporaSetup.ConfFilesDir, conf.CorporaSetup.RegistryDir); err != nil {
			return fmt.Errorf("failed to load subconfig for `corpora`: %w", err)
		}
	}
	return nil
}

func (conf *Conf) TimezoneLocation() *time.Location {
	// we can ignore the error here as we always call c.Validate()
	// first (which also tries to load the location and report possible
	// error)
	loc, _ := time.LoadLocation(conf.TimeZone)
	return loc
}

// GetSourcePath returns an absolute path of a file
// the config was loaded from.
func (conf *Conf) GetSourcePath() string {
	if filepath.IsAbs(conf.srcPath) {
		return conf.srcPath
	}
	var cwd string
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "[failed to get working dir]"
	}
	return filepath.Join(cwd, conf.srcPath)
}

func LoadConfig(path string) *Conf {
	if path == "" {
		log.Fatal().Msg("Cannot load config - path not specified")
	}
	rawData, err := os.ReadFile(path)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot load config")
	}
	var conf Conf
	conf.srcPath = path
	err = json.Unmarshal(rawData, &conf)
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot load config")
	}
	return &conf
}

func ValidateAndDefaults(conf *Conf) {
	if conf.ServerWriteTimeoutSecs == 0 {
		conf.ServerWriteTimeoutSecs = dfltServerWriteTimeoutSecs
		log.Warn().Msgf(
			"serverWriteTimeoutSecs not specified, using default: %d",
			dfltServerWriteTimeoutSecs,
		)
	}
	if len(conf.PublicURLs) == 0 {
		log.Warn().
			Strs("address", conf.PublicURLs).
			Msg("no publicUrls set, using listenAddress")
		conf.PublicURLs = []string{fmt.Sprintf("http://%s", conf.ListenAddress)}
	}
	for _, addr := range conf.PublicURLs {
		if _, err := url.Parse(addr); err != nil {
			log.Fatal().Err(err).Msg("failed to validate publicUrls")
			return
		}
	}

	// check locales conf.
	if len(conf.Locales) == 0 {
		conf.Locales = []LocaleConf{{
			Name:      dfltLanguage,
			IsDefault: true,
		}}
		log.Warn().Msgf("language not specified, using default: %s", conf.Locales.DefaultLocale())

	} else if !conf.Locales.SupportsLocale("en") {
		log.Warn().Msgf("missing `en` locale - adding")
		conf.Locales = append(conf.Locales, LocaleConf{
			Name: dfltLanguage,
		})
	}
	var numLocales int
	for _, v := range conf.Locales {
		if v.IsDefault {
			numLocales++
		}
	}
	if numLocales != 1 {
		log.Fatal().Msg("at least one locale must be set as default")
		return
	}

	// corpora conf
	if err := conf.CorporaSetup.ValidateAndDefaults("corpora"); err != nil {
		log.Fatal().Err(err).Msg("invalid configuration")
	}
	if err := conf.CorporaSetup.ValidateAndDefaults("corporaSetup"); err != nil {
		log.Fatal().Err(err).Msg("invalid configuration")
	}
	if conf.Auth == nil || len(conf.Auth.KnownProxies) == 0 {
		for _, v := range conf.CorporaSetup.Resources {
			if v.InternalNetworkAccessOnly {
				log.Warn().
					Str("corpus", v.ID).
					Msg("found internal-network-only corpus but no `auth.knownProxies` configured - " +
						"if MQuery runs behind a reverse proxy, the corpus may be accessible from outside")
				break
			}
		}
	}
	if conf.TimeZone == "" {
		log.Warn().
			Str("timeZone", dfltTimeZone).
			Msg("time zone not specified, using default")
	}
	if _, err := time.LoadLocation(conf.TimeZone); err != nil {
		log.Fatal().Err(err).Msg("invalid time zone")
	}

	if (strings.HasPrefix(conf.ListenAddress, "0.0.0.0") || strings.HasPrefix(conf.ListenAddress, ":")) &&
		conf.Redis.AllowCustomTimeouts {
		log.Fatal().Msg("allowCustomTimeouts enabled but listening on all interfaces")
	}
}
