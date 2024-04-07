/*
Copyright 2024.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// VpcDnsSpec defines the desired state of VpcDns
type VpcDnsSpec struct {
	Vpc string `json:"vpc,omitempty"`
}

// VpcDnsStatus defines the observed state of VpcDns
type VpcDnsStatus struct {
	Initialized bool `json:"initialized,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// VpcDns is the Schema for the vpcdns API
type VpcDns struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VpcDnsSpec   `json:"spec,omitempty"`
	Status VpcDnsStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VpcDnsList contains a list of VpcDns
type VpcDnsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VpcDns `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VpcDns{}, &VpcDnsList{})
}