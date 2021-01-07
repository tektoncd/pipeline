// +build nostackdriver

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
	"context"
	"errors"

	"go.opencensus.io/stats/view"
	"go.uber.org/zap"
)

func sdinit(ctx context.Context, m map[string]string, mc *metricsConfig, ops ExporterOptions) error {
	return errors.New("Stackdriver support is not included")
}

func newStackdriverExporter(config *metricsConfig, logger *zap.SugaredLogger) (view.Exporter, ResourceExporterFactory, error) {
	return nil, nil, errors.New("Stackdriver support is not included")
}
