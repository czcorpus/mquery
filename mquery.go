// Copyright 2023 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2023 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2023 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/czcorpus/cnc-gokit/logging"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/czcorpus/mquery-common/concordance"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"mquery/cnf"
	"mquery/corpus/handlers"
	"mquery/docs"
	"mquery/general"
	"mquery/merror"
	"mquery/rdb"
	"mquery/rdb/results"
)

const (
	redisConnectionTestTimeout = 120 * time.Second
	TimeoutHeader              = "X-Worker-Timeout"
)

var (
	version   string
	buildDate string
	gitCommit string
)

func init() {
	gob.Register(rdb.CorpusInfoArgs{})
	gob.Register(rdb.FreqDistribArgs{})
	gob.Register(rdb.CollocationsArgs{})
	gob.Register(rdb.TermFrequencyArgs{})
	gob.Register(rdb.ConcordanceArgs{})
	gob.Register(rdb.CalcCollFreqDataArgs{})
	gob.Register(rdb.TextTypeNormsArgs{})
	gob.Register(rdb.TokenContextArgs{})
	gob.Register(results.CollFreqData{})
	gob.Register(results.Collocations{})
	gob.Register(results.ConcSize{})
	gob.Register(results.Concordance{})
	gob.Register(results.CorpusInfo{})
	gob.Register(results.FreqDistrib{})
	gob.Register(results.TextTypeNorms{})
	gob.Register(results.TokenContext{})
	gob.Register(&concordance.Token{})
	gob.Register(&concordance.Struct{})
	gob.Register(&concordance.CloseStruct{})
	gob.Register(merror.InputError{})
	gob.Register(merror.InternalError{})
	gob.Register(merror.RecoveredError{})
	gob.Register(merror.TimeoutError{})
	gob.Register(rdb.ErrorResult{})
}

type service interface {
	Start(ctx context.Context)
	Stop(ctx context.Context) error
}

func getEnv(name string) string {
	for _, p := range os.Environ() {
		items := strings.Split(p, "=")
		if len(items) == 2 && items[0] == name {
			return items[1]
		}
	}
	return ""
}

func getRequestOrigin(ctx *gin.Context) string {
	currOrigin, ok := ctx.Request.Header["Origin"]
	if ok {
		return currOrigin[0]
	}
	return ""
}

func additionalLogEvents() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		logging.AddLogEvent(ctx, "userAgent", ctx.Request.UserAgent())
		logging.AddLogEvent(ctx, "corpusId", ctx.Param("corpusId"))
		ctx.Next()
	}
}

func CORSMiddleware(conf *cnf.Conf) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if strings.HasSuffix(ctx.Request.URL.Path, "/openapi") {
			ctx.Header("Access-Control-Allow-Origin", "*")
			ctx.Header("Access-Control-Allow-Methods", "GET")
			ctx.Header("Access-Control-Allow-Headers", "Content-Type")

		} else {
			var allowedOrigin string
			currOrigin := getRequestOrigin(ctx)
			for _, origin := range conf.CorsAllowedOrigins {
				if currOrigin == origin {
					allowedOrigin = origin
					break
				}
			}
			if allowedOrigin != "" {
				ctx.Writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				ctx.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
				ctx.Writer.Header().Set(
					"Access-Control-Allow-Headers",
					"Content-Type, Content-Length, Accept-Encoding, Authorization, Accept, Origin, Cache-Control, X-Requested-With",
				)
				ctx.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
			}

			if ctx.Request.Method == "OPTIONS" {
				ctx.AbortWithStatus(204)
				return
			}
		}
		ctx.Next()
	}
}

func CustomTimeoutMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if headerVal := ctx.GetHeader(TimeoutHeader); headerVal != "" {
			tmo, err := strconv.Atoi(headerVal)
			if err != nil {
				uniresp.RespondWithErrorJSON(ctx, err, http.StatusBadRequest)
				return
			}
			ctx.Set(handlers.TimeoutCtxKey, time.Duration(tmo)*time.Second)
		}
	}
}

func authTokenMatches(stored, provided string) bool {
	if hashed, ok := strings.CutPrefix(stored, "sha256:"); ok {
		sum := sha256.Sum256([]byte(provided))
		return hex.EncodeToString(sum[:]) == hashed
	}
	return stored == provided
}

func isLocalNetwork(conf *cnf.Conf, ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	if len(conf.Auth.LocalNetworks) > 0 {
		for _, cidr := range conf.Auth.LocalNetworks {
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
	return ip == conf.ListenAddress
}

func isKnownProxy(conf *cnf.Conf, ip string) bool {
	for _, p := range conf.Auth.KnownProxies {
		if p == ip {
			return true
		}
	}
	return false
}

func AuthRequired(conf *cnf.Conf) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		remoteIP, _, err := net.SplitHostPort(ctx.Request.RemoteAddr)
		isLocalDirect := err == nil && isLocalNetwork(conf, remoteIP) && !isKnownProxy(conf, remoteIP)
		if !isLocalDirect {
			provided := ctx.GetHeader(conf.Auth.TokenHeaderName)
			authorized := false
			for _, stored := range conf.Auth.Tokens {
				if authTokenMatches(stored, provided) {
					authorized = true
					break
				}
			}
			if !authorized {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
		}
		ctx.Next()
	}
}

func cleanVersionInfo(v string) string {
	return strings.TrimLeft(strings.Trim(v, "'"), "v")
}

// @title           MQuery - query and analyze corpus data
// @description     Retrieves concordances, frequency information and collocations from language corpora

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /

// @externalDocs.description  OpenAPI
// @externalDocs.url          https://swagger.io/resources/open-api/
func main() {
	version := general.VersionInfo{
		Version:   cleanVersionInfo(version),
		BuildDate: cleanVersionInfo(buildDate),
		GitCommit: cleanVersionInfo(gitCommit),
	}
	docs.SwaggerInfo.Version = version.Version

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "MQUERY - A specialized corpus querying server\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n\t%s [options] server [config.json]\n\t", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "Usage:\n\t%s [options] worker [config.json]\n\t", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "%s [options] version\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()
	action := flag.Arg(0)
	if action == "version" {
		fmt.Printf("mquery %s\nbuild date: %s\nlast commit: %s\n", version.Version, version.BuildDate, version.GitCommit)
		return
	}
	conf := cnf.LoadConfig(flag.Arg(1))

	if action == "worker" {
		if conf.Logging.Path != "" {
			conf.Logging.Path = filepath.Join(filepath.Dir(conf.Logging.Path), "worker.log")
		}
		logging.SetupLogging(conf.Logging)
		log.Logger = log.Logger.With().Str("worker", getWorkerID()).Logger()

	} else if action == "test" {
		cnf.ValidateAndDefaults(conf)
		log.Info().Msg("config OK")
		return

	} else {
		logging.SetupLogging(conf.Logging)
	}

	if err := conf.LoadSubconfigs(); err != nil {
		log.Fatal().Err(err).Msg("Failed to load subconfig(s)")
		return
	}

	log.Info().Msg("Starting MQuery")
	cnf.ValidateAndDefaults(conf)

	switch action {
	case "server":
		runApiServer(conf)
	case "worker":
		runWorker(conf)
	default:
		log.Fatal().Msgf("Unknown action %s", action)
	}

}
