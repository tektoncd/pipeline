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
	"reflect"

	"go.opencensus.io/stats/view"
	"knative.dev/pkg/test"
)

// CheckStatsReported checks that there is a view registered with the given name for each string in names,
// and that each view has at least one record.
func CheckStatsReported(t test.T, names ...string) {
	t.Helper()
	for _, name := range names {
		d, err := view.RetrieveData(name)
		if err != nil {
			t.Error("For metric, Reporter.Report() error", "metric", name, "error", err)
		}
		if len(d) < 1 {
			t.Error("For metric, no data reported when data was expected, view data is empty.", "metric", name)
		}
	}
}

// CheckStatsNotReported checks that there are no records for any views that a name matching a string in names.
// Names that do not match registered views are considered not reported.
func CheckStatsNotReported(t test.T, names ...string) {
	t.Helper()
	for _, name := range names {
		d, err := view.RetrieveData(name)
		// err == nil means a valid stat exists matching "name"
		// len(d) > 0 means a component recorded metrics for that stat
		if err == nil && len(d) > 0 {
			t.Error("For metric, unexpected data reported when no data was expected.", "metric", name, "Reporter len(d)", len(d))
		}
	}
}

// CheckCountData checks the view with a name matching string name to verify that the CountData stats
// reported are tagged with the tags in wantTags and that wantValue matches reported count.
func CheckCountData(t test.T, name string, wantTags map[string]string, wantValue int64) {
	t.Helper()
	if row := checkExactlyOneRow(t, name); row != nil {
		checkRowTags(t, row, name, wantTags)

		if s, ok := row.Data.(*view.CountData); !ok {
			t.Error("want CountData", "metric", name, "got", reflect.TypeOf(row.Data))
		} else if s.Value != wantValue {
			t.Error("Wrong value", "metric", name, "value", s.Value, "want", wantValue)
		}
	}
}

// CheckDistributionData checks the view with a name matching string name to verify that the DistributionData stats reported
// are tagged with the tags in wantTags and that expectedCount number of records were reported.
// It also checks that expectedMin and expectedMax match the minimum and maximum reported values, respectively.
func CheckDistributionData(t test.T, name string, wantTags map[string]string, expectedCount int64, expectedMin float64, expectedMax float64) {
	t.Helper()
	if row := checkExactlyOneRow(t, name); row != nil {
		checkRowTags(t, row, name, wantTags)

		if s, ok := row.Data.(*view.DistributionData); !ok {
			t.Error("want DistributionData", "metric", name, "got", reflect.TypeOf(row.Data))
		} else {
			if s.Count != expectedCount {
				t.Error("reporter count wrong", "metric", name, "got", s.Count, "want", expectedCount)
			}
			if s.Min != expectedMin {
				t.Error("reporter count wrong", "metric", name, "got", s.Min, "want", expectedMin)
			}
			if s.Max != expectedMax {
				t.Error("reporter count wrong", "metric", name, "got", s.Max, "want", expectedMax)
			}
		}
	}
}

// CheckDistributionRange checks the view with a name matching string name to verify that the DistributionData stats reported
// are tagged with the tags in wantTags and that expectedCount number of records were reported.
func CheckDistributionCount(t test.T, name string, wantTags map[string]string, expectedCount int64) {
	t.Helper()
	if row := checkExactlyOneRow(t, name); row != nil {
		checkRowTags(t, row, name, wantTags)

		if s, ok := row.Data.(*view.DistributionData); !ok {
			t.Error("want DistributionData", "metric", name, "got", reflect.TypeOf(row.Data))
		} else if s.Count != expectedCount {
			t.Error("reporter count wrong", "metric", name, "got", s.Count, "want", expectedCount)
		}
	}
}

// CheckLastValueData checks the view with a name matching string name to verify that the LastValueData stats
// reported are tagged with the tags in wantTags and that wantValue matches reported last value.
func CheckLastValueData(t test.T, name string, wantTags map[string]string, wantValue float64) {
	t.Helper()
	if row := lastRow(t, name); row != nil {
		checkRowTags(t, row, name, wantTags)

		if s, ok := row.Data.(*view.LastValueData); !ok {
			t.Error("want LastValueData", "metric", name, "got", reflect.TypeOf(row.Data))
		} else if s.Value != wantValue {
			t.Error("Reporter.Report() wrong value", "metric", name, "got", s.Value, "want", wantValue)
		}
	}
}

// CheckSumData checks the view with a name matching string name to verify that the SumData stats
// reported are tagged with the tags in wantTags and that wantValue matches the reported sum.
func CheckSumData(t test.T, name string, wantTags map[string]string, wantValue float64) {
	t.Helper()
	if row := checkExactlyOneRow(t, name); row != nil {
		checkRowTags(t, row, name, wantTags)

		if s, ok := row.Data.(*view.SumData); !ok {
			t.Error("Wrong type", "metric", name, "got", reflect.TypeOf(row.Data), "want", "SumData")
		} else if s.Value != wantValue {
			t.Error("Wrong sumdata", "metric", name, "got", s.Value, "want", wantValue)
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

func lastRow(t test.T, name string) *view.Row {
	t.Helper()
	d, err := view.RetrieveData(name)
	if err != nil {
		t.Error("Reporter.Report() error", "metric", name, "error", err)
		return nil
	}
	if len(d) < 1 {
		t.Error("Reporter.Report() wrong length", "metric", name, "got", len(d), "want at least", 1)
		return nil
	}

	return d[len(d)-1]
}

func checkExactlyOneRow(t test.T, name string) *view.Row {
	t.Helper()
	d, err := view.RetrieveData(name)
	if err != nil {
		t.Error("Reporter.Report() error", "metric", name, "error", err)
		return nil
	}
	if len(d) != 1 {
		t.Error("Reporter.Report() wrong length", "metric", name, "got", len(d), "want", 1)
		return nil
	}

	return d[0]
}

func checkRowTags(t test.T, row *view.Row, name string, wantTags map[string]string) {
	t.Helper()
	if wantlen, gotlen := len(wantTags), len(row.Tags); gotlen != wantlen {
		t.Error("Reporter got wrong number of tags", "metric", name, "got", gotlen, "want", wantlen)
	}
	for _, got := range row.Tags {
		n := got.Key.Name()
		if want, ok := wantTags[n]; !ok {
			t.Error("Reporter got an extra tag", "metric", name, "gotName", n, "gotValue", got.Value)
		} else if got.Value != want {
			t.Error("Reporter expected a different tag value for key", "metric", name, "key", n, "got", got.Value, "want", want)
		}
	}
}
