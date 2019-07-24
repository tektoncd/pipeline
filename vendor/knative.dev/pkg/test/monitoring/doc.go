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

/*
Package monitoring provides common methods for all the monitoring components used in the tests

This package exposes following methods:

	CheckPortAvailability(port int) error
		Checks if the given port is available
	GetPods(kubeClientset *kubernetes.Clientset, app string) (*v1.PodList, error)
		Gets the list of pods that satisfy the lable selector app=<app>
	Cleanup(pid int) error
		Kill the current port forwarding process running in the background
	PortForward(logf logging.FormatLogger, podList *v1.PodList, localPort, remotePort int) (int, error)
		Create a background process that will port forward the first pod from the local to remote port
		It returns the process id for the background process created.
*/
package monitoring
