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
	"mquery/rdb"
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

// Conf is a global configuration of the app
type Conf struct {
	ListenAddress          string               `json:"listenAddress"`
	PublicURL              string               `json:"publicUrl"`
	ListenPort             int                  `json:"listenPort"`
	ServerReadTimeoutSecs  int                  `json:"serverReadTimeoutSecs"`
	ServerWriteTimeoutSecs int                  `json:"serverWriteTimeoutSecs"`
	CorsAllowedOrigins     []string             `json:"corsAllowedOrigins"`
	CorporaSetup           *corpus.CorporaSetup `json:"corpora"`
	CQLTranslatorURL       string               `json:"cqlTranslatorURL"`
	Redis                  *rdb.Conf            `json:"redis"`
	LogFile                string               `json:"logFile"`
	LogLevel               logging.LogLevel     `json:"logLevel"`
	Locales                LocalesConf          `json:"locales"`
	TimeZone               string               `json:"timeZone"`
	PrivacyPolicy          PrivacyPolicy        `json:"privacyPolicy"`
	AuthHeaderName         string               `json:"authHeaderName"`
	AuthTokens             []string             `json:"authTokens"`

	srcPath string
}

func (conf *Conf) LoadSubconfigs() error {
	if conf.CorporaSetup.ConfFilesDir != "" {
		if err := conf.CorporaSetup.Resources.Load(conf.CorporaSetup.ConfFilesDir); err != nil {
			return fmt.Errorf("failed to load subconfig for `corpora`: %w", err)
		}
	}
	return nil
}

func (conf *Conf) IsDebugMode() bool {
	return conf.LogLevel == "debug"
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
	if conf.PublicURL == "" {
		conf.PublicURL = fmt.Sprintf("http://%s", conf.ListenAddress)
		log.Warn().Str("address", conf.PublicURL).Msg("publicUrl not set, using listenAddress")
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
	if conf.TimeZone == "" {
		log.Warn().
			Str("timeZone", dfltTimeZone).
			Msg("time zone not specified, using default")
	}
	if _, err := time.LoadLocation(conf.TimeZone); err != nil {
		log.Fatal().Err(err).Msg("invalid time zone")
	}
}
