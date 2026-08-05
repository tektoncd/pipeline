// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/elastic/crd-ref-docs/config"
	"github.com/elastic/crd-ref-docs/processor"
	"github.com/elastic/crd-ref-docs/renderer"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var args = config.Flags{}

func main() {
	cmd := cobra.Command{
		Use:           "crd-ref-docs",
		Short:         "Generate CRD reference documentation",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       printVersion(),
		RunE:          doRun,
	}

	cmd.SetVersionTemplate("{{ .Version }}\n")

	cmd.Flags().StringVar(&args.LogLevel, "log-level", "INFO", "Log level")
	cmd.Flags().StringVar(&args.Config, "config", "config.yaml", "Path to config file")
	cmd.Flags().StringVar(&args.SourcePath, "source-path", "", "Path to source directory containing CRDs")
	cmd.Flags().StringVar(&args.TemplatesDir, "templates-dir", "", "Path to the directory containing template files")
	cmd.Flags().StringVar(&args.Renderer, "renderer", "asciidoctor", "Renderer to use ('asciidoctor' or 'markdown')")
	cmd.Flags().StringVar(&args.OutputPath, "output-path", ".", "Path to output the rendered result")
	cmd.Flags().StringVar(&args.OutputMode, "output-mode", "single", "Output mode to generate a single file or one file per group ('group' or 'single')")
	cmd.Flags().IntVar(&args.MaxDepth, "max-depth", 10, "Maximum recursion level for type discovery")
	cmd.Flags().Var(&args.TemplateKeyValues, "template-value", "Can be used in template to pass in a version number or similar information. Example: --template-value=key1=value1 and {{ markdownTemplateValue \"k1\" }}")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func doRun(_ *cobra.Command, _ []string) error {
	initLogging(args.LogLevel)

	zap.S().Infow("Loading configuration", "path", args.Config)
	conf, err := config.Load(args)
	if err != nil {
		zap.S().Errorw("Failed to read config", "error", err)
		return err
	}

	r, err := renderer.New(conf)
	if err != nil {
		zap.S().Errorw("Failed to create renderer", "error", err)
		return err
	}

	startTime := time.Now()
	defer func() {
		zap.S().Infof("Execution time: %s", time.Since(startTime))
	}()

	zap.S().Infow("Processing source directory", "directory", conf.SourcePath, "depth", conf.MaxDepth)
	gvd, err := processor.Process(conf)
	if err != nil {
		zap.S().Errorw("Failed to process source directory", "error", err)
		return err
	}

	zap.S().Infow("Rendering output", "path", conf.OutputPath)
	if err := r.Render(gvd); err != nil {
		zap.S().Errorw("Failed to render", "error", err)
		return err
	}

	zap.S().Info("CRD reference documentation generated")
	return nil
}

func initLogging(level string) {
	var logger *zap.Logger
	var err error
	errorPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})

	minLogLevel := zapcore.InfoLevel
	switch strings.ToUpper(level) {
	case "DEBUG":
		minLogLevel = zapcore.DebugLevel
	case "INFO":
		minLogLevel = zapcore.InfoLevel
	case "WARN":
		minLogLevel = zapcore.WarnLevel
	case "ERROR":
		minLogLevel = zapcore.ErrorLevel
	}

	infoPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl < zapcore.ErrorLevel && lvl >= minLogLevel
	})

	consoleErrors := zapcore.Lock(os.Stderr)
	consoleInfo := zapcore.Lock(os.Stdout)

	encoderConf := zap.NewDevelopmentEncoderConfig()
	encoderConf.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConf)

	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, consoleErrors, errorPriority),
		zapcore.NewCore(consoleEncoder, consoleInfo, infoPriority),
	)

	stackTraceEnabler := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl > zapcore.ErrorLevel
	})
	logger = zap.New(core, zap.AddStacktrace(stackTraceEnabler))

	if err != nil {
		zap.S().Fatalw("Failed to create logger", "error", err)
	}

	zap.ReplaceGlobals(logger.Named("crd-ref-docs"))
	zap.RedirectStdLog(logger.Named("stdlog"))
}

// Global build information variables defined via ldflags during release.
var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

// printVersion prints the version, git commit and build date using global variables defined via ldflags during release,
// or using values ​​from debug.ReadBuildInfo().
func printVersion() string {
	return fmt.Sprintf("Version: %s\nGitCommit: %s\nBuildDate: %s\n", version(), commit(), date())
}

func version() string {
	if buildVersion != "" {
		return buildVersion
	}
	bi, ok := debug.ReadBuildInfo()
	if ok && bi != nil && bi.Main.Version != "" {
		return bi.Main.Version
	}
	return "(unknown)"
}

func date() string {
	if buildDate != "" {
		return buildDate
	}
	bi, ok := debug.ReadBuildInfo()
	if ok {
		for _, setting := range bi.Settings {
			if setting.Key == "vcs.time" {
				return setting.Value
			}
		}
	}
	return "(unknown)"
}

func commit() string {
	if buildCommit != "" {
		return buildCommit
	}
	bi, ok := debug.ReadBuildInfo()
	if ok {
		for _, setting := range bi.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	return "(unknown)"
}
