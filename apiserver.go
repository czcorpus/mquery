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
	"embed"
	"fmt"
	"mquery/cnf"
	corpusActions "mquery/corpus/handlers"
	"mquery/corpus/infoload"
	"mquery/docs"
	"mquery/proxied"
	"mquery/rdb"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/czcorpus/cnc-gokit/logging"
	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type apiServer struct {
	server       *http.Server
	conf         *cnf.Conf
	radapter     *rdb.Adapter
	infoProvider *infoload.Manatee
}

//go:embed docs/swagger.json
var swaggerJSON embed.FS

func (api *apiServer) Start(ctx context.Context) {
	if !api.conf.Logging.Level.IsDebugMode() {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(additionalLogEvents())
	engine.Use(logging.GinMiddleware())
	engine.Use(uniresp.AlwaysJSONContentType())
	engine.Use(CORSMiddleware(api.conf))
	engine.NoMethod(uniresp.NoMethodHandler)
	engine.NoRoute(uniresp.NotFoundHandler)

	protected := engine.Group("/tools").Use(AuthRequired(api.conf))

	ceActions := corpusActions.NewActions(
		api.conf.CorporaSetup, api.radapter, api.infoProvider, api.conf.Locales)

	engine.GET("/", mkServerInfo(api.conf))

	engine.GET("/privacy-policy", mkPrivacyPolicy(api.conf))

	if api.conf.APIDocsURLPath == "" {
		docs.SwaggerInfo.BasePath = api.conf.APIDocsURLPath
	}

	engine.GET("/docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// also serve the JSON variant of the docs on the legacy URL:
	engine.GET(
		"/openapi",
		func(ctx *gin.Context) {
			jsonFile, err := swaggerJSON.ReadFile("docs/swagger.json")
			if err != nil {
				err = fmt.Errorf("Failed to read Swagger file: %w", err)
				uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
				return
			}
			uniresp.WriteRawJSONResponse(ctx.Writer, jsonFile)
		},
	)

	protected.POST(
		"/split/:corpusId", ceActions.SplitCorpus)

	protected.DELETE(
		"/split/:corpusId", ceActions.DeleteSplit)

	engine.GET(
		"/info/:corpusId", ceActions.CorpusInfo)

	engine.GET(
		"/corplist", ceActions.Corplist)

	engine.GET(
		"/term-frequency/:corpusId", ceActions.TermFrequency)

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
		"/text-types-overview/:corpusId", ceActions.TextTypesOverview)

	engine.GET(
		"/collocations/:corpusId", ceActions.Collocations)

	engine.GET(
		"/collocations-extended/:corpusId", ceActions.CollocationsExtended)

	engine.GET(
		"/word-forms/:corpusId", ceActions.WordForms)

	engine.GET(
		"/conc-examples/:corpusId", ceActions.SyntaxConcordance) // TODO rename API endpoint (where is `syntax`?)

	engine.GET(
		"/concordance/:corpusId", ceActions.Concordance)

	engine.GET(
		"/token-context/:corpusId", ceActions.TokenContext)

	engine.GET(
		"/sentences/:corpusId", ceActions.Sentences)

	if api.conf.CorporaSetup.AudioFilesDir != "" {
		engine.GET(
			"/audio/:corpusId", ceActions.Audio)

	} else {
		log.Warn().Msg("the audio files location not specified, endpoint /audio/:corpusId will be disabled")
	}

	if api.conf.CQLTranslatorURL != "" {
		ctActions := proxied.NewActions(api.conf.CQLTranslatorURL)
		engine.GET("/translate", ctActions.RemoteQueryTranslator)
		log.Info().Str("url", api.conf.CQLTranslatorURL).Msg("enabling CQL translator proxy")

	} else {
		log.Info().Msg("CQL translator proxy not specified - /translate endpoint will be disabled")
	}

	log.Info().Msgf("starting to listen at %s:%d", api.conf.ListenAddress, api.conf.ListenPort)
	api.server = &http.Server{
		Handler:      engine,
		Addr:         fmt.Sprintf("%s:%d", api.conf.ListenAddress, api.conf.ListenPort),
		WriteTimeout: time.Duration(api.conf.ServerWriteTimeoutSecs) * time.Second,
		ReadTimeout:  time.Duration(api.conf.ServerReadTimeoutSecs) * time.Second,
	}
	go func() {
		if err := api.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

}

func (s *apiServer) Stop(ctx context.Context) error {
	log.Warn().Msg("shutting down MQuery HTTP API server")
	return s.server.Shutdown(ctx)
}

func runApiServer(
	conf *cnf.Conf,
) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	radapter := rdb.NewAdapter(conf.Redis, ctx)
	err := radapter.TestConnection(redisConnectionTestTimeout)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Redis")
		return
	}
	infoProvider := infoload.NewManatee(radapter, conf.CorporaSetup)
	server := newAPIServer(conf, radapter, infoProvider)

	services := []service{server}
	for _, m := range services {
		m.Start(ctx)
	}
	<-ctx.Done()
	log.Warn().Msg("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for _, s := range services {
		wg.Add(1)
		go func(srv service) {
			defer wg.Done()
			if err := srv.Stop(shutdownCtx); err != nil {
				log.Error().Err(err).Type("service", srv).Msg("Error shutting down service")
			}
		}(s)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info().Msg("Graceful shutdown completed")
	case <-shutdownCtx.Done():
		log.Warn().Msg("Shutdown timed out")
	}
}

func newAPIServer(
	conf *cnf.Conf,
	radapter *rdb.Adapter,
	infoProvider *infoload.Manatee,
) *apiServer {
	return &apiServer{
		conf:         conf,
		radapter:     radapter,
		infoProvider: infoProvider,
	}
}
