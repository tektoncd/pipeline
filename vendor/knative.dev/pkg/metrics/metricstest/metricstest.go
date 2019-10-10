/*
Copyright 2019 The Knative Authors
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

package metricstest

import (
	"testing"

	"go.opencensus.io/stats/view"
)

// CheckStatsReported checks that there is a view registered with the given name for each string in names,
// and that each view has at least one record.
func CheckStatsReported(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		d, err := view.RetrieveData(name)
		if err != nil {
			t.Errorf("For metric %s: Reporter.Report() error = %v", name, err)
		}
		if len(d) < 1 {
			t.Errorf("For metric %s: No data reported when data was expected, view data is empty.", name)
		}
	}
}

// CheckStatsNotReported checks that there are no records for any views that a name matching a string in names.
// Names that do not match registered views are considered not reported.
func CheckStatsNotReported(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		d, err := view.RetrieveData(name)
		// err == nil means a valid stat exists matching "name"
		// len(d) > 0 means a component recorded metrics for that stat
		if err == nil && len(d) > 0 {
			t.Errorf("For metric %s: Unexpected data reported when no data was expected. Reporter len(d) = %d", name, len(d))
		}
	}
}

// CheckCountData checks the view with a name matching string name to verify that the CountData stats
// reported are tagged with the tags in wantTags and that wantValue matches reported count.
func CheckCountData(t *testing.T, name string, wantTags map[string]string, wantValue int64) {
	t.Helper()
	if row := checkExactlyOneRow(t, name, wantTags); row != nil {
		checkRowTags(t, row, name, wantTags)

		if s, ok := row.Data.(*view.CountData); !ok {
			t.Errorf("%s: got %T, want CountData", name, row.Data)
		} else if s.Value != wantValue {
			t.Errorf("For metric %s: value = %v, want: %d", name, s.Value, wantValue)
		}
	}
}

// CheckDistributionData checks the view with a name matching string name to verify that the DistributionData stats reported
// are tagged with the tags in wantTags and that expectedCount number of records were reported.
// It also checks that expectedMin and expectedMax match the minimum and maximum reported values, respectively.
func CheckDistributionData(t *testing.T, name string, wantTags map[string]string, expectedCount int64, expectedMin float64, expectedMax float64) {
	t.Helper()
	if row := checkExactlyOneRow(t, name, wantTags); row != nil {
		checkRowTags(t, row, name, wantTags)

		if s, ok := row.Data.(*view.DistributionData); !ok {
			t.Errorf("%s: got %T, want DistributionData", name, row.Data)
		} else {
			if s.Count != expectedCount {
				t.Errorf("For metric %s: reporter count = %d, want = %d", name, s.Count, expectedCount)
			}
			if s.Min != expectedMin {
				t.Errorf("For metric %s: reporter count = %f, want = %f", name, s.Min, expectedMin)
			}
			if s.Max != expectedMax {
				t.Errorf("For metric %s: reporter count = %f, want = %f", name, s.Max, expectedMax)
			}
		}
	}
}

// CheckLastValueData checks the view with a name matching string name to verify that the LastValueData stats
// reported are tagged with the tags in wantTags and that wantValue matches reported last value.
func CheckLastValueData(t *testing.T, name string, wantTags map[string]string, wantValue float64) {
	t.Helper()
	if row := checkExactlyOneRow(t, name, wantTags); row != nil {
		checkRowTags(t, row, name, wantTags)

		if s, ok := row.Data.(*view.LastValueData); !ok {
			t.Errorf("%s: got %T, want LastValueData", name, row.Data)
		} else if s.Value != wantValue {
			t.Errorf("For metric %s: Reporter.Report() expected %v got %v", name, s.Value, wantValue)
		}
	}
}

// CheckSumData checks the view with a name matching string name to verify that the SumData stats
// reported are tagged with the tags in wantTags and that wantValue matches the reported sum.
func CheckSumData(t *testing.T, name string, wantTags map[string]string, wantValue float64) {
	t.Helper()
	if row := checkExactlyOneRow(t, name, wantTags); row != nil {
		checkRowTags(t, row, name, wantTags)

		if s, ok := row.Data.(*view.SumData); !ok {
			t.Errorf("%s: got %T, want SumData", name, row.Data)
		} else if s.Value != wantValue {
			t.Errorf("For metric %s: value = %v, want: %v", name, s.Value, wantValue)
		}
	}
}

// Unregister unregisters the metrics that were registered.
// This is useful for testing since golang execute test iterations within the same process and
// opencensus views maintain global state. At the beginning of each test, tests should
// unregister for all metrics and then re-register for the same metrics. This effectively clears
// out any existing data and avoids a panic due to re-registering a metric.
//
// In normal process shutdown, metrics do not need to be unregistered.
func Unregister(names ...string) {
	for _, n := range names {
		if v := view.Find(n); v != nil {
			view.Unregister(v)
		}
	}
}

func checkExactlyOneRow(t *testing.T, name string, wantTags map[string]string) *view.Row {
	t.Helper()
	d, err := view.RetrieveData(name)
	if err != nil {
		t.Errorf("For metric %s: Reporter.Report() error = %v", name, err)
		return nil
	}
	if len(d) != 1 {
		t.Errorf("For metric %s: Reporter.Report() len(d)=%v, want 1", name, len(d))
	}

	return d[0]
}

func checkRowTags(t *testing.T, row *view.Row, name string, wantTags map[string]string) {
	t.Helper()
	for _, got := range row.Tags {
		n := got.Key.Name()
		if want, ok := wantTags[n]; !ok {
			t.Errorf("For metric %s: Reporter got an extra tag %v: %v", name, n, got.Value)
		} else if got.Value != want {
			t.Errorf("For metric %s: Reporter expected a different tag value for key: %s, got: %s, want: %s", name, n, got.Value, want)
		}
	}
}
