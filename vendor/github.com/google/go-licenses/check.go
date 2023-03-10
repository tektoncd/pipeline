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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-licenses/licenses"
	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	checkHelp = "Checks whether licenses for a package are not allowed."
	checkCmd  = &cobra.Command{
		Use:   "check <package> [package...]",
		Short: checkHelp,
		Long:  checkHelp + packageHelp,
		Args:  cobra.MinimumNArgs(1),
		RunE:  checkMain,
	}

	allowedLicenses []string
	disallowedTypes []string
)

func init() {
	checkCmd.Flags().StringSliceVar(&allowedLicenses, "allowed_licenses", []string{}, "list of allowed license names, can't be used in combination with disallowed_types")
	checkCmd.Flags().StringSliceVar(&disallowedTypes, "disallowed_types", []string{}, "list of disallowed license types, can't be used in combination with allowed_licenses (default: forbidden, unknown)")

	rootCmd.AddCommand(checkCmd)
}

func checkMain(_ *cobra.Command, args []string) error {
	var disallowedLicenseTypes []licenses.Type

	allowedLicenseNames := getAllowedLicenseNames()
	disallowedLicenseTypes = getDisallowedLicenseTypes()

	hasLicenseNames := len(allowedLicenseNames) > 0
	hasLicenseType := len(disallowedLicenseTypes) > 0

	if hasLicenseNames && hasLicenseType {
		return errors.New("allowed_licenses && disallowed_types can't be used at the same time")
	}

	if !hasLicenseNames && !hasLicenseType {
		// fallback to original behaviour to avoid breaking changes
		disallowedLicenseTypes = []licenses.Type{licenses.Forbidden, licenses.Unknown}
		hasLicenseType = true
	}

	classifier, err := licenses.NewClassifier(confidenceThreshold)
	if err != nil {
		return err
	}

	libs, err := licenses.Libraries(context.Background(), classifier, includeTests, ignore, args...)
	if err != nil {
		return err
	}

	// indicate that a forbidden license was found
	found := false

	for _, lib := range libs {
		licenseName, licenseType, err := classifier.Identify(lib.LicensePath)
		if err != nil {
			return err
		}

		if hasLicenseNames && !isAllowedLicenseName(licenseName, allowedLicenseNames) {
			fmt.Fprintf(os.Stderr, "Not allowed license %s found for library %v\n", licenseName, lib)
			found = true
		}

		if hasLicenseType && isDisallowedLicenseType(licenseType, disallowedLicenseTypes) {
			fmt.Fprintf(
				os.Stderr,
				"%s license type %s found for library %v\n",
				cases.Title(language.English).String(licenseType.String()),
				licenseName,
				lib)
			found = true
		}
	}

	if found {
		os.Exit(1)
	}

	return nil
}

func getDisallowedLicenseTypes() []licenses.Type {
	if len(disallowedTypes) == 0 {
		return []licenses.Type{}
	}

	excludedLicenseTypes := make([]licenses.Type, 0)

	for _, v := range disallowedTypes {
		switch strings.TrimSpace(strings.ToLower(v)) {
		case "forbidden":
			excludedLicenseTypes = append(excludedLicenseTypes, licenses.Forbidden)
		case "notice":
			excludedLicenseTypes = append(excludedLicenseTypes, licenses.Notice)
		case "permissive":
			excludedLicenseTypes = append(excludedLicenseTypes, licenses.Permissive)
		case "reciprocal":
			excludedLicenseTypes = append(excludedLicenseTypes, licenses.Reciprocal)
		case "restricted":
			excludedLicenseTypes = append(excludedLicenseTypes, licenses.Restricted)
		case "unencumbered":
			excludedLicenseTypes = append(excludedLicenseTypes, licenses.Unencumbered)
		case "unknown":
			excludedLicenseTypes = append(excludedLicenseTypes, licenses.Unknown)
		default:
			fmt.Fprintf(
				os.Stderr,
				"Unknown license type '%s' provided.\n"+
					"Allowed types: forbidden, notice, permissive, reciprocal, restricted, unencumbered, unknown\n",
				v)
		}
	}

	return excludedLicenseTypes
}

func isDisallowedLicenseType(licenseType licenses.Type, excludedLicenseTypes []licenses.Type) bool {
	for _, excluded := range excludedLicenseTypes {
		if excluded == licenseType {
			return true
		}
	}

	return false
}

func getAllowedLicenseNames() []string {
	if len(allowedLicenses) == 0 {
		return []string{}
	}

	var allowed []string

	for _, licenseName := range allowedLicenses {
		allowed = append(allowed, strings.TrimSpace(licenseName))
	}

	return allowed
}

func isAllowedLicenseName(licenseName string, allowedLicenseNames []string) bool {
	for _, allowed := range allowedLicenseNames {
		if allowed == licenseName {
			return true
		}
	}

	return false
}
