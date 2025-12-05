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
	"mquery/cnf"
	"mquery/rdb"
	"mquery/worker"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
)

func getWorkerID() (workerID string) {
	workerID = getEnv("WORKER_ID")
	if workerID == "" {
		workerID = strconv.Itoa(os.Getpid())
	}
	return
}

// -------

type NullLogger struct{}

func (n *NullLogger) Log(rec rdb.JobLog) {}

//

type NullStatusWriter struct{}

func (n *NullStatusWriter) Write(rec rdb.JobLog) {}

// -------

func runWorker(conf *cnf.Conf) {
	workerID := getWorkerID()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	radapter := rdb.NewAdapter(conf.Redis, ctx, &NullStatusWriter{})

	err := radapter.TestConnection(redisConnectionTestTimeout)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Redis")
	}

	ch := radapter.Subscribe()
	wrk := worker.NewWorker(workerID, radapter, ch)

	services := []service{wrk}
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
