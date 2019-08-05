/*
Copyright 2018 Google, Inc. All rights reserved.

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
package fetcher

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/cloud-builders/gcs-fetcher/pkg/common"
	"google.golang.org/api/googleapi"
)

var (
	// sourceExt is a best-effort to identify files that should have a short
	// download time and thus get a short timeout for the first tries.
	sourceExt = map[string]bool{
		".js":   true,
		".py":   true,
		".php":  true,
		".java": true,
		".go":   true,
		".cs":   true,
		".rb":   true,
		".css":  true,
		".vb":   true,
		".pl":   true, // Perl. C'mon, live a little.
	}
	sourceTimeout = map[int]time.Duration{ // try number -> timeout duration
		0: 1 * time.Second,
		1: 2 * time.Second,
	}
	notSourceTimeout = map[int]time.Duration{ // try number -> timeout duration
		0: 3 * time.Second,
		1: 6 * time.Second,
	}
	defaultTimeout = 1 * time.Hour
	noTimeout      = 0 * time.Second
	errGCSTimeout  = errors.New("GCS timeout")

	robotRegex = regexp.MustCompile(`<Details>(\S+@\S+)\s`)
)

type sizeBytes int64

// job is a file to download, corresponds to an entry in the manifest file.
type job struct {
	filename        string
	bucket, object  string
	generation      int64
	sha1sum         string
	destDirOverride string
}

// jobAttempt is an attempt to download a particular file, may result in
// success or failure (indicated by err).
type jobAttempt struct {
	started    time.Time
	duration   time.Duration
	err        error
	gcsTimeout time.Duration
}

// jobReport stores all the details about the attempts to download a
// particular file.
type jobReport struct {
	job       job
	started   time.Time
	completed time.Time
	size      sizeBytes
	attempts  []jobAttempt
	success   bool
	finalname string
	err       error
}

type fetchOnceResult struct {
	size sizeBytes
	err  error
}

type stats struct {
	workers     int
	files       int
	size        sizeBytes
	duration    time.Duration
	retries     int
	gcsTimeouts int
	success     bool
	errs        []error
}

// OS allows us to inject dependencies to facilitate testing.
type OS interface {
	Rename(oldpath, newpath string) error
	Chmod(name string, mode os.FileMode) error
	Create(name string) (*os.File, error)
	MkdirAll(path string, perm os.FileMode) error
	Open(name string) (*os.File, error)
	RemoveAll(path string) error
}

// GCS allows us to inject dependencies to facilitate testing.
type GCS interface {
	NewReader(ctx context.Context, bucket, object string) (io.ReadCloser, error)
}

// Fetcher is the main workhorse of this package and does all the heavy lifting.
type Fetcher struct {
	GCS GCS
	OS  OS

	DestDir    string
	StagingDir string

	// mu guards CreatedDirs
	mu          sync.Mutex
	CreatedDirs map[string]bool

	SourceType     string
	Bucket, Object string
	Generation     int64

	TimeoutGCS  bool
	WorkerCount int
	Retries     int
	Backoff     time.Duration
	Verbose     bool
	Stdout      io.Writer
	Stderr      io.Writer
}

type permissionError struct {
	bucket string
	robot  string
}

func (e *permissionError) Error() string {
	return fmt.Sprintf("Access to bucket %s denied. You must grant Storage Object Viewer permission to %s.", e.bucket, e.robot)
}

func logit(writer io.Writer, format string, a ...interface{}) {
	if _, err := fmt.Fprintf(writer, format+"\n", a...); err != nil {
		log.Printf("Failed to write message: "+format, a...)
	}
}

func (gf *Fetcher) log(format string, a ...interface{}) {
	logit(gf.Stdout, format, a...)
}

func (gf *Fetcher) logErr(format string, a ...interface{}) {
	logit(gf.Stderr, format, a...)
}

func (gf *Fetcher) recordFailure(j job, started time.Time, gcsTimeout time.Duration, err error, report *jobReport) {
	attempt := jobAttempt{
		started:    started,
		duration:   time.Since(started),
		err:        err,
		gcsTimeout: gcsTimeout,
	}
	report.success = false
	report.err = err // Hold the latest error.
	report.attempts = append(report.attempts, attempt)

	isLast := len(report.attempts) == gf.Retries
	if gf.Verbose || isLast {
		retryMsg := ", will retry"
		if isLast {
			retryMsg = ", will no longer retry"
		}
		gf.log("Failed to fetch %s%s: %v", formatGCSName(j.bucket, j.object, j.generation), retryMsg, err)
	}
}

func (gf *Fetcher) recordSuccess(j job, started time.Time, size sizeBytes, finalname string, report *jobReport) {
	attempt := jobAttempt{
		started:  started,
		duration: time.Since(started),
	}
	report.success = true
	report.err = nil
	report.size = size
	report.attempts = append(report.attempts, attempt)
	report.finalname = finalname

	mibps := math.MaxFloat64
	if attempt.duration > 0 {
		mibps = (float64(report.size) / 1024 / 1024) / attempt.duration.Seconds()
	}
	if gf.Verbose {
		log.Printf("Fetched %s (%dB in %v, %.2fMiB/s)", formatGCSName(j.bucket, j.object, j.generation), report.size, attempt.duration, mibps)
	}
}

// fetchObject is responsible for trying (and retrying) to fetch a single file
// from GCS. It first downloads the file to a temp file, then renames it to
// the final location and sets the permissions on the final file.
func (gf *Fetcher) fetchObject(ctx context.Context, j job) *jobReport {
	report := &jobReport{job: j, started: time.Now()}
	defer func() {
		report.completed = time.Now()
	}()

	var tmpfile string
	var backoff time.Duration

	// Within a manifest, multiple files may have the same SHA. This can lead
	// to a race condition within the goworkers that are downloading the files
	// concurrently. To mitigate this issue, we add some randomness to the name
	// of the temp file being pulled.
	fuzz := rand.Intn(999999)

	for retrynum := 0; retrynum <= gf.Retries; retrynum++ {
		// Apply appropriate retry backoff.
		if retrynum > 0 {
			if retrynum == 1 {
				backoff = gf.Backoff
			} else {
				backoff *= 2
			}
			time.Sleep(backoff)
		}

		started := time.Now()

		// Download to temp location [DestDir]/[StagingDir]/[Bucket]-[Object]-[fuzz]-[retry]
		// If fetchObjectOnceWithTimeout() times out, this file will be orphaned and we can
		// clean it up later.
		tmpfile = filepath.Join(gf.StagingDir, fmt.Sprintf("%s-%s-%d-%d", j.bucket, j.object, fuzz, retrynum))
		if err := gf.ensureFolders(tmpfile); err != nil {
			e := fmt.Errorf("creating folders for temp file %q: %v", tmpfile, err)
			gf.recordFailure(j, started, noTimeout, e, report)
			continue
		}

		allowedGCSTimeout := gf.timeout(j.filename, retrynum)
		size, err := gf.fetchObjectOnceWithTimeout(ctx, j, allowedGCSTimeout, tmpfile)
		if err != nil {
			// Allow permissionError to bubble up.
			e := err
			if _, ok := err.(*permissionError); !ok {
				e = fmt.Errorf("fetching %q with timeout %v to temp file %q: %v", formatGCSName(j.bucket, j.object, j.generation), allowedGCSTimeout, tmpfile, err)
			}
			gf.recordFailure(j, started, allowedGCSTimeout, e, report)
			continue
		}

		// Rename the temp file to the final filename
		dest := gf.DestDir
		if j.destDirOverride != "" {
			dest = j.destDirOverride
		}
		finalname := filepath.Join(dest, j.filename)
		if err := gf.ensureFolders(finalname); err != nil {
			e := fmt.Errorf("creating folders for final file %q: %v", finalname, err)
			gf.recordFailure(j, started, noTimeout, e, report)
			continue
		}
		if err := gf.OS.Rename(tmpfile, finalname); err != nil {
			e := fmt.Errorf("renaming %q to %q: %v", tmpfile, finalname, err)
			gf.recordFailure(j, started, noTimeout, e, report)
			continue
		}

		// TODO(jasonco): make the posix attributes match the source
		// This will only work if the original upload sends the posix
		// attributes to GCS. For now, we'll just give the user full
		// access.
		mode := os.FileMode(0555)
		if err := gf.OS.Chmod(finalname, mode); err != nil {
			e := fmt.Errorf("chmod %q to %v: %v", finalname, mode, err)
			gf.recordFailure(j, started, noTimeout, e, report)
			continue
		}

		gf.recordSuccess(j, started, size, finalname, report)
		break // Success! No more retries needed.
	}

	return report
}

// fetchObjectOnceWithTimeout is merely mechanics to call fetchObjectOnce(),
// using a circuit breaker pattern to timeout the call if it takes too long.
// GCS has long tail latencies, so we retry with low timeouts on the first
// couple of attempts. On subsequent attempts, we simply wait for a long time.
func (gf *Fetcher) fetchObjectOnceWithTimeout(ctx context.Context, j job, timeout time.Duration, dest string) (sizeBytes, error) {
	result := make(chan fetchOnceResult, 1)
	breakerSig := make(chan struct{}, 1)

	// Start the function that we want to timeout if it takes too long.
	go func() {
		result <- gf.fetchObjectOnce(ctx, j, dest, breakerSig)
	}()

	// Wait to see who finshes first: function or timeout
	select {
	case r := <-result:
		return r.size, r.err
	case <-ctx.Done():
		close(breakerSig) // Signal fetchObjectOnce() to cancel
		if ctx.Err() == context.DeadlineExceeded {
			return 0, errGCSTimeout
		}
		return 0, ctx.Err()
	case <-time.After(timeout):
		close(breakerSig) // Signal fetchObjectOnce() to cancel
		return 0, errGCSTimeout
	}
}

// fetchObjectOnce has the responsibility of downloading a file from
// GCS and saving it to the dest location. If it receives a signal on
// breakerSig, it will attempt to return quickly, though it is assumed
// that no one is listening for a response anymore.
func (gf *Fetcher) fetchObjectOnce(ctx context.Context, j job, dest string, breakerSig <-chan struct{}) fetchOnceResult {
	var result fetchOnceResult

	r, err := gf.GCS.NewReader(ctx, j.bucket, j.object)
	if err != nil {
		// Check for AccessDenied failure here and return a useful error message on Stderr and exit immediately.
		if gerr, ok := err.(*googleapi.Error); ok && gerr.Code == http.StatusForbidden {
			// Try to parse out the robot name.
			match := robotRegex.FindStringSubmatch(err.Error())
			robot := "your Cloud Build service account"
			if len(match) == 2 {
				robot = match[1]
			}
			result.err = &permissionError{bucket: j.bucket, robot: robot}
			return result
		}
		result.err = fmt.Errorf("creating GCS reader for %q: %v", formatGCSName(j.bucket, j.object, j.generation), err)
		return result
	}
	defer func() {
		if cerr := r.Close(); cerr != nil {
			result.err = fmt.Errorf("Failed to close GCS reader: %v", cerr)
		}
	}()

	// If we're cancelled, just return.
	select {
	case <-breakerSig:
		result.err = errGCSTimeout
		return result
	default:
		// Fallthrough
	}

	f, err := gf.OS.Create(dest)
	if err != nil {
		result.err = fmt.Errorf("creating destination file %q: %v", dest, err)
		return result
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			result.err = fmt.Errorf("Failed to close file %q: %v", dest, cerr)
		}
	}()

	h := sha1.New()
	n, err := io.Copy(f, io.TeeReader(r, h))
	if err != nil {
		result.err = fmt.Errorf("copying bytes from %q to %q: %v", formatGCSName(j.bucket, j.object, j.generation), dest, err)
		return result
	}

	// If we're cancelled, just return.
	select {
	case <-breakerSig:
		result.err = errGCSTimeout
		return result
	default:
		result.size = sizeBytes(n)
		return result
	}

	// Verify the sha1sum before declaring success.
	if j.sha1sum != "" {
		got := fmt.Sprintf("%x", h.Sum(nil))
		want := j.sha1sum
		if got != want {
			result.err = fmt.Errorf("%s SHA mismatch, got %q, want %q", j.filename, got, want)
			return result
		}
	}
	return result
}

// ensureFolders takes a full path to a filename and makes sure that
// all the folders leading to the filename exist.
func (gf *Fetcher) ensureFolders(filename string) error {
	filedir := filepath.Dir(filename)
	gf.mu.Lock()
	defer gf.mu.Unlock()
	if _, ok := gf.CreatedDirs[filedir]; !ok {
		if err := gf.OS.MkdirAll(filedir, os.FileMode(0777)|os.ModeDir); err != nil {
			return fmt.Errorf("ensuring folders for %q: %v", filedir, err)
		}
		gf.CreatedDirs[filedir] = true
	}
	return nil
}

// doWork is the worker routine. It listens for jobs, fetches the file,
// and emits a job report. This continues until channel job is closed.
func (gf *Fetcher) doWork(ctx context.Context, todo <-chan job, results chan<- jobReport) {
	for j := range todo {
		report := gf.fetchObject(ctx, j)
		if gf.Verbose {
			gf.log("Report: %#v", report)
		}
		results <- *report
	}
}

// processJobs is the primary concurrency mechanics for Fetcher.
// This method spins up a set of worker goroutines, creates a
// goroutine to send all the jobs to the workers, then waits for
// all the jobs to complete. It also compiles and returns final
// statistics for the jobs.
func (gf *Fetcher) processJobs(ctx context.Context, jobs []job) stats {
	workerCount := gf.WorkerCount
	if len(jobs) < workerCount {
		workerCount = len(jobs)
	}
	todo := make(chan job, workerCount)
	results := make(chan jobReport, workerCount)
	stats := stats{workers: workerCount, files: len(jobs), success: true}

	// Spin up our workers.
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			gf.doWork(ctx, todo, results)
			wg.Done()
		}()
	}

	// Queue the jobs.
	started := time.Now()
	var qwg sync.WaitGroup
	qwg.Add(1)
	go func() {
		for _, j := range jobs {
			todo <- j
		}
		qwg.Done()
	}()

	// Consume the reports.
	failed := false
	for n := 0; n < len(jobs); n++ {
		report := <-results
		if !report.success {
			failed = true
		}
		stats.size += report.size
		lastIndex := len(report.attempts) - 1
		stats.retries += lastIndex // First attempt is not considered a "retry".
		finalAttempt := report.attempts[lastIndex]
		stats.duration += finalAttempt.duration
		if finalAttempt.err != nil {
			stats.errs = append(stats.errs, finalAttempt.err)
		}
		for _, attempt := range report.attempts {
			if attempt.gcsTimeout > noTimeout {
				stats.gcsTimeouts++
			}
		}
	}
	qwg.Wait()
	close(results)
	close(todo)
	wg.Wait()

	if failed {
		stats.success = false
		gf.logErr("Failed to download at least one file. Cannot continue.")
		os.Exit(1)
	}

	stats.duration = time.Since(started)
	return stats
}

// getTimeout returns the GCS timeout that should be used for a given
// filenum on a given retry number. GCS has long tails on occasion, so
// in some cases, it's faster to give up early and retry on a second
// connection.
func (gf *Fetcher) timeout(filename string, retrynum int) time.Duration {
	if gf.TimeoutGCS == false {
		return defaultTimeout
	}

	// Use short timeouts for source code, longer for non-source
	if sourceExt[filepath.Ext(filename)] {
		if timeout, ok := sourceTimeout[retrynum]; ok {
			return timeout
		}
	} else {
		if timeout, ok := notSourceTimeout[retrynum]; ok {
			return timeout
		}
	}
	return defaultTimeout
}

// fetchFromManifest is used when downloading source based on a manifest file.
// It is responsible for fetching the manifest file, decoding the JSON, and
// assembling the list of jobs to process (i.e., files to download).
func (gf *Fetcher) fetchFromManifest(ctx context.Context) (err error) {
	started := time.Now()
	gf.log("Fetching manifest %s.", formatGCSName(gf.Bucket, gf.Object, gf.Generation))

	// Download the manifest file from GCS.
	manifestDir := gf.StagingDir
	j := job{
		filename:        gf.Object,
		bucket:          gf.Bucket,
		object:          gf.Object,
		generation:      gf.Generation,
		destDirOverride: manifestDir,
	}
	// Override the retry/backoff to span an up-to-11 second eventual consistency
	// issue on new project creation. We'll only do this for the first file
	// (the manifest), and then drop back to the original retry/backoff.
	oretries, obackoff := gf.Retries, gf.Backoff
	gf.Retries, gf.Backoff = 6, 1*time.Second // Yields 1s, 2s, 4s, 8s, 16s
	report := gf.fetchObject(ctx, j)
	gf.Retries, gf.Backoff = oretries, obackoff
	if !report.success {
		if err, ok := report.err.(*permissionError); ok {
			gf.logErr(err.Error())
			os.Exit(1)
		}
		return fmt.Errorf("failed to download manifest %s: %v", formatGCSName(gf.Bucket, gf.Object, gf.Generation), report.err)
	}

	// Decode the JSON manifest
	manifestFile := filepath.Join(manifestDir, j.filename)
	r, err := gf.OS.Open(manifestFile)
	if err != nil {
		return fmt.Errorf("opening manifest file %q: %v", manifestFile, err)
	}
	defer func() {
		if cerr := r.Close(); cerr != nil {
			err = fmt.Errorf("Failed to close file %q: %v", manifestFile, cerr)
		}
	}()
	var files map[string]common.ManifestItem
	if err := json.NewDecoder(r).Decode(&files); err != nil {
		return fmt.Errorf("decoding JSON from manifest file %q: %v", manifestFile, err)
	}

	// Create the jobs
	var jobs []job
	for filename, info := range files {
		bucket, object, generation, err := common.ParseBucketObject(info.SourceURL)
		if err != nil {
			return fmt.Errorf("parsing bucket/object from %q: %v", info.SourceURL, err)
		}
		j := job{
			filename:   filename,
			bucket:     bucket,
			object:     object,
			generation: generation,
			sha1sum:    info.Sha1Sum,
		}
		jobs = append(jobs, j)
	}

	gf.log("Processing %v files.", len(jobs))
	stats := gf.processJobs(ctx, jobs)

	// Final cleanup of failed downloads. We won't miss any files; these vestiges
	// are from go routines that have timed out and would otherwise check their
	// circuit breaker and die. However, we won't wait for these remaining
	// go routines to finish because out goal is to get done as fast as possible!
	if err := gf.OS.RemoveAll(gf.StagingDir); err != nil {
		gf.log("Failed to remove staging dir %v, continuing: %v", gf.StagingDir, err)
	}

	// Emit final stats.
	mib := float64(stats.size) / 1024 / 1024
	var mibps float64
	if stats.duration > 0 {
		mibps = mib / stats.duration.Seconds()
	}
	manifestDuration := report.attempts[len(report.attempts)-1].duration
	status := "SUCCESS"
	if !stats.success {
		status = "FAILURE"
	}
	gf.log("******************************************************")
	gf.log("Status:                      %s", status)
	gf.log("Started:                     %s", started.Format(time.RFC3339))
	gf.log("Completed:                   %s", time.Now().Format(time.RFC3339))
	gf.log("Requested workers: %6d", gf.WorkerCount)
	gf.log("Actual workers:    %6d", stats.workers)
	gf.log("Total files:       %6d", stats.files)
	gf.log("Total retries:     %6d", stats.retries)
	if gf.TimeoutGCS {
		gf.log("GCS timeouts:      %6d", stats.gcsTimeouts)
	}
	gf.log("MiB downloaded:    %9.2f MiB", mib)
	gf.log("MiB/s throughput:  %9.2f MiB/s", mibps)

	gf.log("Time for manifest: %9.2f ms", float64(manifestDuration)/float64(time.Millisecond))
	gf.log("Total time:        %9.2f s", time.Since(started).Seconds())
	gf.log("******************************************************")

	if len(stats.errs) > 0 {
		var es []string
		es = append(es, fmt.Sprintf("Errors (%d):", len(stats.errs)))
		for _, e := range stats.errs {
			es = append(es, fmt.Sprintf(" - %s", e))
		}
		return fmt.Errorf(strings.Join(es, "\n"))
	}
	return nil
}

func (gf *Fetcher) copyFile(name string, mode os.FileMode, rc io.ReadCloser) (err error) {
	defer func() {
		if cerr := rc.Close(); cerr != nil {
			err = fmt.Errorf("Failed to close file %q: %v", name, cerr)
		}
	}()

	targetFile := filepath.Join(gf.DestDir, name)
	if err := gf.OS.MkdirAll(filepath.Dir(targetFile), mode); err != nil {
		return err
	}

	targetWriter, err := os.OpenFile(targetFile, os.O_WRONLY|os.O_CREATE, mode)
	if err != nil {
		return fmt.Errorf("failed to open target file %q: %v", targetFile, err)
	}
	defer func() {
		if cerr := targetWriter.Close(); cerr != nil {
			err = fmt.Errorf("Failed to close file %q: %v", targetFile, cerr)
		}
	}()

	if _, err := io.Copy(targetWriter, rc); err != nil {
		return fmt.Errorf("failed to copy %q to %q: %v", name, targetFile, err)
	}
	return nil
}

// fetchFromZip is used when downloading a single zip of source files. It is
// responsible to fetch the zip file and unzip it into the destination folder.
func (gf *Fetcher) fetchFromZip(ctx context.Context) (err error) {
	started := time.Now()
	gf.log("Fetching archive %s.", formatGCSName(gf.Bucket, gf.Object, gf.Generation))

	// Download the archive from GCS.
	zipDir := gf.StagingDir
	j := job{
		filename:        gf.Object,
		bucket:          gf.Bucket,
		object:          gf.Object,
		generation:      gf.Generation,
		destDirOverride: zipDir,
	}
	report := gf.fetchObject(ctx, j)
	if !report.success {
		return fmt.Errorf("failed to download archive %s: %v", formatGCSName(gf.Bucket, gf.Object, gf.Generation), report.err)
	}

	// Unzip into the destination directory
	unzipStart := time.Now()
	zipfile := filepath.Join(zipDir, gf.Object)
	zipReader, err := zip.OpenReader(zipfile)
	if err != nil {
		return fmt.Errorf("failed to open archive %s: %v", zipfile, err)
	}
	defer func() {
		if cerr := zipReader.Close(); cerr != nil {
			err = fmt.Errorf("Failed to close file %q: %v", zipfile, cerr)
		}
	}()

	numFiles := 0
	for _, file := range zipReader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		numFiles++
		zf, err := file.Open()
		if err != nil {
			return err
		}
		if err := gf.copyFile(file.Name, file.Mode(), zf); err != nil {
			return err
		}
	}
	unzipDuration := time.Since(unzipStart)

	// Remove the zip file (best effort only, no harm if this fails).
	if err := os.RemoveAll(zipfile); err != nil {
		gf.log("Failed to remove zipfile %s, continuing: %v", zipfile, err)
	}

	// Final cleanup of staging directory, which is only a temporary staging
	// location for downloading the zipfile in this case.
	if err := gf.OS.RemoveAll(gf.StagingDir); err != nil {
		gf.log("Failed to remove staging dir %q, continuing: %v", gf.StagingDir, err)
	}

	mib := float64(report.size) / 1024 / 1024
	var mibps float64
	zipfileDuration := report.attempts[len(report.attempts)-1].duration
	if zipfileDuration > 0 {
		mibps = mib / zipfileDuration.Seconds()
	}
	gf.log("******************************************************")
	gf.log("Status:                      SUCCESS")
	gf.log("Started:                     %s", started.Format(time.RFC3339))
	gf.log("Completed:                   %s", time.Now().Format(time.RFC3339))
	gf.log("Total files:       %6d", numFiles)
	gf.log("MiB downloaded:    %9.2f MiB", mib)
	gf.log("MiB/s throughput:  %9.2f MiB/s", mibps)
	gf.log("Time for zipfile:  %9.2f s", zipfileDuration.Seconds())
	gf.log("Time to unzip:     %9.2f s", unzipDuration.Seconds())
	gf.log("Total time:        %9.2f s", time.Since(started).Seconds())
	gf.log("******************************************************")
	return nil
}

// fetchFromTarGz is used when downloading a single .tar.gz of source files. It
// is responsible to fetch the .tar.gz file and unzip it into the destination
// folder.
func (gf *Fetcher) fetchFromTarGz(ctx context.Context) (err error) {
	started := time.Now()
	gf.log("Fetching archive %s.", formatGCSName(gf.Bucket, gf.Object, gf.Generation))

	// Download the archive from GCS.
	tgzDir := gf.StagingDir
	j := job{
		filename:        gf.Object,
		bucket:          gf.Bucket,
		object:          gf.Object,
		generation:      gf.Generation,
		destDirOverride: tgzDir,
	}
	report := gf.fetchObject(ctx, j)
	if !report.success {
		return fmt.Errorf("failed to download archive %s: %v", formatGCSName(gf.Bucket, gf.Object, gf.Generation), report.err)
	}

	// Untgz into the destination directory
	untgzStart := time.Now()
	tgzfile := filepath.Join(tgzDir, gf.Object)
	f, err := os.Open(tgzfile)
	if err != nil {
		return err
	}
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gzr)

	defer func() {
		if cerr := f.Close(); cerr != nil {
			err = fmt.Errorf("Failed to close file %q: %v", tgzfile, cerr)
		}
	}()

	numFiles := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		n := filepath.Join(gf.DestDir, h.Name)
		switch h.Typeflag {
		case tar.TypeDir:
			if err := gf.OS.MkdirAll(n, h.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := func() error {
				f, err := os.OpenFile(n, os.O_WRONLY|os.O_CREATE, h.FileInfo().Mode())
				if err != nil {
					return err
				}
				defer f.Close()
				_, err = io.Copy(f, tr)
				return err
			}(); err != nil {
				return err
			}
		}
	}
	untgzDuration := time.Since(untgzStart)

	// Remove the tgz file (best effort only, no harm if this fails).
	if err := gf.OS.RemoveAll(tgzfile); err != nil {
		gf.log("Failed to remove tgzfile %s, continuing: %v", tgzfile, err)
	}

	// Final cleanup of staging directory, which is only a temporary staging
	// location for downloading the tgzfile in this case.
	if err := gf.OS.RemoveAll(gf.StagingDir); err != nil {
		gf.log("Failed to remove staging dir %q, continuing: %v", gf.StagingDir, err)
	}

	mib := float64(report.size) / 1024 / 1024
	var mibps float64
	tgzfileDuration := report.attempts[len(report.attempts)-1].duration
	if tgzfileDuration > 0 {
		mibps = mib / tgzfileDuration.Seconds()
	}
	gf.log("******************************************************")
	gf.log("Status:                      SUCCESS")
	gf.log("Started:                     %s", started.Format(time.RFC3339))
	gf.log("Completed:                   %s", time.Now().Format(time.RFC3339))
	gf.log("Total files:       %6d", numFiles)
	gf.log("MiB downloaded:    %9.2f MiB", mib)
	gf.log("MiB/s throughput:  %9.2f MiB/s", mibps)
	gf.log("Time for tgzfile:  %9.2f s", tgzfileDuration.Seconds())
	gf.log("Time to untgz:     %9.2f s", untgzDuration.Seconds())
	gf.log("Total time:        %9.2f s", time.Since(started).Seconds())
	gf.log("******************************************************")
	return nil
}

// Fetch is the main entry point into Fetcher. Based on configuration,
// it pulls source from GCS into the destination directory.
func (gf *Fetcher) Fetch(ctx context.Context) error {
	switch gf.SourceType {
	case "Manifest":
		return gf.fetchFromManifest(ctx)
	case "Archive":
		fmt.Println("WARNING: -type=Archive is deprecated; use -type=ZipArchive")
		fallthrough
	case "ZipArchive":
		return gf.fetchFromZip(ctx)
	case "TarGzArchive":
		return gf.fetchFromTarGz(ctx)
	default:
		return fmt.Errorf("misconfigured GCSFetcher, unsupported -type %q", gf.SourceType)
	}
	return nil
}

func formatGCSName(bucket, object string, generation int64) string {
	n := fmt.Sprintf("gs://%s/%s", bucket, object)
	if generation > 0 {
		n += fmt.Sprintf("#%d", generation)
	}
	return n
}
