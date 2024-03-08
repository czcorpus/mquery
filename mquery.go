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
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/czcorpus/cnc-gokit/logging"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"mquery/cnf"
	corpusActions "mquery/corpus/handlers"
	"mquery/corpus/infoload"
	"mquery/general"
	"mquery/monitoring"
	monitoringActions "mquery/monitoring/handlers"
	"mquery/rdb"
	"mquery/worker"
)

var (
	version   string
	buildDate string
	gitCommit string
)

func getEnv(name string) string {
	for _, p := range os.Environ() {
		items := strings.Split(p, "=")
		if len(items) == 2 && items[0] == name {
			return items[1]
		}
	}
	return ""
}

func init() {
}

func getRequestOrigin(ctx *gin.Context) string {
	currOrigin, ok := ctx.Request.Header["Origin"]
	if ok {
		return currOrigin[0]
	}
	return ""
}

func CORSMiddleware(conf *cnf.Conf) gin.HandlerFunc {
	return func(ctx *gin.Context) {
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

		ctx.Next()
	}
}

func runApiServer(
	conf *cnf.Conf,
	syscallChan chan os.Signal,
	exitEvent chan os.Signal,
	radapter *rdb.Adapter,
	infoProvider infoload.Provider,
) {
	if !conf.LogLevel.IsDebugMode() {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(logging.GinMiddleware())
	engine.Use(uniresp.AlwaysJSONContentType())
	engine.Use(CORSMiddleware(conf))
	engine.NoMethod(uniresp.NoMethodHandler)
	engine.NoRoute(uniresp.NotFoundHandler)

	ceActions := corpusActions.NewActions(
		conf.CorporaSetup, radapter, infoProvider, conf.Language)

	engine.POST(
		"/corpus/:corpusId/split", ceActions.SplitCorpus)

	engine.DELETE(
		"/corpus/:corpusId/split", ceActions.DeleteSplit)

	engine.GET(
		"/corpus/:corpusId/info", ceActions.CorpusInfo)

	engine.GET(
		"/freqs/:corpusId", ceActions.FreqDistrib)

	engine.GET(
		"/freqs2/:corpusId", ceActions.FreqDistribParallel)

	engine.GET(
		"/text-types-norms/:corpusId", ceActions.TextTypesNorms)

	engine.GET(
		"/text-types-streamed/:corpusId", ceActions.TextTypesStreamed)

	engine.GET(
		"/freqs-by-year-streamed/:corpusId", ceActions.FreqsByYears)

	engine.GET(
		"/text-types/:corpusId", ceActions.TextTypes)

	engine.GET(
		"/text-types2/:corpusId", ceActions.TextTypesParallel)

	engine.GET(
		"/text-types-overview", ceActions.TextTypesOverview)

	engine.GET(
		"/collocs/:corpusId", ceActions.Collocations)

	engine.GET(
		"/word-forms/:corpusId", ceActions.WordForms)

	engine.GET(
		"/conc-examples/:corpusId", ceActions.SyntaxConcordance) // TODO rename API endpoint (where is `syntax`?)

	engine.GET(
		"/concordance/:corpusId", ceActions.Concordance)

	logger := monitoring.NewWorkerJobLogger(conf.TimezoneLocation())
	logger.GoRunTimelineWriter()
	monitoringActions := monitoringActions.NewActions(logger, conf.TimezoneLocation())

	engine.GET(
		"/monitoring/workers-load", monitoringActions.WorkersLoad)

	log.Info().Msgf("starting to listen at %s:%d", conf.ListenAddress, conf.ListenPort)
	srv := &http.Server{
		Handler:      engine,
		Addr:         fmt.Sprintf("%s:%d", conf.ListenAddress, conf.ListenPort),
		WriteTimeout: time.Duration(conf.ServerWriteTimeoutSecs) * time.Second,
		ReadTimeout:  time.Duration(conf.ServerReadTimeoutSecs) * time.Second,
	}
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Error().Err(err).Msg("")
		}
		syscallChan <- syscall.SIGTERM
	}()

	select {
	case <-exitEvent:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := srv.Shutdown(ctx)
		if err != nil {
			log.Info().Err(err).Msg("Shutdown request error")
		}
	}
}

func runWorker(conf *cnf.Conf, workerID string, radapter *rdb.Adapter, exitEvent chan os.Signal) {
	ch := radapter.Subscribe()
	logger := monitoring.NewWorkerJobLogger(conf.TimezoneLocation())
	w := worker.NewWorker(workerID, radapter, ch, exitEvent, logger)
	w.Listen()
}

func getWorkerID() (workerID string) {
	workerID = getEnv("WORKER_ID")
	if workerID == "" {
		workerID = "0"
	}
	return
}

func main() {
	version := general.VersionInfo{
		Version:   version,
		BuildDate: buildDate,
		GitCommit: gitCommit,
	}

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
		var wPath string
		if conf.LogFile != "" {
			wPath = filepath.Join(filepath.Dir(conf.LogFile), "worker.log")
		}
		logging.SetupLogging(wPath, conf.LogLevel)
		log.Logger = log.Logger.With().Str("worker", getWorkerID()).Logger()

	} else if action == "test" {
		cnf.ValidateAndDefaults(conf)
		log.Info().Msg("config OK")
		return

	} else {
		logging.SetupLogging(conf.LogFile, conf.LogLevel)
	}
	log.Info().Msg("Starting MQUERY")
	cnf.ValidateAndDefaults(conf)
	syscallChan := make(chan os.Signal, 1)
	signal.Notify(syscallChan, os.Interrupt)
	signal.Notify(syscallChan, syscall.SIGTERM)
	exitEvent := make(chan os.Signal)
	testConnCancel := make(chan bool)
	go func() {
		evt := <-syscallChan
		testConnCancel <- true
		close(testConnCancel)
		exitEvent <- evt
		close(exitEvent)
	}()

	radapter := rdb.NewAdapter(conf.Redis)

	switch action {
	case "server":
		err := radapter.TestConnection(20*time.Second, testConnCancel)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to connect to Redis")
		}
		infoProvider := infoload.NewManatee(radapter, conf.CorporaSetup)
		runApiServer(conf, syscallChan, exitEvent, radapter, infoProvider)
	case "worker":
		err := radapter.TestConnection(20*time.Second, testConnCancel)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to connect to Redis")
		}
		runWorker(conf, getWorkerID(), radapter, exitEvent)
	default:
		log.Fatal().Msgf("Unknown action %s", action)
	}

}
