/*
Copyright 2018 The Knative Authors

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
	"fmt"

	"go.uber.org/zap"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	v1alpha1 "github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/logging"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const (
	threadsPerController = 2
)

var (
	clusterConfig = flag.String("clusterConfig", "", "Configuration of a cluster. Only required if out-of-cluster.")
)

func main() {
	flag.Parse()

	logger, _ := logging.NewLogger("", "kubeconfig")
	defer logger.Sync()

	cr := v1alpha1.ClusterResource{}
	err := json.Unmarshal([]byte(*clusterConfig), &cr)
	if err != nil {
		logger.Fatalf("Error reading cluster config: %v", err)
	}
	createKubeconfigFile(&cr, logger)
}

func createKubeconfigFile(resource *v1alpha1.ClusterResource, logger *zap.SugaredLogger) {
	cluster := &clientcmdapi.Cluster{
		Server:                   resource.URL,
		InsecureSkipTLSVerify:    resource.Insecure,
		CertificateAuthorityData: resource.CAData,
	}
	//only one authentication method is allowed for user
	user := resource.Username
	pass := resource.Password
	if resource.Token != "" {
		user = ""
		pass = ""
	}
	auth := &clientcmdapi.AuthInfo{
		Token:    resource.Token,
		Username: user,
		Password: pass,
	}
	context := &clientcmdapi.Context{
		Cluster:  resource.ClusterName,
		AuthInfo: resource.Username,
	}
	c := clientcmdapi.NewConfig()
	c.Clusters[resource.ClusterName] = cluster
	c.AuthInfos[resource.Username] = auth
	c.Contexts[resource.ClusterName] = context
	c.CurrentContext = resource.ClusterName
	c.APIVersion = "v1"
	c.Kind = "Config"

	destinationFile := fmt.Sprintf("/workspace/%s/kubeconfig", resource.ClusterName)
	if err := clientcmd.WriteToFile(*c, destinationFile); err != nil {
		logger.Fatalf("Error writing kubeconfig to file: %v", err)
	}
}
