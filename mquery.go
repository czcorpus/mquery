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
	corpusEdit "mquery/corpus/edit"
	"mquery/corpus/query"
	"mquery/corpus/scoll"
	"mquery/db"
	"mquery/general"
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
) {
	if !conf.LogLevel.IsDebugMode() {
		gin.SetMode(gin.ReleaseMode)
	}

	sqlDB, err := db.Open(conf.DB)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize cache database")
	}
	backend := db.NewBackend(sqlDB)
	scollQueryExecutor := scoll.NewQueryExecutor(backend, radapter)

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(logging.GinMiddleware())
	engine.Use(uniresp.AlwaysJSONContentType())
	engine.Use(CORSMiddleware(conf))
	engine.NoMethod(uniresp.NoMethodHandler)
	engine.NoRoute(uniresp.NotFoundHandler)

	ceActions := corpusEdit.NewActions(conf.CorporaSetup, radapter)

	engine.POST(
		"/corpus/:corpusId/split", ceActions.SplitCorpus)

	engine.DELETE(
		"/corpus/:corpusId/split", ceActions.DeleteSplit)

	engine.POST(
		"/corpus/:corpusId/multisample", ceActions.MultiSample)

	engine.POST(
		"/corpus/:corpusId/collFreqData/:variant", ceActions.CollFreqData)

	concActions := query.NewActions(conf.CorporaSetup, radapter)

	engine.GET(
		"/freqs/:corpusId", concActions.FreqDistrib)

	engine.GET(
		"/freqs2/:corpusId", concActions.FreqDistribParallel)

	engine.GET(
		"/text-types-streamed/:corpusId", concActions.TextTypesStreamed)

	engine.GET(
		"/text-types/:corpusId", concActions.TextTypes)

	engine.GET(
		"/text-types2/:corpusId", concActions.TextTypesParallel)

	engine.GET(
		"/collocs/:corpusId", concActions.Collocations)

	engine.GET(
		"/collocs2/:corpusId", concActions.CollocationsParallel)

	engine.GET(
		"/word-forms/:corpusId", concActions.WordForms)

	engine.GET(
		"/conc-examples/:corpusId", concActions.ConcExample)

	sketchActions := scoll.NewActions(
		conf.CorporaSetup,
		conf.SketchSetup,
		radapter,
		scollQueryExecutor,
	)

	engine.GET(
		"/scoll/:corpusId/noun-modified-by", sketchActions.NounsModifiedBy)

	engine.GET(
		"/scoll/:corpusId/modifiers-of", sketchActions.ModifiersOf)

	engine.GET(
		"/scoll/:corpusId/verbs-subject", sketchActions.VerbsSubject)

	engine.GET(
		"/scoll/:corpusId/verbs-object", sketchActions.VerbsObject)

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

func runWorker(radapter *rdb.Adapter, exitEvent chan os.Signal) {
	ch := radapter.Subscribe()
	w := worker.NewWorker(radapter, ch, exitEvent)
	w.Listen()
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
		workerID := getEnv("WORKER_ID")
		if action == "worker" && workerID == "" {
			workerID = "0"
		}
		log.Logger = log.Logger.With().Str("worker", workerID).Logger()

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

	go func() {
		select {
		case evt := <-syscallChan:
			exitEvent <- evt
			close(exitEvent)
		}
	}()

	radapter := rdb.NewAdapter(conf.Redis)

	switch action {
	case "server":
		err := radapter.TestConnection(20 * time.Second)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to connect to Redis")
		}
		runApiServer(conf, syscallChan, exitEvent, radapter)
	case "worker":
		err := radapter.TestConnection(20 * time.Second)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to connect to Redis")
		}
		runWorker(radapter, exitEvent)
	default:
		log.Fatal().Msgf("Unknown action %s", action)
	}

}
