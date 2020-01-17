/*
Copyright 2020 The Knative Authors
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
	"contrib.go.opencensus.io/exporter/ocagent"
	"go.opencensus.io/stats/view"
	"go.uber.org/zap"
)

func newOpenCensusExporter(config *metricsConfig, logger *zap.SugaredLogger) (view.Exporter, error) {
	opts := []ocagent.ExporterOption{ocagent.WithServiceName(config.component)}
	if config.collectorAddress != "" {
		opts = append(opts, ocagent.WithAddress(config.collectorAddress))
	}
	if !config.requireSecure {
		opts = append(opts, ocagent.WithInsecure())
	}
	e, err := ocagent.NewExporter(opts...)
	if err != nil {
		logger.Errorw("Failed to create the OpenCensus exporter.", zap.Error(err))
		return nil, err
	}
	logger.Infof("Created OpenCensus exporter with config: %+v.", *config)
	view.RegisterExporter(e)
	return e, nil
}
