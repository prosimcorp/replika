/*
Copyright 2022.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SynchronizationSpec defines the spec of the synchronization section of a Replika
type SynchronizationSpec struct {
	Time string `json:"time"`
}

// ReplikaTargetNamespacesSpec defines the spec of the target namespaces section of a Replika
type ReplikaTargetNamespacesSpec struct {
	ReplicateIn []string `json:"replicateIn,omitempty"`
	MatchAll    bool     `json:"matchAll"`
	ExcludeFrom []string `json:"excludeFrom,omitempty"`
}

// ReplikaTargetSpec defines the spec of the target section of a Replica
type ReplikaTargetSpec struct {
	Namespaces ReplikaTargetNamespacesSpec `json:"namespaces,omitempty"`
}

// ReplikaSourceSpec defines the spec of the source section of a Replika
type ReplikaSourceSpec struct {
	Group     string `json:"group"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// ReplikaSpec defines the desired state of a Replika
type ReplikaSpec struct {

	// SynchronizationSpec defines the behavior of synchronization
	Synchronization SynchronizationSpec `json:"synchronization"`

	// ReplikaSourceSpec define the source resource
	Source ReplikaSourceSpec `json:"source,omitempty"`

	// ReplikaTargetSpec defines the target [...]
	Target ReplikaTargetSpec `json:"target"`
}

// ReplikaStatus defines the observed state of a Replika
type ReplikaStatus struct {

	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Namespaced,categories={replikas}
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"SourceSynced\")].status",description=""
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"SourceSynced\")].reason",description=""
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// Replika is the Schema for the each Replika CR
type Replika struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReplikaSpec   `json:"spec,omitempty"`
	Status ReplikaStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ReplikaList contains a list of Replika resources
type ReplikaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Replika `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Replika{}, &ReplikaList{})
}
