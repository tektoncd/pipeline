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

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/tektoncd/pipeline/cmd/entrypoint/subcommands"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1/types"
	"github.com/tektoncd/pipeline/pkg/credentials/dockercreds"
	"github.com/tektoncd/pipeline/pkg/credentials/gitcreds"
	credwriter "github.com/tektoncd/pipeline/pkg/credentials/writer"
	"github.com/tektoncd/pipeline/pkg/entrypoint"
	"github.com/tektoncd/pipeline/pkg/platforms"
	"github.com/tektoncd/pipeline/pkg/termination"
)

var (
	ep                  = flag.String("entrypoint", "", "Original specified entrypoint to execute")
	waitFiles           = flag.String("wait_file", "", "Comma-separated list of paths to wait for")
	waitFileContent     = flag.Bool("wait_file_content", false, "If specified, expect wait_file to have content")
	postFile            = flag.String("post_file", "", "If specified, file to write upon completion")
	terminationPath     = flag.String("termination_path", "/tekton/termination", "If specified, file to write upon termination")
	results             = flag.String("results", "", "If specified, list of file names that might contain task results")
	stepResults         = flag.String("step_results", "", "step results if specified")
	whenExpressions     = flag.String("when_expressions", "", "when expressions if specified")
	timeout             = flag.Duration("timeout", time.Duration(0), "If specified, sets timeout for step")
	stdoutPath          = flag.String("stdout_path", "", "If specified, file to copy stdout to")
	stderrPath          = flag.String("stderr_path", "", "If specified, file to copy stderr to")
	breakpointOnFailure = flag.Bool("breakpoint_on_failure", false, "If specified, expect steps to not skip on failure")
	debugBeforeStep     = flag.Bool("debug_before_step", false, "If specified, wait for a debugger to attach before executing the step")
	onError             = flag.String("on_error", "", "Set to \"continue\" to ignore an error and continue when a container terminates with a non-zero exit code."+
		" Set to \"stopAndFail\" to declare a failure with a step error and stop executing the rest of the steps.")
	stepMetadataDir        = flag.String("step_metadata_dir", "", "If specified, create directory to store the step metadata e.g. /tekton/steps/<step-name>/")
	resultExtractionMethod = flag.String("result_from", entrypoint.ResultExtractionMethodTerminationMessage, "The method using which to extract results from tasks. Default is using the termination message.")
	secretMaskFile         = flag.String("secret_mask_file", "", "If specified, file containing base64-encoded secrets to mask in stdout/stderr (one per line)")
)

const (
	defaultWaitPollingInterval = time.Second
	TektonPlatformCommandsEnv  = "TEKTON_PLATFORM_COMMANDS"
)

