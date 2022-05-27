/*
Copyright 2019 The Tekton Authors

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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/termination"
	"go.uber.org/zap"

	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	"github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3"
)

// RFC3339 with millisecond
const (
	timeFormat      = "2006-01-02T15:04:05.000Z07:00"
	ContinueOnError = "continue"
	FailOnError     = "stopAndFail"
)

// Constants for IBM COS values
const (
	apiKey            = "<>"
	serviceInstanceID = "crn:v1:bluemix:public:cloud-object-storage:global:a/9b13b857a32341b7167255de717172f5:8fbc0235-1bed-48b8-a3f2-81f2b8d1a7b4::"
	authEndpoint      = "https://iam.cloud.ibm.com/identity/token"
	serviceEndpoint   = "https://s3.us-south.cloud-object-storage.appdomain.cloud"
)

// Create config

var conf = aws.NewConfig().
	WithRegion("us").
	WithEndpoint(serviceEndpoint).
	WithCredentials(ibmiam.NewStaticCredentials(aws.NewConfig(), authEndpoint, apiKey, serviceInstanceID)).
	WithS3ForcePathStyle(true)

// Create client
var sess = session.Must(session.NewSession())
var client = s3.New(sess, conf)

// Entrypointer holds fields for running commands with redirected
// entrypoints.
type Entrypointer struct {
	// Command is the original specified command and args.
	Command []string

	// WaitFiles is the set of files to wait for. If empty, execution
	// begins immediately.
	WaitFiles []string
	// WaitFileContent indicates the WaitFile should have non-zero size
	// before continuing with execution.
	WaitFileContent bool
	// PostFile is the file to write when complete. If not specified, no
	// file is written.
	PostFile string

	// Termination path is the path of a file to write the starting time of this endpopint
	TerminationPath string

	TaskRunName string
	// Waiter encapsulates waiting for files to exist.
	Waiter Waiter
	// Runner encapsulates running commands.
	Runner Runner
	// PostWriter encapsulates writing files when complete.
	PostWriter PostWriter

	// Results is the set of files that might contain task results
	Results []string
	// Timeout is an optional user-specified duration within which the Step must complete
	Timeout *time.Duration
	// BreakpointOnFailure helps determine if entrypoint execution needs to adapt debugging requirements
	BreakpointOnFailure bool
	// OnError defines exiting behavior of the entrypoint
	// set it to "stopAndFail" to indicate the entrypoint to exit the taskRun if the container exits with non zero exit code
	// set it to "continue" to indicate the entrypoint to continue executing the rest of the steps irrespective of the container exit code
	OnError string
	// StepMetadataDir is the directory for a step where the step related metadata can be stored
	StepMetadataDir string
}

// Waiter encapsulates waiting for files to exist.
type Waiter interface {
	// Wait blocks until the specified file exists.
	Wait(file string, expectContent bool, breakpointOnFailure bool) error
}

// Runner encapsulates running commands.
type Runner interface {
	Run(ctx context.Context, args ...string) error
}

// PostWriter encapsulates writing a file when complete.
type PostWriter interface {
	// Write writes to the path when complete.
	Write(file, content string)
}

func (e Entrypointer) storeLargeFilesOnCOS(resultFilePath string) (string, error) {

	// Variables and random content to sample, replace when appropriate
	bucketName := "pipelineloop-test"
	key := e.TaskRunName + "/" + resultFilePath
	fileContents, _ := ioutil.ReadFile(resultFilePath)
	content := bytes.NewReader([]byte(fileContents))

	input := s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   content,
	}

	// Call Function to upload (Put) an object
	_, _ = client.PutObject(&input)
	fmt.Println("Stored result on COS: ", bucketName, key)
	return fmt.Sprintf("cos://%s/%s", bucketName, key), nil
}

// Go optionally waits for a file, runs the command, and writes a
// post file.
func (e Entrypointer) Go() error {
	prod, _ := zap.NewProduction()
	logger := prod.Sugar()
	output := []v1beta1.PipelineResourceResult{}
	defer func() {
		logger.Infof("Trying to write the output:%s to termination path: ", output)
		if wErr := termination.WriteMessage(e.TerminationPath, output); wErr != nil {
			logger.Fatalf("Error while writing message: %s", wErr)
		}
		_ = logger.Sync()
	}()

	for _, f := range e.WaitFiles {
		if err := e.Waiter.Wait(f, e.WaitFileContent, e.BreakpointOnFailure); err != nil {
			// An error happened while waiting, so we bail
			// *but* we write postfile to make next steps bail too.
			// In case of breakpoint on failure do not write post file.
			if !e.BreakpointOnFailure {
				e.WritePostFile(e.PostFile, err)
			}
			output = append(output, v1beta1.PipelineResourceResult{
				Key:        "StartedAt",
				Value:      time.Now().Format(timeFormat),
				ResultType: v1beta1.InternalTektonResultType,
			})
			return err
		}
	}

	output = append(output, v1beta1.PipelineResourceResult{
		Key:        "StartedAt",
		Value:      time.Now().Format(timeFormat),
		ResultType: v1beta1.InternalTektonResultType,
	})

	var err error
	if e.Timeout != nil && *e.Timeout < time.Duration(0) {
		err = fmt.Errorf("negative timeout specified")
	}

	if err == nil {
		ctx := context.Background()
		var cancel context.CancelFunc
		if e.Timeout != nil && *e.Timeout != time.Duration(0) {
			ctx, cancel = context.WithTimeout(ctx, *e.Timeout)
			defer cancel()
		}
		replacedStrings := make([]string, len(e.Command))
		for i, s := range e.Command {
			splitCommand := strings.Split(s, " ")
			replacedStrings1 := make([]string, len(splitCommand))
			for j, s1 := range splitCommand {
				replacedStrings1[j], err = e.prefetchFilesWithLink(logger, s1)
				logger.Infof("error while prefetching..", err)
			}
			replacedStrings[i] = strings.Join(replacedStrings1, " ")
		}
		logger.Infof("Running command: %s", strings.Join(replacedStrings, " "))
		err = e.Runner.Run(ctx, replacedStrings...)
		if err == context.DeadlineExceeded {
			output = append(output, v1beta1.PipelineResourceResult{
				Key:        "Reason",
				Value:      "TimeoutExceeded",
				ResultType: v1beta1.InternalTektonResultType,
			})
		}
	}

	var ee *exec.ExitError
	switch {
	case err != nil && e.BreakpointOnFailure:
		logger.Info("Skipping writing to PostFile")
	case e.OnError == ContinueOnError && errors.As(err, &ee):
		// with continue on error and an ExitError, write non-zero exit code and a post file
		exitCode := strconv.Itoa(ee.ExitCode())
		output = append(output, v1beta1.PipelineResourceResult{
			Key:        "ExitCode",
			Value:      exitCode,
			ResultType: v1beta1.InternalTektonResultType,
		})
		e.WritePostFile(e.PostFile, nil)
		e.WriteExitCodeFile(e.StepMetadataDir, exitCode)
	case err == nil:
		// if err is nil, write zero exit code and a post file
		e.WritePostFile(e.PostFile, nil)
		e.WriteExitCodeFile(e.StepMetadataDir, "0")
	default:
		// for a step without continue on error and any error, write a post file with .err
		e.WritePostFile(e.PostFile, err)
	}

	// strings.Split(..) with an empty string returns an array that contains one element, an empty string.
	// This creates an error when trying to open the result folder as a file.
	if len(e.Results) >= 1 && e.Results[0] != "" {
		logger.Infof("Results produced... %s", strings.Join(e.Results, "-"))
		if err := e.readResultsFromDisk(pipeline.DefaultResultPath); err != nil {
			logger.Fatalf("Error while handling results: %s", err)
		}
	}

	return err
}
func (e Entrypointer) prefetchFilesWithLink(logger *zap.SugaredLogger, str1 string) (string, error) {
	// Create client
	// Variables
	r := regexp.MustCompile(`cos://(?P<bucketName>[a-zA-Z0-9\-]+)/(?P<str>[a-zA-Z0-9\-]+)/(?P<key>.+)`)
	if r.MatchString(str1) {
		parsedString := r.FindStringSubmatch(str1)
		bucketName := parsedString[1]
		key := parsedString[2] + parsedString[3]
		fileName := parsedString[3]
		logger.Infof("Key: %s \n FileName: %s \nBucketName: %s", key, fileName, bucketName)
		_, err := os.Open(fileName)
		if err == nil {
			return key, nil // i.e. it was already fetched.
		} else {
			logger.Infof("Key: %s not found... fetching it. %v", key, err)
		}
		// users will need to create bucket, key (flat string name)
		Input := s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(key),
		}
		// Call Function
		res, err := client.GetObject(&Input)
		if err != nil {
			return "", err
		}
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return "", err
		}
		err = ioutil.WriteFile(fileName, body, fs.ModePerm)
		if err != nil {
			return "", err
		}
		return fileName, nil
	} else {
		logger.Infof("Not matched : %s", str1)
		return str1, nil
	}
}

func (e Entrypointer) readResultsFromDisk(resultDir string) error {
	output := []v1beta1.PipelineResourceResult{}
	for _, resultFile := range e.Results {
		if resultFile == "" {
			continue
		}
		resultFilePath := filepath.Join(resultDir, resultFile)
		file, err := os.Open(resultFilePath)
		if err != nil {
			log.Fatal(err)
		}
		fi, err := file.Stat()
		if err != nil {
			log.Fatal(err)
		}
		if fi.Size() > 2048 {
			// store on COS
			res, err2 := e.storeLargeFilesOnCOS(resultFilePath)
			if err2 != nil {
				log.Fatal(err2)
			} else {
				output = append(output, v1beta1.PipelineResourceResult{
					Key:        resultFile,
					Value:      res,
					ResultType: v1beta1.TaskRunResultType,
				})
			}
			continue
		}
		fileContents, err := ioutil.ReadFile(resultFilePath)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		// if the file doesn't exist, ignore it
		output = append(output, v1beta1.PipelineResourceResult{
			Key:        resultFile,
			Value:      string(fileContents),
			ResultType: v1beta1.TaskRunResultType,
		})
	}
	// push output to termination path
	if len(output) != 0 {
		if err := termination.WriteMessage(e.TerminationPath, output); err != nil {
			return err
		}
	}
	return nil
}

// BreakpointExitCode reads the post file and returns the exit code it contains
func (e Entrypointer) BreakpointExitCode(breakpointExitPostFile string) (int, error) {
	exitCode, err := ioutil.ReadFile(breakpointExitPostFile)
	if os.IsNotExist(err) {
		return 0, fmt.Errorf("breakpoint postfile %s not found", breakpointExitPostFile)
	}
	strExitCode := strings.TrimSuffix(string(exitCode), "\n")
	log.Println("Breakpoint exiting with exit code " + strExitCode)

	return strconv.Atoi(strExitCode)
}

// WritePostFile write the postfile
func (e Entrypointer) WritePostFile(postFile string, err error) {
	if err != nil && postFile != "" {
		postFile = fmt.Sprintf("%s.err", postFile)
	}
	if postFile != "" {
		e.PostWriter.Write(postFile, "")
	}
}

// WriteExitCodeFile write the exitCodeFile
func (e Entrypointer) WriteExitCodeFile(stepPath, content string) {
	exitCodeFile := filepath.Join(stepPath, "exitCode")
	e.PostWriter.Write(exitCodeFile, content)
}
