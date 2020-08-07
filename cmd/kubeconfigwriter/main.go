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
	"flag"
	"os"
	"path/filepath"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/cluster"
	"go.uber.org/zap"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"knative.dev/pkg/logging"
)

var (
	clusterConfig  = flag.String("clusterConfig", "", "json string with the configuration of a cluster based on values from a cluster resource. Only required for external clusters.")
	destinationDir = flag.String("destinationDir", "", "destination directory where generated kubeconfig file will be stored.")
)

func main() {
	flag.Parse()

	logger, _ := logging.NewLogger("", "kubeconfig")
	defer func() {
		_ = logger.Sync()
	}()

	cr := cluster.Resource{}
	err := json.Unmarshal([]byte(*clusterConfig), &cr)
	if err != nil {
		logger.Fatalf("Error reading cluster config: %v", err)
	}
	createKubeconfigFile(&cr, logger, destinationDir)
}

func createKubeconfigFile(resource *cluster.Resource, logger *zap.SugaredLogger, destinationDir *string) {
	cluster := &clientcmdapi.Cluster{
		Server:                   resource.URL,
		InsecureSkipTLSVerify:    resource.Insecure,
		CertificateAuthorityData: resource.CAData,
	}
	if caFromEnv := os.Getenv("CADATA"); caFromEnv != "" {
		cluster.CertificateAuthorityData = []byte(caFromEnv)
	}
	if tokenFromEnv := os.Getenv("TOKEN"); tokenFromEnv != "" {
		resource.Token = strings.TrimRight(tokenFromEnv, "\r\n")
	}
	if usernameFromEnv := os.Getenv("USERNAME"); usernameFromEnv != "" {
		resource.Username = usernameFromEnv
	}
	if passwordFromEnv := os.Getenv("PASSWORD"); passwordFromEnv != "" {
		resource.Password = passwordFromEnv
	}
	//only one authentication technique per user is allowed in a kubeconfig, so clear out the password if a token is provided
	user := resource.Username
	pass := resource.Password
	clientKeyData := resource.ClientKeyData
	clientCertificateData := resource.ClientCertificateData
	if resource.Token != "" {
		user = ""
		pass = ""
	}
	auth := &clientcmdapi.AuthInfo{
		Token:                 resource.Token,
		Username:              user,
		Password:              pass,
		ClientKeyData:         clientKeyData,
		ClientCertificateData: clientCertificateData,
	}
	context := &clientcmdapi.Context{
		Cluster:  resource.Name,
		AuthInfo: resource.Username,
		// Namespace isn't written to kubeconfig if this is empty
		Namespace: resource.Namespace,
	}
	c := clientcmdapi.NewConfig()
	c.Clusters[resource.Name] = cluster
	c.AuthInfos[resource.Username] = auth
	c.Contexts[resource.Name] = context
	c.CurrentContext = resource.Name
	c.APIVersion = "v1"
	c.Kind = "Config"

	// kubeconfig file location
	var destinationFile string

	// If the destination Directory is provided, kubeconfig will be written to the given directory.
	// otherwise it will use default location i.e. "/workspace/<cluster-name>/
	if *destinationDir != "" {
		destinationFile = filepath.Join(*destinationDir, "kubeconfig")
	} else {
		destinationFile = filepath.Join("/workspace", resource.Name, "kubeconfig")
	}

	if err := clientcmd.WriteToFile(*c, destinationFile); err != nil {
		logger.Fatalf("Error writing kubeconfig to file: %v", err)
	}
	logger.Infof("kubeconfig file successfully written to %s", destinationFile)
}