func main() {
	// Add credential flags originally introduced with our legacy credentials helper
	// image (creds-init).
	gitcreds.AddFlags(flag.CommandLine)
	dockercreds.AddFlags(flag.CommandLine)

	// Split args with `--` for the entrypoint and what it should execute
	args, commandArgs := extractArgs(os.Args[1:])

	// We are using the global variable flag.CommandLine here to be able
	// to define what args it should parse.
	// flag.Parse() does flag.CommandLine.Parse(os.Args[1:])
	if err := flag.CommandLine.Parse(args); err != nil {
		os.Exit(1)
	}
	if err := subcommands.Process(flag.CommandLine.Args()); err != nil {
		log.Println(err.Error())
		var ok subcommands.OK
		if errors.As(err, &ok) {
			return
		}
		os.Exit(1)
	}

	// Copy credentials we're expecting from the legacy credentials helper (creds-init)
	// from secret volume mounts to /tekton/creds. This is done to support the expansion
	// of a variable, $(credentials.path), that resolves to a single place with all the
	// stored credentials.
	builders := []credwriter.Writer{dockercreds.NewBuilder(), gitcreds.NewBuilder()}
	for _, c := range builders {
		if err := c.Write(entrypoint.CredsDir); err != nil {
			log.Printf("Error initializing credentials: %s", err)
		}
	}

	var cmd []string
	if *ep != "" {
		cmd = []string{*ep}
	} else {
		env := os.Getenv(TektonPlatformCommandsEnv)
		var cmds map[string][]string
		if err := json.Unmarshal([]byte(env), &cmds); err != nil {
			log.Fatal(err)
		}
		// NB: This value contains OS/architecture and maybe variant.
		// It doesn't include osversion, which is necessary to
		// disambiguate two images both for e.g., Windows, that only
		// differ by osversion.
		plat := platforms.NewPlatform().Format()
		var err error
		cmd, err = selectCommandForPlatform(cmds, plat)
		if err != nil {
			log.Fatal(err)
		}
	}
	var when v1.StepWhenExpressions
	if len(*whenExpressions) > 0 {
		if err := json.Unmarshal([]byte(*whenExpressions), &when); err != nil {
			log.Fatal(err)
		}
	}

	spireWorkloadAPI := initializeSpireAPI()

	e := entrypoint.Entrypointer{
		Command:         append(cmd, commandArgs...),
		WaitFiles:       strings.Split(*waitFiles, ","),
		WaitFileContent: *waitFileContent,
		PostFile:        *postFile,
		TerminationPath: *terminationPath,
		Waiter:          &realWaiter{waitPollingInterval: defaultWaitPollingInterval, breakpointOnFailure: *breakpointOnFailure},
		Runner: &realRunner{
			stdoutPath:     *stdoutPath,
			stderrPath:     *stderrPath,
			secretMaskFile: *secretMaskFile,
		},
		PostWriter:             &realPostWriter{},
		Results:                strings.Split(*results, ","),
		StepResults:            strings.Split(*stepResults, ","),
		Timeout:                timeout,
		StepWhenExpressions:    when,
		BreakpointOnFailure:    *breakpointOnFailure,
		DebugBeforeStep:        *debugBeforeStep,
		OnError:                *onError,
		StepMetadataDir:        *stepMetadataDir,
		SpireWorkloadAPI:       spireWorkloadAPI,
		ResultExtractionMethod: *resultExtractionMethod,
	}

	// Copy any creds injected by the controller into the $HOME directory of the current
	// user so that they're discoverable by git / ssh.
	if err := credwriter.CopyCredsToHome(credwriter.CredsInitCredentials); err != nil {
		log.Printf("non-fatal error copying credentials: %q", err)
	}

	if err := e.Go(); err != nil {
		switch t := err.(type) { //nolint:errorlint // checking for multiple types with errors.As is ugly.
		case entrypoint.DebugBeforeStepError:
			log.Println("Skipping execute step script because before step breakpoint fail-continue")
			os.Exit(1)
		case entrypoint.SkipError:
			log.Print("Skipping step because a previous step failed")
			os.Exit(1)
		case termination.MessageLengthError:
			log.Print(err.Error())
			os.Exit(1)
		case entrypoint.ContextError:
			if entrypoint.IsContextCanceledError(err) {
				log.Print("Step was cancelled")
				// use the SIGKILL signal to distinguish normal exit programs, just like kill -9 PID
				os.Exit(int(syscall.SIGKILL))
			} else {
				log.Print(err.Error())
				os.Exit(1)
			}
		case *exec.ExitError:
			// Copied from https://stackoverflow.com/questions/10385551/get-exit-code-go
			// This works on both Unix and Windows. Although
			// package syscall is generally platform dependent,
			// WaitStatus is defined for both Unix and Windows and
			// in both cases has an ExitStatus() method with the
			// same signature.
			if status, ok := t.Sys().(syscall.WaitStatus); ok {
				e.CheckForBreakpointOnFailure()
				// ignore a step error i.e. do not exit if a container terminates with a non-zero exit code when onError is set to "continue"
				if e.OnError != entrypoint.ContinueOnError {
					os.Exit(status.ExitStatus())
				}
			}
			// log and exit only if a step error must cause run failure
			if e.OnError != entrypoint.ContinueOnError {
				log.Fatalf("Error executing command (ExitError): %v", err)
			}
		default:
			e.CheckForBreakpointOnFailure()
			log.Fatalf("Error executing command: %v", err)
		}
	}
}

func selectCommandForPlatform(cmds map[string][]string, plat string) ([]string, error) {
	cmd, found := cmds[plat]
	if found {
		return cmd, nil
	}

	// If the command wasn't found, check if there's a
	// command defined for the same platform without a CPU
	// variant specified.
	platWithoutVariant := plat[:strings.LastIndex(plat, "/")]
	cmd, found = cmds[platWithoutVariant]
	if found {
		return cmd, nil
	}
	return nil, fmt.Errorf("could not find command for platform %q", plat)
}
