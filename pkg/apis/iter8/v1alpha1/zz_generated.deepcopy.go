// +build !ignore_autogenerated

/*

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
// Code generated by main. DO NOT EDIT.

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Analysis) DeepCopyInto(out *Analysis) {
	*out = *in
	if in.SuccessCriteria != nil {
		in, out := &in.SuccessCriteria, &out.SuccessCriteria
		*out = make([]SuccessCriterion, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Reward != nil {
		in, out := &in.Reward, &out.Reward
		*out = new(Reward)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Analysis.
func (in *Analysis) DeepCopy() *Analysis {
	if in == nil {
		return nil
	}
	out := new(Analysis)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Experiment) DeepCopyInto(out *Experiment) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	if in.Metrics != nil {
		in, out := &in.Metrics, &out.Metrics
		*out = make(ExperimentMetrics, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Experiment.
func (in *Experiment) DeepCopy() *Experiment {
	if in == nil {
		return nil
	}
	out := new(Experiment)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Experiment) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExperimentList) DeepCopyInto(out *ExperimentList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Experiment, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExperimentList.
func (in *ExperimentList) DeepCopy() *ExperimentList {
	if in == nil {
		return nil
	}
	out := new(ExperimentList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ExperimentList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExperimentMetric) DeepCopyInto(out *ExperimentMetric) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExperimentMetric.
func (in *ExperimentMetric) DeepCopy() *ExperimentMetric {
	if in == nil {
		return nil
	}
	out := new(ExperimentMetric)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in ExperimentMetrics) DeepCopyInto(out *ExperimentMetrics) {
	{
		in := &in
		*out = make(ExperimentMetrics, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
		return
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExperimentMetrics.
func (in ExperimentMetrics) DeepCopy() ExperimentMetrics {
	if in == nil {
		return nil
	}
	out := new(ExperimentMetrics)
	in.DeepCopyInto(out)
	return *out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExperimentSpec) DeepCopyInto(out *ExperimentSpec) {
	*out = *in
	in.TargetService.DeepCopyInto(&out.TargetService)
	in.TrafficControl.DeepCopyInto(&out.TrafficControl)
	in.Analysis.DeepCopyInto(&out.Analysis)
	if in.RoutingReference != nil {
		in, out := &in.RoutingReference, &out.RoutingReference
		*out = new(v1.ObjectReference)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExperimentSpec.
func (in *ExperimentSpec) DeepCopy() *ExperimentSpec {
	if in == nil {
		return nil
	}
	out := new(ExperimentSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExperimentStatus) DeepCopyInto(out *ExperimentStatus) {
	*out = *in
	in.Status.DeepCopyInto(&out.Status)
	in.LastIncrementTime.DeepCopyInto(&out.LastIncrementTime)
	in.AnalysisState.DeepCopyInto(&out.AnalysisState)
	in.AssessmentSummary.DeepCopyInto(&out.AssessmentSummary)
	out.TrafficSplit = in.TrafficSplit
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExperimentStatus.
func (in *ExperimentStatus) DeepCopy() *ExperimentStatus {
	if in == nil {
		return nil
	}
	out := new(ExperimentStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Host) DeepCopyInto(out *Host) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Host.
func (in *Host) DeepCopy() *Host {
	if in == nil {
		return nil
	}
	out := new(Host)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MinMax) DeepCopyInto(out *MinMax) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MinMax.
func (in *MinMax) DeepCopy() *MinMax {
	if in == nil {
		return nil
	}
	out := new(MinMax)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Reward) DeepCopyInto(out *Reward) {
	*out = *in
	if in.MinMax != nil {
		in, out := &in.MinMax, &out.MinMax
		*out = new(MinMax)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Reward.
func (in *Reward) DeepCopy() *Reward {
	if in == nil {
		return nil
	}
	out := new(Reward)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SuccessCriterion) DeepCopyInto(out *SuccessCriterion) {
	*out = *in
	if in.SampleSize != nil {
		in, out := &in.SampleSize, &out.SampleSize
		*out = new(int)
		**out = **in
	}
	if in.MinMax != nil {
		in, out := &in.MinMax, &out.MinMax
		*out = new(MinMax)
		**out = **in
	}
	if in.StopOnFailure != nil {
		in, out := &in.StopOnFailure, &out.StopOnFailure
		*out = new(bool)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SuccessCriterion.
func (in *SuccessCriterion) DeepCopy() *SuccessCriterion {
	if in == nil {
		return nil
	}
	out := new(SuccessCriterion)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SuccessCriterionStatus) DeepCopyInto(out *SuccessCriterionStatus) {
	*out = *in
	if in.Conclusions != nil {
		in, out := &in.Conclusions, &out.Conclusions
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SuccessCriterionStatus.
func (in *SuccessCriterionStatus) DeepCopy() *SuccessCriterionStatus {
	if in == nil {
		return nil
	}
	out := new(SuccessCriterionStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Summary) DeepCopyInto(out *Summary) {
	*out = *in
	if in.Conclusions != nil {
		in, out := &in.Conclusions, &out.Conclusions
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.SuccessCriteriaStatus != nil {
		in, out := &in.SuccessCriteriaStatus, &out.SuccessCriteriaStatus
		*out = make([]SuccessCriterionStatus, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Summary.
func (in *Summary) DeepCopy() *Summary {
	if in == nil {
		return nil
	}
	out := new(Summary)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TargetService) DeepCopyInto(out *TargetService) {
	*out = *in
	if in.ObjectReference != nil {
		in, out := &in.ObjectReference, &out.ObjectReference
		*out = new(v1.ObjectReference)
		**out = **in
	}
	if in.Hosts != nil {
		in, out := &in.Hosts, &out.Hosts
		*out = make([]Host, len(*in))
		copy(*out, *in)
	}
	if in.Port != nil {
		in, out := &in.Port, &out.Port
		*out = new(int32)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TargetService.
func (in *TargetService) DeepCopy() *TargetService {
	if in == nil {
		return nil
	}
	out := new(TargetService)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TrafficControl) DeepCopyInto(out *TrafficControl) {
	*out = *in
	if in.Strategy != nil {
		in, out := &in.Strategy, &out.Strategy
		*out = new(string)
		**out = **in
	}
	if in.MaxTrafficPercentage != nil {
		in, out := &in.MaxTrafficPercentage, &out.MaxTrafficPercentage
		*out = new(float64)
		**out = **in
	}
	if in.TrafficStepSize != nil {
		in, out := &in.TrafficStepSize, &out.TrafficStepSize
		*out = new(float64)
		**out = **in
	}
	if in.Interval != nil {
		in, out := &in.Interval, &out.Interval
		*out = new(string)
		**out = **in
	}
	if in.MaxIterations != nil {
		in, out := &in.MaxIterations, &out.MaxIterations
		*out = new(int)
		**out = **in
	}
	if in.OnSuccess != nil {
		in, out := &in.OnSuccess, &out.OnSuccess
		*out = new(string)
		**out = **in
	}
	if in.Confidence != nil {
		in, out := &in.Confidence, &out.Confidence
		*out = new(float64)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TrafficControl.
func (in *TrafficControl) DeepCopy() *TrafficControl {
	if in == nil {
		return nil
	}
	out := new(TrafficControl)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TrafficSplit) DeepCopyInto(out *TrafficSplit) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TrafficSplit.
func (in *TrafficSplit) DeepCopy() *TrafficSplit {
	if in == nil {
		return nil
	}
	out := new(TrafficSplit)
	in.DeepCopyInto(out)
	return out
}
