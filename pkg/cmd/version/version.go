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
	"io/ioutil"
	"log"
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
		Annotations: map[string]string{
			"commandType": "utility",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "Client version: %s\n", clientVersion)

			if !check || clientVersion == devVersion {
				return nil
			}

			client := NewClient(time.Duration(3 * time.Second))
			output, err := checkRelease(client)
			fmt.Fprintf(cmd.OutOrStdout(), output)
			return err
		},
	}

	cmd.Flags().BoolVarP(&check, "check", "c", false, "check if a newer version is available")
	return cmd
}

type GHVersion struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

type Option func(*Client)

type Client struct {
	httpClient *http.Client
}

func SetHTTPClient(httpClient *http.Client) Option {
	return func(cli *Client) {
		cli.httpClient = httpClient
	}
}

func NewClient(timeout time.Duration, options ...Option) *Client {
	cli := Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}

	for i := range options {
		options[i](&cli)
	}
	return &cli
}

func (cli *Client) getRelease(url string) (ghversion GHVersion, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ghversion, errors.Wrap(err, "failed to fetch the latest version")
	}

	res, err := cli.httpClient.Do(req)
	defer func() {
		err := res.Body.Close()
		if err != nil {
			log.Fatal(err)
		}
	}()
	if err != nil {
		return ghversion, errors.Wrap(err, "request failed")
	}

	if res.StatusCode != http.StatusOK {
		return ghversion, fmt.Errorf("invalid http status %d, error: %s", res.StatusCode, res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return ghversion, errors.Wrap(err, "failed to read the latest version response body")
	}
	response := GHVersion{}

	if err := json.Unmarshal(body, &response); err != nil {
		return ghversion, errors.Wrap(err, "failed to unmarshal the latest version response body")
	}

	return response, nil
}

func checkRelease(client *Client) (string, error) {
	response, err := client.getRelease(latestReleaseURL)
	if err != nil {
		return "", err
	}

	latest, err := parseVersion(response.TagName)
	if err != nil {
		return "", err
	}

	current, err := parseVersion(clientVersion)
	if err != nil {
		return "", err
	}

	if current.LT(*latest) {
		return fmt.Sprintf("A newer version (v%s) of Tekton CLI is available, please check %s\n", latest, response.HTMLURL), nil
	}

	return fmt.Sprintf("You are running the latest version (v%s) of Tekton CLI\n", latest), nil
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
