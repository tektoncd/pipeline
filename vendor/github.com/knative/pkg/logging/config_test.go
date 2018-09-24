/*
Copyright 2018 The Knative Authors.

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

package logging

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewLogger(t *testing.T) {
	logger, _ := NewLogger("", "")
	if logger == nil {
		t.Error("expected a non-nil logger")
	}

	logger, _ = NewLogger("some invalid JSON here", "")
	if logger == nil {
		t.Error("expected a non-nil logger")
	}

	logger, atomicLevel := NewLogger("", "debug")
	if logger == nil {
		t.Error("expected a non-nil logger")
	}
	if atomicLevel.Level() != zapcore.DebugLevel {
		t.Error("expected level to be debug")
	}

	// No good way to test if all the config is applied,
	// but at the minimum, we can check and see if level is getting applied.
	logger, atomicLevel = NewLogger("{\"level\": \"error\", \"outputPaths\": [\"stdout\"],\"errorOutputPaths\": [\"stderr\"],\"encoding\": \"json\"}", "")
	if logger == nil {
		t.Error("expected a non-nil logger")
	}
	if ce := logger.Desugar().Check(zap.InfoLevel, "test"); ce != nil {
		t.Error("not expected to get info logs from the logger configured with error as min threshold")
	}
	if ce := logger.Desugar().Check(zap.ErrorLevel, "test"); ce == nil {
		t.Error("expected to get error logs from the logger configured with error as min threshold")
	}
	if atomicLevel.Level() != zapcore.ErrorLevel {
		t.Errorf("expected atomicLevel.Level() to be ErrorLevel but got %v.", atomicLevel.Level())
	}

	logger, atomicLevel = NewLogger("{\"level\": \"info\", \"outputPaths\": [\"stdout\"],\"errorOutputPaths\": [\"stderr\"],\"encoding\": \"json\"}", "")
	if logger == nil {
		t.Error("expected a non-nil logger")
	}
	if ce := logger.Desugar().Check(zap.DebugLevel, "test"); ce != nil {
		t.Error("not expected to get debug logs from the logger configured with info as min threshold")
	}
	if ce := logger.Desugar().Check(zap.InfoLevel, "test"); ce == nil {
		t.Error("expected to get info logs from the logger configured with info as min threshold")
	}
	if atomicLevel.Level() != zapcore.InfoLevel {
		t.Errorf("expected atomicLevel.Level() to be InfoLevel but got %v.", atomicLevel.Level())
	}

	// Let's change the logging level using atomicLevel
	atomicLevel.SetLevel(zapcore.ErrorLevel)
	if ce := logger.Desugar().Check(zap.InfoLevel, "test"); ce != nil {
		t.Error("not expected to get info logs from the logger configured with error as min threshold")
	}
	if ce := logger.Desugar().Check(zap.ErrorLevel, "test"); ce == nil {
		t.Error("expected to get error logs from the logger configured with error as min threshold")
	}
	if atomicLevel.Level() != zapcore.ErrorLevel {
		t.Errorf("expected atomicLevel.Level() to be ErrorLevel but got %v.", atomicLevel.Level())
	}

	// Test logging override
	logger, _ = NewLogger("{\"level\": \"error\", \"outputPaths\": [\"stdout\"],\"errorOutputPaths\": [\"stderr\"],\"encoding\": \"json\"}", "info")
	if logger == nil {
		t.Error("expected a non-nil logger")
	}
	if ce := logger.Desugar().Check(zap.DebugLevel, "test"); ce != nil {
		t.Error("not expected to get debug logs from the logger configured with info as min threshold")
	}
	if ce := logger.Desugar().Check(zap.InfoLevel, "test"); ce == nil {
		t.Error("expected to get info logs from the logger configured with info as min threshold")
	}

	// Invalid logging override
	logger, _ = NewLogger("{\"level\": \"error\", \"outputPaths\": [\"stdout\"],\"errorOutputPaths\": [\"stderr\"],\"encoding\": \"json\"}", "randomstring")
	if logger == nil {
		t.Error("expected a non-nil logger")
	}
	if ce := logger.Desugar().Check(zap.InfoLevel, "test"); ce != nil {
		t.Error("not expected to get info logs from the logger configured with error as min threshold")
	}
	if ce := logger.Desugar().Check(zap.ErrorLevel, "test"); ce == nil {
		t.Error("expected to get error logs from the logger configured with error as min threshold")
	}
}

func TestNewConfigNoEntry(t *testing.T) {
	c, err := NewConfigFromConfigMap(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "knative-something",
			Name:      "config-logging",
		},
	})
	if err != nil {
		t.Errorf("Expected no errors. got: %v", err)
	}
	if got, want := c.LoggingConfig, ""; got != want {
		t.Errorf("LoggingConfig = %v, want %v", got, want)
	}
	if got, want := len(c.LoggingLevel), 0; got != want {
		t.Errorf("len(LoggingLevel) = %v, want %v", got, want)
	}
}

func TestNewConfig(t *testing.T) {
	wantCfg := "{\"level\": \"error\",\n\"outputPaths\": [\"stdout\"],\n\"errorOutputPaths\": [\"stderr\"],\n\"encoding\": \"json\"}"
	wantLevel := zapcore.InfoLevel
	c, err := NewConfigFromConfigMap(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "knative-something",
			Name:      "config-logging",
		},
		Data: map[string]string{
			"zap-logger-config":   wantCfg,
			"loglevel.queueproxy": wantLevel.String(),
		},
	})
	if err != nil {
		t.Errorf("Expected no errors. got: %v", err)
	}
	if got := c.LoggingConfig; got != wantCfg {
		t.Errorf("LoggingConfig = %v, want %v", got, wantCfg)
	}
	if got := c.LoggingLevel["queueproxy"]; got != wantLevel {
		t.Errorf("LoggingLevel[queueproxy] = %v, want %v", got, wantLevel)
	}
}

func TestNewLoggerFromConfig(t *testing.T) {
	c, _, _ := getTestConfig()
	_, atomicLevel := NewLoggerFromConfig(c, "queueproxy")
	if atomicLevel.Level() != zapcore.DebugLevel {
		t.Errorf("logger level wanted: DebugLevel, got: %v", atomicLevel)
	}
}

func TestEmptyLevel(t *testing.T) {
	c, err := NewConfigFromConfigMap(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "knative-something",
			Name:      "config-logging",
		},
		Data: map[string]string{
			"zap-logger-config":   "{\"level\": \"error\",\n\"outputPaths\": [\"stdout\"],\n\"errorOutputPaths\": [\"stderr\"],\n\"encoding\": \"json\"}",
			"loglevel.queueproxy": "",
		},
	})
	if err != nil {
		t.Errorf("Expected no errors. got: %v", err)
	}
	if _, ok := c.LoggingLevel["queueproxy"]; ok {
		t.Errorf("Expected nothing for LoggingLevel[queueproxy]. got: %v", c.LoggingLevel["queueproxy"])
	}
}

func TestInvalidLevel(t *testing.T) {
	wantCfg := "{\"level\": \"error\",\n\"outputPaths\": [\"stdout\"],\n\"errorOutputPaths\": [\"stderr\"],\n\"encoding\": \"json\"}"
	_, err := NewConfigFromConfigMap(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "knative-something",
			Name:      "config-logging",
		},
		Data: map[string]string{
			"zap-logger-config":   wantCfg,
			"loglevel.queueproxy": "invalid",
		},
	}, "queueproxy")
	if err == nil {
		t.Errorf("Expected errors when invalid level is present in logging config. got nothing")
	}
}

func getTestConfig() (*Config, string, string) {
	wantCfg := "{\"level\": \"error\",\n\"outputPaths\": [\"stdout\"],\n\"errorOutputPaths\": [\"stderr\"],\n\"encoding\": \"json\"}"
	wantLevel := "debug"
	c, _ := NewConfigFromConfigMap(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "knative-something",
			Name:      "config-logging",
		},
		Data: map[string]string{
			"zap-logger-config":   wantCfg,
			"loglevel.queueproxy": wantLevel,
		},
	}, "queueproxy")
	return c, wantCfg, wantLevel
}

func TestUpdateLevelFromConfigMap(t *testing.T) {
	logger, atomicLevel := NewLogger("", "debug")
	want := zapcore.DebugLevel
	if atomicLevel.Level() != zapcore.DebugLevel {
		t.Fatalf("Expected initial logger level to %v, got: %v", want, atomicLevel.Level())
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "knative-something",
			Name:      "config-logging",
		},
		Data: map[string]string{
			"zap-logger-config":   "",
			"loglevel.controller": "panic",
		},
	}

	tests := []struct {
		setLevel  string
		wantLevel zapcore.Level
	}{
		{"info", zapcore.InfoLevel},
		{"error", zapcore.ErrorLevel},
		{"invalid", zapcore.ErrorLevel},
		{"debug", zapcore.DebugLevel},
		{"debug", zapcore.DebugLevel},
	}

	u := UpdateLevelFromConfigMap(logger, atomicLevel, "controller", "controller")
	for _, tt := range tests {
		cm.Data["loglevel.controller"] = tt.setLevel
		u(cm)
		if atomicLevel.Level() != tt.wantLevel {
			t.Errorf("Invalid logging level. want: %v, got: %v", tt.wantLevel, atomicLevel.Level())
		}
	}
}
