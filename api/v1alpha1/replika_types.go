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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type SynchronizationSpec struct {
	Time string `json:"time"`
}

//
type ReplikaTargetSpec struct {
	Namespaces []string `json:"namespaces,omitempty"`
}

//
type ReplikaSourceSpec struct {
	Group     string `json:"group"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// ReplikaSpec defines the desired state of Replika
type ReplikaSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// SynchronizationSpec defines the behavior of synchronization
	Synchronization SynchronizationSpec `json:"synchronization"`

	// ReplikaSourceSpec define the source resource
	Source ReplikaSourceSpec `json:"source,omitempty"`

	// ReplikaTargetSpec defines the target [...]
	Target ReplikaTargetSpec `json:"target"`
}

// ReplikaStatus defines the observed state of Replika
type ReplikaStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Replika is the Schema for the replikas API
type Replika struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReplikaSpec   `json:"spec,omitempty"`
	Status ReplikaStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ReplikaList contains a list of Replika
type ReplikaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Replika `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Replika{}, &ReplikaList{})
}
