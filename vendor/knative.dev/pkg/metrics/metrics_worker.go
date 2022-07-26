/*
Copyright 2021 The Knative Authors

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

	"go.opencensus.io/stats/view"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
)

type metricsWorker struct {
	c chan command
}

func newMetricsWorker() *metricsWorker {
	return &metricsWorker{c: make(chan command)}
}

type command interface {
	handleCommand(w *metricsWorker)
}

type readExporter struct {
	done chan *view.Exporter
}

type setExporter struct {
	newExporter *view.Exporter
	done        chan struct{}
}

type readMetricsConfig struct {
	done chan *metricsConfig
}

type updateMetricsConfigWithExporter struct {
	ctx       context.Context
	newConfig *metricsConfig
	done      chan error
}

type setMetricsConfig struct {
	newConfig *metricsConfig
	done      chan struct{}
}

func (cmd *readMetricsConfig) handleCommand(w *metricsWorker) {
	cmd.done <- curMetricsConfig
}

func (cmd *setMetricsConfig) handleCommand(w *metricsWorker) {
	setCurMetricsConfigUnlocked(cmd.newConfig)
	cmd.done <- struct{}{}
}

func (cmd *updateMetricsConfigWithExporter) handleCommand(w *metricsWorker) {
	ctx := cmd.ctx
	logger := logging.FromContext(ctx)
	if isNewExporterRequired(cmd.newConfig) {
		logger.Debug("Flushing the existing exporter before setting up the new exporter.")
		flushGivenExporter(curMetricsExporter)
		e, f, err := newMetricsExporter(cmd.newConfig, logger)
		if err != nil {
			logger.Errorw("Failed to update a new metrics exporter based on metric config", zap.Error(err), "config", cmd.newConfig)
			cmd.done <- err
			return
		}
		existingConfig := curMetricsConfig
		curMetricsExporter = e
		if err := setFactory(f); err != nil {
			logger.Errorw("Failed to update metrics factory when loading metric config", zap.Error(err), "config", cmd.newConfig)
			cmd.done <- err
			return
		}
		logger.Debugf("Successfully updated the metrics exporter; old config: %v; new config %v", existingConfig, cmd.newConfig)
	}
	setCurMetricsConfigUnlocked(cmd.newConfig)
	cmd.done <- nil
}

func (w *metricsWorker) start() {
	for {
		cmd := <-w.c
		cmd.handleCommand(w)
	}
}

func (cmd *setExporter) handleCommand(w *metricsWorker) {
	curMetricsExporter = *cmd.newExporter
	cmd.done <- struct{}{}
}

func (cmd *readExporter) handleCommand(w *metricsWorker) {
	cmd.done <- &curMetricsExporter
}
