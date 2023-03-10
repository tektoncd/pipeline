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
	"github.com/spf13/cobra"
)

var (
	csvHelp = "Prints all licenses that apply to one or more Go packages and their dependencies. (Deprecated: use report instead)"
	csvCmd  = &cobra.Command{
		Use:   "csv <package> [package...]",
		Short: csvHelp,
		Long:  csvHelp + packageHelp,
		Args:  cobra.MinimumNArgs(1),
		RunE:  csvMain,
	}
)

func init() {
	rootCmd.AddCommand(csvCmd)
}

func csvMain(_ *cobra.Command, args []string) error {
	// without a --template flag, reportMain will output CSV
	return reportMain(nil, args)
}
