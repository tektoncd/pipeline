/*
Copyright 2025 The Tekton Authors

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

/*
This utility can be used to generate a new release name in the format:
	<cat breed> <robot name>

to be used for a Tekton Pipelines release.
It looks for cat breeds from CatAPIURL and it parses robot names out
of Wikipedia WikiURL. It filters names that have been used already,
based on the GitHub API GitHubReleasesURL

To use, run:
	go run release_names.go


Example output:
	{
		"release_name": "California Spangled Clank",
		"cat_breed_url": "https://en.wikipedia.org/wiki/California_Spangled",
		"robot_url": "https://en.wikipedia.org/wiki/Clank"
	}
*/

package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"regexp"
	"strings"
)

// API Endpoints
const (
	CatAPIURL         = "https://api.thecatapi.com/v1/breeds"
	RobotWikiURL      = "https://en.wikipedia.org/wiki/List_of_fictional_robots_and_androids"
	WikiURL           = "https://en.wikipedia.org/wiki/"
	GitHubReleasesURL = "https://api.github.com/repos/tektoncd/pipeline/releases"
)

// Structs to hold API responses
type CatBreed struct {
	Name string `json:"name"`
}

type Release struct {
	Name string `json:"name"`
}

func httpGet(url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

// Fetch cat breeds and organize them by first letter
func getCatBreeds() (map[string][]string, error) {
	resp, err := httpGet(CatAPIURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var breeds []CatBreed
	if err := json.NewDecoder(resp.Body).Decode(&breeds); err != nil {
		return nil, err
	}

	catDict := make(map[string][]string)
	for _, breed := range breeds {
		firstLetter := strings.ToUpper(string(breed.Name[0]))
		catDict[firstLetter] = append(catDict[firstLetter], breed.Name)
	}

	return catDict, nil
}

// Scrape Wikipedia for robot names
func getRobotNames() (map[string][]string, error) {
	resp, err := httpGet(RobotWikiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	robotDict := make(map[string][]string)

	// Regex to extract robot names from <li><b>Robot Name</b>
	re := regexp.MustCompile(`<li>\s*<b>\s*<a[^>]*>([^<]+)</a>\s*</b>`)
	matches := re.FindAllStringSubmatch(string(bodyBytes), -1)

	for _, match := range matches {
		if len(match) > 1 {
			name := strings.TrimSpace(match[1])
			firstLetter := strings.ToUpper(string(name[0]))
			robotDict[firstLetter] = append(robotDict[firstLetter], name)
		}
	}

	return robotDict, nil
}

// Fetch past releases from GitHub
func getPastReleases() (map[string]bool, error) {
	pastReleases := make(map[string]bool)
	page := 1
	perPage := 100

	// Loop until we get an page smaller than perPage (or empty)
	for {
		url := fmt.Sprintf("%s?per_page=%d&page=%d", GitHubReleasesURL, perPage, page)
		resp, err := httpGet(url)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch releases page %d: %w", page, err)
		}
		defer resp.Body.Close()

		var pageReleases []Release
		if err := json.NewDecoder(resp.Body).Decode(&pageReleases); err != nil {
			return nil, fmt.Errorf("failed to fetch releases page %d: %w", page, err)
		}

		// If we got an empty page, we've reached the end
		if len(pageReleases) == 0 {
			break
		}

		pastReleases := make(map[string]bool)
		for _, release := range pageReleases {
			pastReleases[release.Name] = true
		}

		// If we got fewer than the requested number, we've reached the end
		if len(pageReleases) < perPage {
			break
		}

		page++
	}

	return pastReleases, nil
}

func randomElement(array []string) (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(array))))
	if err != nil {
		return "", err
	}
	return array[n.Int64()], nil
}

// Generate a unique release name
func generateUniqueTuple() (string, string, error) {
	catBreeds, err := getCatBreeds()
	if err != nil {
		return "", "", err
	}

	robotNames, err := getRobotNames()
	if err != nil {
		return "", "", err
	}

	pastReleases, err := getPastReleases()
	if err != nil {
		return "", "", err
	}

	// Find common letters
	commonLetters := []string{}
	for letter := range catBreeds {
		if _, exists := robotNames[letter]; exists {
			commonLetters = append(commonLetters, letter)
		}
	}

	if len(commonLetters) == 0 {
		return "", "", errors.New("no matching names found")
	}

	maxAttempts := 10
	for range maxAttempts {
		chosenLetter, err := randomElement(commonLetters)
		if err != nil {
			return "", "", err
		}

		cat, err := randomElement(catBreeds[chosenLetter])
		if err != nil {
			return "", "", err
		}

		robot, err := randomElement(robotNames[chosenLetter])
		if err != nil {
			return "", "", err
		}

		newName := cat + " " + robot
		if !pastReleases[newName] {
			return cat, robot, nil
		}
	}

	return "", "", errors.New("could not generate a unique name after multiple attempts")
}

func printJsonError(err error) {
	fmt.Println(`{"error": "` + err.Error() + `"}`) //nolint:forbidigo
}

func main() {
	cat, robot, err := generateUniqueTuple()
	if err != nil {
		printJsonError(err)
		return
	}

	output := map[string]string{
		"release_name":  cat + " " + robot,
		"cat_breed_url": WikiURL + strings.ReplaceAll(cat, " ", "_"),
		"robot_url":     WikiURL + strings.ReplaceAll(robot, " ", "_"),
	}

	jsonOutput, err := json.MarshalIndent(output, "", "    ")
	if err != nil {
		printJsonError(err)
		return
	}
	fmt.Println(string(jsonOutput)) //nolint:forbidigo
}
