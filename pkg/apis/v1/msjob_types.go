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
	commonv1 "github.com/kubeflow/common/pkg/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=msjob
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.conditions[-1:].type`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// MSJob is the Schema for the msjobs API
type MSJob struct {
	// Standard Kubernetes type metadata.
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired state of the MSJob.
	// +optional
	Spec MSJobSpec `json:"spec,omitempty"`

	// Most recently observed status of the MSJob.
	// Populated by the system.
	// Read-only.
	// +optional
	Status commonv1.JobStatus `json:"status,omitempty"`
}

// MSJobSpec defines the desired state of MSJob
type MSJobSpec struct {
	// RunPolicy encapsulates various runtime policies of the distributed training
	// job, for example how to clean up resources and how long the job can stay
	// active.
	//+kubebuilder:validation:Optional
	RunPolicy commonv1.RunPolicy `json:"runPolicy"`

	// SuccessPolicy defines the policy to mark the MSJob as succeeded.
	// Default to "", using the default rules.
	// +optional
	SuccessPolicy *SuccessPolicy `json:"successPolicy,omitempty"`

	// A map of MSReplicaType (type) to ReplicaSpec (value). Specifies the MS cluster configuration.
	// For example,
	//   {
	//     "Scheduler": ReplacaSpec,
	//     "PS": ReplicaSpec,
	//     "Worker": ReplicaSpec,
	//   }
	MSReplicaSpecs map[commonv1.ReplicaType]*commonv1.ReplicaSpec `json:"msReplicaSpecs"`

	// A switch to enable dynamic worker
	EnableDynamicWorker bool `json:"enableDynamicWorker,omitempty"`
}

// MSReplicaType is the type for MSReplica. Can be one of: "Scheduler",
// "Worker" or "PS".

const (
	// MSReplicaTypePS is the type for parameter servers of distributed MS.
	MSReplicaTypePS commonv1.ReplicaType = "PS"

	// MSReplicaTypeWorker is the type for workers of distributed MS.
	// This is also used for non-distributed MS.
	MSReplicaTypeWorker commonv1.ReplicaType = "Worker"

	// MSReplicaTypeScheduler is the type for Scheduler of distributed MS.
	MSReplicaTypeScheduler commonv1.ReplicaType = "Scheduler"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=msjobs
//+kubebuilder:object:root=true
// MSJobList contains a list of MSJob
type MSJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MSJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MSJob{}, &MSJobList{})
	SchemeBuilder.SchemeBuilder.Register(addDefaultingFuncs)
}
