/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metrics

import (
	"net/http"
	"strconv"
	"sync"

	prom "contrib.go.opencensus.io/exporter/prometheus"
	"go.opencensus.io/resource"
	"go.opencensus.io/stats/view"
	"go.uber.org/zap"
)

var (
	curPromSrv    *http.Server
	curPromSrvMux sync.Mutex
)

type emptyPromExporter struct{}

var _ view.Exporter = (*emptyPromExporter)(nil)

func (emptyPromExporter) ExportView(viewData *view.Data) {
	// Prometheus runs a loop to read stats via ReadAndExport, so this is just
	// a signal to enrich the internal Meters with Resource information.
}

// nolint: unparam // False positive of flagging the second result of this function unused.
func newPrometheusExporter(config *metricsConfig, logger *zap.SugaredLogger) (view.Exporter, ResourceExporterFactory, error) {
	e, err := prom.NewExporter(prom.Options{Namespace: config.component})
	if err != nil {
		logger.Errorw("Failed to create the Prometheus exporter.", zap.Error(err))
		return nil, nil, err
	}
	logger.Debugf("Created Prometheus exporter with config: %v. Start the server for Prometheus exporter.", config)
	// Start the server for Prometheus scraping
	go func() {
		srv := startNewPromSrv(e, config.prometheusHost, config.prometheusPort)
		srv.ListenAndServe()
	}()
	return e,
		func(r *resource.Resource) (view.Exporter, error) { return &emptyPromExporter{}, nil },
		nil
}

func getCurPromSrv() *http.Server {
	curPromSrvMux.Lock()
	defer curPromSrvMux.Unlock()
	return curPromSrv
}

func resetCurPromSrv() {
	curPromSrvMux.Lock()
	defer curPromSrvMux.Unlock()
	if curPromSrv != nil {
		curPromSrv.Close()
		curPromSrv = nil
	}
}

func startNewPromSrv(e *prom.Exporter, host string, port int) *http.Server {
	sm := http.NewServeMux()
	sm.Handle("/metrics", e)
	curPromSrvMux.Lock()
	defer curPromSrvMux.Unlock()
	if curPromSrv != nil {
		curPromSrv.Close()
	}
	//nolint:gosec
	curPromSrv = &http.Server{
		Addr:    host + ":" + strconv.Itoa(port),
		Handler: sm,
	}
	return curPromSrv
}
