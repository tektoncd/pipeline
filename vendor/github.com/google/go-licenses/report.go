// Copyright 2019 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/csv"
	"io/ioutil"
	"os"
	"text/template"

	"github.com/google/go-licenses/licenses"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

const (
	UNKNOWN = "Unknown"
)

var (
	reportHelp = "Prints report of all licenses that apply to one or more Go packages and their dependencies."
	reportCmd  = &cobra.Command{
		Use:   "report <package> [package...]",
		Short: reportHelp,
		Long:  reportHelp + packageHelp,
		Args:  cobra.MinimumNArgs(1),
		RunE:  reportMain,
	}

	templateFile string
)

func init() {
	reportCmd.Flags().StringVar(&templateFile, "template", "", "Custom Go template file to use for report")

	rootCmd.AddCommand(reportCmd)
}

type libraryData struct {
	Name        string
	LicenseURL  string
	LicenseName string
	LicensePath string
	Version     string
}

// LicenseText reads and returns the contents of LicensePath, if set
// or an empty string if not.
func (lib libraryData) LicenseText() (string, error) {
	if lib.LicensePath == "" {
		return "", nil
	}
	data, err := ioutil.ReadFile(lib.LicensePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func reportMain(_ *cobra.Command, args []string) error {
	classifier, err := licenses.NewClassifier(confidenceThreshold)
	if err != nil {
		return err
	}

	libs, err := licenses.Libraries(context.Background(), classifier, includeTests, ignore, args...)
	if err != nil {
		return err
	}

	var reportData []libraryData
	for _, lib := range libs {
		version := lib.Version()
		if len(version) == 0 {
			version = UNKNOWN
		}
		libData := libraryData{
			Name:        lib.Name(),
			Version:     version,
			LicenseURL:  UNKNOWN,
			LicenseName: UNKNOWN,
		}
		if lib.LicensePath != "" {
			libData.LicensePath = lib.LicensePath
			name, _, err := classifier.Identify(lib.LicensePath)
			if err == nil {
				libData.LicenseName = name
			} else {
				klog.Errorf("Error identifying license in %q: %v", lib.LicensePath, err)
			}
			url, err := lib.FileURL(context.Background(), lib.LicensePath)
			if err == nil {
				libData.LicenseURL = url
			} else {
				klog.Warningf("Error discovering license URL: %s", err)
			}
		}
		reportData = append(reportData, libData)
	}

	if templateFile == "" {
		return reportCSV(reportData)
	} else {
		return reportTemplate(reportData)
	}
}

func reportCSV(libs []libraryData) error {
	writer := csv.NewWriter(os.Stdout)
	for _, lib := range libs {
		if err := writer.Write([]string{lib.Name, lib.LicenseURL, lib.LicenseName}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func reportTemplate(libs []libraryData) error {
	templateBytes, err := os.ReadFile(templateFile)
	if err != nil {
		return err
	}
	tmpl, err := template.New("").Parse(string(templateBytes))
	if err != nil {
		return err
	}
	return tmpl.Execute(os.Stdout, libs)
}
