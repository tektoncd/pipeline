/*
Copyright 2018 The Knative Authors

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

package changeset

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	commitIDFile      = "HEAD"
	koDataPathEnvName = "KO_DATA_PATH"
	// packedRefsFile is a file containing a list of refs, used to compact the
	// list of refs instead of storing them on the filesystem directly.
	// See https://git-scm.com/docs/git-pack-refs
	packedRefsFile = "packed-refs"
)

var commitIDRE = regexp.MustCompile(`^[a-f0-9]{40}$`)

// Get tries to fetch the first 7 digitals of GitHub commit ID from HEAD file in
// KO_DATA_PATH. If it fails, it returns the error it gets.
func Get() (string, error) {
	data, err := readFileFromKoData(commitIDFile)
	if err != nil {
		return "", err
	}
	commitID := strings.TrimSpace(string(data))
	if rID := strings.TrimPrefix(commitID, "ref: "); rID != commitID {
		// First try to read from the direct ref file - e.g. refs/heads/main
		data, err := readFileFromKoData(rID)
		if err != nil {
			if !os.IsNotExist(err) {
				return "", err
			}

			// Ref file didn't exist - it might be contained in the packed-refs
			// file.
			var pferr error
			data, pferr = findPackedRef(rID)
			// Only return the sub-error if the packed-refs file exists, otherwise
			// just let the original error return (e.g. treat it as if we didn't
			// even attempt the operation). This is primarily to keep the error
			// messages clean.
			if pferr != nil {
				if os.IsNotExist(pferr) {
					return "", err
				}
				return "", pferr
			}
		}
		commitID = strings.TrimSpace(string(data))
	}
	if commitIDRE.MatchString(commitID) {
		return commitID[:7], nil
	}
	return "", fmt.Errorf("%q is not a valid GitHub commit ID", commitID)
}

// readFileFromKoData tries to read data as string from the file with given name
// under KO_DATA_PATH then returns the content as string. The file is expected
// to be wrapped into the container from /kodata by ko. If it fails, returns
// the error it gets.
func readFileFromKoData(filename string) ([]byte, error) {
	f, err := koDataFile(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ioutil.ReadAll(f)
}

// readFileFromKoData tries to open the file with given name under KO_DATA_PATH.
// The file is expected to be wrapped into the container from /kodata by ko.
// If it fails, returns the error it gets.
func koDataFile(filename string) (*os.File, error) {
	koDataPath := os.Getenv(koDataPathEnvName)
	if koDataPath == "" {
		return nil, fmt.Errorf("%q does not exist or is empty", koDataPathEnvName)
	}
	return os.Open(filepath.Join(koDataPath, filename))
}

// findPackedRef searches the packed-ref file for ref values.
// This can happen if the # of refs in a repo grows too much - git will try
// and condense them into a file.
// See https://git-scm.com/docs/git-pack-refs
func findPackedRef(ref string) ([]byte, error) {
	f, err := koDataFile(packedRefsFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// We only care about lines with `<commit> <ref>` pairs.
		// Why this might happen:
		// 1. Lines starting with ^ refer to unpeeled tag SHAs
		// 	  (e.g. the commits pointed to by annotated tags)
		s := strings.Split(scanner.Text(), " ")
		if len(s) != 2 {
			continue
		}
		if ref == s[1] {
			return []byte(s[0]), nil
		}
	}
	return nil, fmt.Errorf("%q ref not found in packed-refs", ref)
}
