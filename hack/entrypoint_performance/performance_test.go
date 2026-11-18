//go:build entrypoint_performance && linux

/*
Copyright 2026 The Tekton Authors

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

package entrypoint

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

var (
	baselinePath     = flag.String("baseline", "", "Baseline entrypoint executable")
	candidatePath    = flag.String("candidate", "", "Candidate entrypoint executable")
	reportPath       = flag.String("report", "", "Output JSON file including raw samples")
	rounds           = flag.Int("rounds", 30, "Even number of paired rounds, at least 10")
	maxSizeMiB       = flag.Float64("max-size-mib", 24, "Candidate executable size ceiling in MiB")
	maxRSSMiB        = flag.Float64("max-rss-mib", 24, "Candidate median live RSS ceiling in MiB")
	maxSizeGrowth    = flag.Float64("max-size-growth", 10, "Maximum executable growth over baseline, percent")
	maxStartupGrowth = flag.Float64("max-startup-growth", 0, "Optional startup growth limit, percent; zero reports only")
)

type distribution struct {
	Median  float64   `json:"median"`
	Q1      float64   `json:"q1"`
	Q3      float64   `json:"q3"`
	Samples []float64 `json:"samples"`
}

type measurements struct {
	Bytes     int64        `json:"bytes"`
	SHA256    string       `json:"sha256"`
	StartupMS distribution `json:"startup_ms"`
	RSSKiB    distribution `json:"rss_kib"`
}

func TestEntrypointPerformance(t *testing.T) {
	if *baselinePath == "" || *candidatePath == "" || *reportPath == "" {
		t.Fatal("-baseline, -candidate and -report are required")
	}
	if *rounds < 10 || *rounds%2 != 0 {
		t.Fatal("-rounds must be even and at least 10")
	}
	for _, value := range []float64{*maxSizeMiB, *maxRSSMiB, *maxSizeGrowth, *maxStartupGrowth} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			t.Fatal("budgets must be finite and nonnegative")
		}
	}
	if *maxSizeMiB == 0 || *maxRSSMiB == 0 {
		t.Fatal("size and RSS ceilings must be positive")
	}
	t.Setenv("GOMAXPROCS", "4")
	t.Setenv("GODEBUG", "")
	t.Setenv("HOME", t.TempDir())
	paths := []string{*baselinePath, *candidatePath}
	stats := make([]measurements, len(paths))
	for i, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		stats[i].Bytes = int64(len(data))
		stats[i].SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
		for range 3 {
			measureStartup(t, path)
			measureRSS(t, path)
		}
	}
	// Alternate AB/BA order so neither executable always benefits from going second.
	for round := range *rounds {
		for offset := range paths {
			i := (round + offset) % len(paths)
			stats[i].StartupMS.Samples = append(stats[i].StartupMS.Samples, measureStartup(t, paths[i]))
			stats[i].RSSKiB.Samples = append(stats[i].RSSKiB.Samples, measureRSS(t, paths[i]))
		}
	}
	for i := range stats {
		stats[i].StartupMS = summarize(stats[i].StartupMS.Samples)
		stats[i].RSSKiB = summarize(stats[i].RSSKiB.Samples)
	}
	report := struct {
		Platform  string       `json:"platform"`
		GoVersion string       `json:"go_version"`
		Rounds    int          `json:"rounds"`
		Baseline  measurements `json:"baseline"`
		Candidate measurements `json:"candidate"`
	}{runtime.GOOS + "/" + runtime.GOARCH, runtime.Version(), *rounds, stats[0], stats[1]}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(*reportPath, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("metric                 baseline     candidate\nbinary MiB           %10.3f %13.3f\nlive RSS MiB         %10.3f %13.3f\nwarm help launch ms  %10.3f %13.3f",
		float64(stats[0].Bytes)/(1<<20), float64(stats[1].Bytes)/(1<<20),
		stats[0].RSSKiB.Median/1024, stats[1].RSSKiB.Median/1024,
		stats[0].StartupMS.Median, stats[1].StartupMS.Median)
	if float64(stats[1].Bytes) > *maxSizeMiB*(1<<20) {
		t.Errorf("candidate executable exceeds %.1f MiB", *maxSizeMiB)
	}
	if float64(stats[1].Bytes) > float64(stats[0].Bytes)*(1+*maxSizeGrowth/100) {
		t.Errorf("candidate executable grew by more than %.1f%%", *maxSizeGrowth)
	}
	if stats[1].RSSKiB.Median > *maxRSSMiB*1024 {
		t.Errorf("candidate median live RSS exceeds %.1f MiB", *maxRSSMiB)
	}
	// A 3 ms floor avoids treating small absolute differences as timing regressions.
	if *maxStartupGrowth > 0 && stats[1].StartupMS.Median > stats[0].StartupMS.Median+math.Max(3, stats[0].StartupMS.Median*(*maxStartupGrowth)/100) {
		t.Errorf("candidate warm help launch exceeds %.1f%% growth and the 3 ms noise floor", *maxStartupGrowth)
	}
}

func measureStartup(t *testing.T, binary string) float64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-help")
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	start := time.Now()
	if err := cmd.Run(); err != nil {
		t.Fatalf("help launch %s: %v", binary, err)
	}
	return float64(time.Since(start)) / float64(time.Millisecond)
}

func measureRSS(t *testing.T, binary string) float64 {
	t.Helper()
	work := t.TempDir()
	gatePath := filepath.Join(work, "release")
	if err := syscall.Mkfifo(gatePath, 0o600); err != nil {
		t.Fatal(err)
	}
	gate, err := os.OpenFile(gatePath, os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary,
		"-enable_spire=false", "-entrypoint", "/bin/sh",
		"-step_metadata_dir", filepath.Join(work, "metadata"),
		"-termination_path", filepath.Join(work, "termination"),
		"-post_file", filepath.Join(work, "out"),
		"--", "-c", `printf 'READY\n'; IFS= read -r release < "$1"`, "sh", gatePath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.WaitDelay = time.Second
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		// Release the wrapped child even when readiness or RSS collection fails.
		if _, err := gate.WriteString("release\n"); err != nil {
			t.Error(err)
		}
		if err := cmd.Wait(); err != nil {
			t.Errorf("wrapped command: %v\n%s", err, stderr.String())
		}
	}()
	deadline, _ := ctx.Deadline()
	pipe, ok := stdout.(*os.File)
	if !ok {
		t.Fatal("stdout is not a pipe")
	}
	if err := pipe.SetReadDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || line != "READY\n" {
		t.Fatalf("wrapped command readiness: %q, %v", line, err)
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) != 3 || fields[2] != "kB" {
				t.Fatalf("invalid RSS: %q", line)
			}
			rss, err := strconv.ParseFloat(fields[1], 64)
			if err != nil || rss <= 0 {
				t.Fatalf("invalid RSS: %q, %v", line, err)
			}
			return rss
		}
	}
	t.Fatal("entrypoint has no live RSS")
	return 0
}

func summarize(samples []float64) distribution {
	sorted := slices.Sorted(slices.Values(samples))
	median := func(values []float64) float64 {
		middle := len(values) / 2
		if len(values)%2 == 0 {
			return (values[middle-1] + values[middle]) / 2
		}
		return values[middle]
	}
	return distribution{median(sorted), median(sorted[:len(sorted)/2]), median(sorted[(len(sorted)+1)/2:]), samples}
}
