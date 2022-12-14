package sidecarlogresults

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestLookForResults_FanOutAndWait(t *testing.T) {
	for _, c := range []struct {
		desc    string
		results []SidecarLogResult
	}{{
		desc: "multiple results",
		results: []SidecarLogResult{{
			Name:  "foo",
			Value: "bar",
		}, {
			Name:  "foo2",
			Value: "bar2",
		}},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			dir := t.TempDir()
			resultNames := []string{}
			wantResults := []byte{}
			for _, result := range c.results {
				createResult(t, dir, result.Name, result.Value)
				resultNames = append(resultNames, result.Name)
				encodedResult, err := json.Marshal(result)
				if err != nil {
					t.Error(err)
				}
				// encode adds a newline character at the end.
				// We need to do the same before comparing
				encodedResult = append(encodedResult, '\n')
				wantResults = append(wantResults, encodedResult...)
			}
			dir2 := t.TempDir()
			createRun(t, dir2, false)
			got := new(bytes.Buffer)
			err := LookForResults(got, dir2, dir, resultNames)
			if err != nil {
				t.Fatalf("Did not expect any error but got: %v", err)
			}
			// sort because the order of results is not always the same because of go routines.
			sort.Slice(wantResults, func(i int, j int) bool { return wantResults[i] < wantResults[j] })
			sort.Slice(got.Bytes(), func(i int, j int) bool { return got.Bytes()[i] < got.Bytes()[j] })
			if d := cmp.Diff(wantResults, got.Bytes()); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}

func TestLookForResults(t *testing.T) {
	for _, c := range []struct {
		desc         string
		resultName   string
		resultValue  string
		createResult bool
		stepError    bool
	}{{
		desc:         "good result",
		resultName:   "foo",
		resultValue:  "bar",
		createResult: true,
		stepError:    false,
	}, {
		desc:         "empty result",
		resultName:   "foo",
		resultValue:  "",
		createResult: true,
		stepError:    true,
	}, {
		desc:         "missing result",
		resultName:   "missing",
		resultValue:  "",
		createResult: false,
		stepError:    false,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			dir := t.TempDir()
			if c.createResult == true {
				createResult(t, dir, c.resultName, c.resultValue)
			}
			dir2 := t.TempDir()
			createRun(t, dir2, c.stepError)

			var want []byte
			if c.createResult == true {
				// This is the expected result
				result := SidecarLogResult{
					Name:  c.resultName,
					Value: c.resultValue,
				}
				encodedResult, err := json.Marshal(result)
				if err != nil {
					t.Error(err)
				}
				// encode adds a newline character at the end.
				// We need to do the same before comparing
				encodedResult = append(encodedResult, '\n')
				want = encodedResult
			}
			got := new(bytes.Buffer)
			err := LookForResults(got, dir2, dir, []string{c.resultName})
			if err != nil {
				t.Fatalf("Did not expect any error but got: %v", err)
			}
			if d := cmp.Diff(want, got.Bytes()); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}

func createResult(t *testing.T, dir string, resultName string, resultValue string) {
	t.Helper()
	resultFile := filepath.Join(dir, resultName)
	err := os.WriteFile(resultFile, []byte(resultValue), 0644)
	if err != nil {
		t.Fatal(err)
	}
}

func createRun(t *testing.T, dir string, causeErr bool) {
	t.Helper()
	stepFile := filepath.Join(dir, "1")
	_ = os.Mkdir(stepFile, 0755)
	stepFile = filepath.Join(stepFile, "out")
	if causeErr {
		stepFile += ".err"
	}
	err := os.WriteFile(stepFile, []byte(""), 0644)
	if err != nil {
		t.Fatal(err)
	}
}
