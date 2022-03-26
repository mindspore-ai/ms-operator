/*
Copyright 2022 Huawei Technologies Co., Ltd.

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

package v1

import (
	"strings"

	"k8s.io/apimachinery/pkg/runtime"

	commonv1 "github.com/kubeflow/common/pkg/apis/common/v1"
	v1 "k8s.io/api/core/v1"
)

// Int32 is a helper routine that allocates a new int32 value
// to store v and returns a pointer to it.
func Int32(v int32) *int32 {
	return &v
}

// addDefaultingFuncs is used to register default funcs
func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// setDefaultPort sets the default ports for mindspore container.
func setDefaultPort(spec *v1.PodSpec) {
	index := 0
	for i, container := range spec.Containers {
		if container.Name == DefaultContainerName {
			index = i
			break

		}
	}
	hasMSJobPort := false
	for _, port := range spec.Containers[index].Ports {
		if port.Name == DefaultPortName {
			hasMSJobPort = true
			break
		}
	}
	if !hasMSJobPort {
		spec.Containers[index].Ports = append(spec.Containers[index].Ports, v1.ContainerPort{
			Name:          DefaultPortName,
			ContainerPort: DefaultPort,
		})
	}
}

func setDefaultReplicas(spec *commonv1.ReplicaSpec) {
	if spec.Replicas == nil {
		spec.Replicas = Int32(1)
	}
	if spec.RestartPolicy == "" {
		spec.RestartPolicy = DefaultRestartPolicy
	}
}

// setTypeNamesToCamelCase sets the name of all replica types from any case to correct case.
func setTypeNamesToCamelCase(msJob *MSJob) {
	setTypeNameToCamelCase(msJob, MSReplicaTypePS)
	setTypeNameToCamelCase(msJob, MSReplicaTypeWorker)
	setTypeNameToCamelCase(msJob, MSReplicaTypeScheduler)
}

// setTypeNameToCamelCase sets the name of the replica type from any case to correct case.
// E.g. from ps to PS; from WORKER to Worker.
func setTypeNameToCamelCase(msJob *MSJob, typ commonv1.ReplicaType) {
	for t := range msJob.Spec.MSReplicaSpecs {
		if strings.EqualFold(string(t), string(typ)) && t != typ {
			spec := msJob.Spec.MSReplicaSpecs[t]
			delete(msJob.Spec.MSReplicaSpecs, t)
			msJob.Spec.MSReplicaSpecs[typ] = spec
			return
		}
	}
}

// SetDefaults_MSJob sets any unspecified values to defaults.
func SetDefaults_MSJob(msjob *MSJob) {
	// Set default cleanpod policy to Running.
	if msjob.Spec.RunPolicy.CleanPodPolicy == nil {
		running := commonv1.CleanPodPolicyRunning
		msjob.Spec.RunPolicy.CleanPodPolicy = &running
	}
	// Set default success policy to "".
	if msjob.Spec.SuccessPolicy == nil {
		defaultPolicy := SuccessPolicyDefault
		msjob.Spec.SuccessPolicy = &defaultPolicy
	}

	// Update the key of MSReplicaSpecs to camel case.
	setTypeNamesToCamelCase(msjob)

	for _, spec := range msjob.Spec.MSReplicaSpecs {
		// Set default replicas to 1.
		setDefaultReplicas(spec)
		// Set default port to mindspore container.
		setDefaultPort(&spec.Template.Spec)
	}
}
