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
	common "github.com/kubeflow/common/pkg/apis/common/v1"
)

const (
	// DefaultPortName is name of the port used to communicate between Scheduler, PS and
	// workers.
	DefaultPortName = "msjob-port"
	// DefaultContainerName is the name of the MSJob container.
	DefaultContainerName = "mindspore"
	// DefaultPort is default value of the port.
	DefaultPort = 2222
	// DefaultRestartPolicy is default RestartPolicy for MSReplicaSpec.
	DefaultRestartPolicy = common.RestartPolicyNever
	// Kind is the kind name.
	Kind = "MSJob"
	// Plural is the Plural for MSJob.
	Plural = "msjobs"
	// Singular is the singular for MSJob.
	Singular = "msjob"
	// FrameworkName is the name of the ML Framework
	FrameworkName = "mindspore"
)
