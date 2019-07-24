// Copyright © 2019 The Tekton Authors.
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

package version

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// NOTE: use go build -ldflags "-X github.com/tektoncd/cli/pkg/cmd/version.clientVersion=$(git describe)"
var clientVersion = devVersion

const devVersion = "dev"
const latestReleaseURL = "https://api.github.com/repos/tektoncd/cli/releases/latest"

// Command returns version command
func Command() *cobra.Command {
	var check bool

	var cmd = &cobra.Command{
		Use:   "version",
		Short: "Prints version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "Client version: %s\n", clientVersion)

			if !check || clientVersion == devVersion {
				return nil
			}

			return checkRelease(cmd.OutOrStdout())
		},
	}

	cmd.Flags().BoolVarP(&check, "check", "c", false, "check if a newer version is available")
	return cmd
}

func checkRelease(outStream io.Writer) error {
	client := http.Client{Timeout: time.Duration(3 * time.Second)}

	res, err := client.Get(latestReleaseURL)
	if err != nil {
		return errors.Wrap(err, "failed to fetch the latest version")
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid http status %d, error: %s", res.StatusCode, res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read the latest version response body")
	}

	var response struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return errors.Wrap(err, "failed to unmarshal the latest version response body")
	}

	latest, err := parseVersion(response.TagName)
	if err != nil {
		return err
	}

	current, err := parseVersion(clientVersion)
	if err != nil {
		return err
	}

	if current.LT(*latest) {
		fmt.Fprintf(outStream, "A newer version (v%s) of Tekton CLI is available, please check %s\n", latest, response.HTMLURL)
	} else {
		fmt.Fprintf(outStream, "You are running the latest version (v%s) of Tekton CLI\n", latest)
	}

	return nil
}

func parseVersion(version string) (*semver.Version, error) {
	version = strings.TrimSpace(version)
	// Strip the leading 'v' in the version strings
	v, err := semver.Parse(strings.TrimLeft(version, "v"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse version")
	}
	return &v, nil
}
