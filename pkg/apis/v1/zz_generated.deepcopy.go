//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	commonv1 "github.com/kubeflow/common/pkg/apis/common/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MSJob) DeepCopyInto(out *MSJob) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MSJob.
func (in *MSJob) DeepCopy() *MSJob {
	if in == nil {
		return nil
	}
	out := new(MSJob)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MSJob) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MSJobList) DeepCopyInto(out *MSJobList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]MSJob, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MSJobList.
func (in *MSJobList) DeepCopy() *MSJobList {
	if in == nil {
		return nil
	}
	out := new(MSJobList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MSJobList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MSJobSpec) DeepCopyInto(out *MSJobSpec) {
	*out = *in
	in.RunPolicy.DeepCopyInto(&out.RunPolicy)
	if in.SuccessPolicy != nil {
		in, out := &in.SuccessPolicy, &out.SuccessPolicy
		*out = new(SuccessPolicy)
		**out = **in
	}
	if in.MSReplicaSpecs != nil {
		in, out := &in.MSReplicaSpecs, &out.MSReplicaSpecs
		*out = make(map[commonv1.ReplicaType]*commonv1.ReplicaSpec, len(*in))
		for key, val := range *in {
			var outVal *commonv1.ReplicaSpec
			if val == nil {
				(*out)[key] = nil
			} else {
				in, out := &val, &outVal
				*out = new(commonv1.ReplicaSpec)
				(*in).DeepCopyInto(*out)
			}
			(*out)[key] = outVal
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MSJobSpec.
func (in *MSJobSpec) DeepCopy() *MSJobSpec {
	if in == nil {
		return nil
	}
	out := new(MSJobSpec)
	in.DeepCopyInto(out)
	return out
}
