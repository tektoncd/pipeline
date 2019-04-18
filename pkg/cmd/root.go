// Copyright © 2019 The Knative Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeCfgFile string
	kubeConfig  *rest.Config
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tkn",
	Short: "CLI for tekton pipelines",
	Long: `

	`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initKubeCfgFile)
	cobra.OnInitialize(initKubeConfig)

	rootCmd.PersistentFlags().StringVarP(
		&kubeCfgFile, "kubeconfig", "k",
		"", "kubectl config file (default: $HOME/.kube/config)")
}

func initKubeCfgFile() {
	if kubeCfgFile != "" {
		return
	}

	if kubeEnvConf, ok := os.LookupEnv("KUBECONFIG"); ok {
		kubeCfgFile = kubeEnvConf
		return
	}

	home, err := homedir.Dir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	kubeCfgFile = filepath.Join(home, ".kube", "config")

	if kubeConfig, err = clientcmd.BuildConfigFromFlags("", kubeCfgFile); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initKubeConfig() {
	fmt.Fprintln(os.Stderr, "Using kubeconfig file:", kubeCfgFile)
	c, err := clientcmd.BuildConfigFromFlags("", kubeCfgFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	kubeConfig = c
}
