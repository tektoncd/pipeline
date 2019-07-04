package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/pflag"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/cmd"
)

const descriptionSourcePath = "docs/reference/cmd/"

func generateCliYaml(opts *options) error {
	tp := &cli.TektonParams{}
	tkn := cmd.Root(tp)
	disableFlagsInUseLine(tkn)
	source := filepath.Join(opts.source, descriptionSourcePath)
	if err := loadLongDescription(tkn, source); err != nil {
		return err
	}

	tkn.DisableAutoGenTag = true
	err := doc.GenMarkdownTree(tkn, opts.target)
	if err != nil {
		return nil
	}
	return nil
}

func disableFlagsInUseLine(cmd *cobra.Command) {
	visitAll(cmd, func(ccmd *cobra.Command) {
		// do not add a `[flags]` to the end of the usage line.
		ccmd.DisableFlagsInUseLine = true
	})
}

// visitAll will traverse all commands from the root.
// This is different from the VisitAll of cobra.Command where only parents
// are checked.
func visitAll(root *cobra.Command, fn func(*cobra.Command)) {
	for _, cmd := range root.Commands() {
		visitAll(cmd, fn)
	}
	fn(root)
}

func loadLongDescription(cmd *cobra.Command, path ...string) error {
	for _, cmd := range cmd.Commands() {
		if cmd.Name() == "" {
			continue
		}
		fullpath := filepath.Join(path[0], strings.Join(append(path[1:], cmd.Name()), "_")+".md")
		if cmd.HasSubCommands() {
			loadLongDescription(cmd, path[0], cmd.Name())
		}

		if _, err := os.Stat(fullpath); err != nil {
			log.Printf("WARN: %s does not exist, skipping\n", fullpath)
			continue
		}

		content, err := ioutil.ReadFile(fullpath)
		if err != nil {
			return err
		}
		description, examples := parseMDContent(string(content))
		cmd.Long = description
		cmd.Example = examples
	}
	return nil
}

type options struct {
	source string
	target string
}

func parseArgs() (*options, error) {
	opts := &options{}
	cwd, _ := os.Getwd()
	flags := pflag.NewFlagSet(os.Args[0], pflag.ContinueOnError)
	flags.StringVar(&opts.source, "root", cwd, "Path to project root")
	flags.StringVar(&opts.target, "target", "/tmp", "Target path for generated yaml files")
	err := flags.Parse(os.Args[1:])
	return opts, err
}

func parseMDContent(mdString string) (description string, examples string) {
	parsedContent := strings.Split(mdString, "\n## ")
	for _, s := range parsedContent {
		if strings.Index(s, "Description") == 0 {
			description = strings.TrimSpace(strings.TrimPrefix(s, "Description"))
		}
		if strings.Index(s, "Examples") == 0 {
			examples = strings.TrimSpace(strings.TrimPrefix(s, "Examples"))
		}
	}
	fmt.Println(description, examples)
	return description, examples
}

func main() {
	opts, err := parseArgs()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	fmt.Printf("Project root: %s\n", opts.source)
	fmt.Printf("Generating yaml files into %s\n", opts.target)
	if err := generateCliYaml(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate yaml files: %s\n", err.Error())
	}
}
