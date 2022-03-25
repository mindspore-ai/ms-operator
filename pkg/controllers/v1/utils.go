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

package controllers

import (
	"fmt"
	mindsporev1 "my-operator/pkg/apis/v1"
	"strconv"

	commonv1 "github.com/kubeflow/common/pkg/apis/common/v1"
	"github.com/prometheus/common/log"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
)

// originally from pkg/controller.v1/tensorflow/pod.go (deleted)
func getContainerExitCode(pod *corev1.Pod) int32 {
	var exitCode int32 = 0xbeef // magic number
	for _, status := range pod.Status.ContainerStatuses {
		state := status.State
		if status.Name == mindsporev1.DefaultContainerName && state.Terminated != nil {
			exitCode = state.Terminated.ExitCode
		}
	}
	return exitCode
}

// originally from pkg/controller.v1/tensorflow/pod.go (deleted)
func setRestartPolicy(podTemplateSpec *corev1.PodTemplateSpec, spec *commonv1.ReplicaSpec) {
	// This is necessary since restartPolicyExitCode is not supported in v1.PodTemplateSpec
	if spec.RestartPolicy == commonv1.RestartPolicyExitCode {
		podTemplateSpec.Spec.RestartPolicy = corev1.RestartPolicyNever
	} else {
		podTemplateSpec.Spec.RestartPolicy = corev1.RestartPolicy(spec.RestartPolicy)
	}
}

// isDistributed returns if the MSJob is a distributed training job.
// Ref https://github.com/kubeflow/training-operator/issues/1078.
// originally from pkg/controller.v1/tensorflow/pod.go (deleted)
func isDistributed(msjob *mindsporev1.MSJob) bool {
	replicas := msjob.Spec.MSReplicaSpecs
	distributionCount := 0
	allTypes := []commonv1.ReplicaType{
		mindsporev1.MSReplicaTypeScheduler,
		mindsporev1.MSReplicaTypePS,
		mindsporev1.MSReplicaTypeWorker,
	}
	// Check if there is only one replica.
	for _, typ := range allTypes {
		if replicas[typ] != nil {
			if replicas[typ].Replicas == nil {
				distributionCount++
			} else {
				distributionCount += int(*replicas[typ].Replicas)
			}
		}
	}
	return distributionCount != 1
}

// initializeReplicaStatuses initializes the ReplicaStatuses for replica.
// originally from pkg/controller.v1/tensorflow/status.go (deleted)
func initializeReplicaStatuses(jobStatus *commonv1.JobStatus, rtype commonv1.ReplicaType) {
	if jobStatus.ReplicaStatuses == nil {
		jobStatus.ReplicaStatuses = make(map[commonv1.ReplicaType]*commonv1.ReplicaStatus)
	}

	jobStatus.ReplicaStatuses[rtype] = &commonv1.ReplicaStatus{}
}

// updateJobReplicaStatuses updates the JobReplicaStatuses according to the pod.
// originally from pkg/controller.v1/tensorflow/status.go (deleted)
func updateJobReplicaStatuses(jobStatus *commonv1.JobStatus, rtype commonv1.ReplicaType, pod *corev1.Pod) {
	switch pod.Status.Phase {
	case corev1.PodRunning:
		jobStatus.ReplicaStatuses[rtype].Active++
	case corev1.PodSucceeded:
		jobStatus.ReplicaStatuses[rtype].Succeeded++
	case corev1.PodFailed:
		jobStatus.ReplicaStatuses[rtype].Failed++
	}
}

func getServiceIpAndPort(service *v1.Service) (string, string) {
	schedulerPort := ""
	for _, port := range service.Spec.Ports {
		if port.Name == mindsporev1.DefaultPortName {
			schedulerPort = strconv.Itoa(int(port.Port))
			break
		}
	}
	return service.Spec.ClusterIP, schedulerPort
}

func genPSAndWorkerNum(msjob *mindsporev1.MSJob) (string, string) {
	msServerNumStr, msWorkerNumStr := "0", "0"
	for rtype, spec := range msjob.Spec.MSReplicaSpecs {
		if isServer(rtype) {
			msServerNumStr = strconv.Itoa(int(*spec.Replicas))
		}
		if isWorker(rtype) {
			msWorkerNumStr = strconv.Itoa(int(*spec.Replicas))
		}
	}
	return msServerNumStr, msWorkerNumStr

}

// isScheduler returns true if the type is Scheduler.
func isScheduler(typ commonv1.ReplicaType) bool {
	return typ == mindsporev1.MSReplicaTypeScheduler
}

// isWorker returns true if the type is Worker.
func isWorker(typ commonv1.ReplicaType) bool {
	return typ == mindsporev1.MSReplicaTypeWorker
}

// isServer returns true if the type is PS.
func isServer(typ commonv1.ReplicaType) bool {
	return typ == mindsporev1.MSReplicaTypePS
}

func validateV1ReplicaSpecs(specs map[commonv1.ReplicaType]*commonv1.ReplicaSpec) error {
	if specs == nil {
		return fmt.Errorf("MSJobSpec is not valid")
	}
	foundScheduler := 0
	for rType, value := range specs {
		if value == nil || len(value.Template.Spec.Containers) == 0 {
			return fmt.Errorf("MSJobSpec is not valid: containers definition expected in %v", rType)
		}
		if isScheduler(rType) {
			foundScheduler++
		}
		// Make sure the image is defined in the container.
		numNamedMindSpore := 0
		for _, container := range value.Template.Spec.Containers {
			if container.Image == "" {
				msg := fmt.Sprintf("MSJobSpec is not valid: Image is undefined in the container of %v", rType)
				log.Error(msg)
				return fmt.Errorf(msg)
			}
			if container.Name == mindsporev1.DefaultContainerName {
				numNamedMindSpore++
			}
		}
		if numNamedMindSpore == 0 {
			msg := fmt.Sprintf("MSJobSpec is not valid: There is no container named %s in %v", mindsporev1.DefaultContainerName, rType)
			log.Error(msg)
			return fmt.Errorf(msg)
		}
	}
	if foundScheduler > 1 {
		return fmt.Errorf("MSJobSpec is not valid: %d Scheduler found", foundScheduler)
	}

	return nil
}
