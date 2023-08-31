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
	"mquery/corpus"
	"mquery/corpus/scoll"
	"mquery/db"
	"mquery/rdb"
	"os"
	"path/filepath"

	"github.com/czcorpus/cnc-gokit/logging"
	"github.com/rs/zerolog/log"
)

const (
	dfltServerWriteTimeoutSecs = 30
	dfltLanguage               = "en"
	dfltMaxNumConcurrentJobs   = 4
	dfltVertMaxNumErrors       = 100
)

// Conf is a global configuration of the app
type Conf struct {
	ListenAddress          string               `json:"listenAddress"`
	ListenPort             int                  `json:"listenPort"`
	ServerReadTimeoutSecs  int                  `json:"serverReadTimeoutSecs"`
	ServerWriteTimeoutSecs int                  `json:"serverWriteTimeoutSecs"`
	CorsAllowedOrigins     []string             `json:"corsAllowedOrigins"`
	CorporaSetup           *corpus.CorporaSetup `json:"corporaSetup"`
	SketchSetup            *scoll.SketchSetup   `json:"sketchSetup"`
	DB                     *db.Conf             `json:"db"`
	Redis                  *rdb.Conf            `json:"redis"`
	LogFile                string               `json:"logFile"`
	LogLevel               logging.LogLevel     `json:"logLevel"`
	Language               string               `json:"language"`
	srcPath                string
}

func (conf *Conf) IsDebugMode() bool {
	return conf.LogLevel == "debug"
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

func DefaultsAndValidate(conf *Conf) {
	if conf.ServerWriteTimeoutSecs == 0 {
		conf.ServerWriteTimeoutSecs = dfltServerWriteTimeoutSecs
		log.Warn().Msgf(
			"serverWriteTimeoutSecs not specified, using default: %d",
			dfltServerWriteTimeoutSecs,
		)
	}
	if conf.Language == "" {
		conf.Language = dfltLanguage
		log.Warn().Msgf("language not specified, using default: %s", conf.Language)
	}
	if err := conf.SketchSetup.DefaultsAndValidate(); err != nil {
		log.Fatal().Err(err).Msg("invalid configuration")
	}
	if err := conf.CorporaSetup.DefaultsAndValidate(); err != nil {
		log.Fatal().Err(err).Msg("invalid configuration")
	}
}
