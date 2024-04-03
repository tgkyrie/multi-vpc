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

type VpcConnectionState string

type VpcConnectionOperation string

const (
	DNSConnectionRunning VpcConnectionState = "DNSConnectionRunning"
	VpcConnectionRunning VpcConnectionState = "VpcConnectionRunning"
)

const (
	DnsConnectionCreate   VpcConnectionOperation = "DnsConnectionCreate"
	VpcConnectionCreate   VpcConnectionOperation = "VpcConnectionCreate"
	VpcConnectionRecovery VpcConnectionOperation = "Recovery"
	VpcConnectionStop     VpcConnectionOperation = "Stop"
)

// VpcConnectionSpec defines the desired state of VpcConnection
type VpcConnectionSpec struct {
	Vpc        string                 `json:"vpc,omitempty"`
	Gateway    string                 `json:"gateway,omitempty"`
	SubnetCIDR string                 `json:"cidr,omitempty"`
	SubnetIP   string                 `json:"ip,omitempty"`
	Operation  VpcConnectionOperation `json:"operation,omitempty"`
}

// VpcConnectionStatus defines the observed state of VpcConnection
type VpcConnectionStatus struct {
	State VpcConnectionState `json:"state,omitempty"`
}

// VpcConnection is the Schema for the vpcconnections API
type VpcConnection struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VpcConnectionSpec   `json:"spec,omitempty"`
	Status VpcConnectionStatus `json:"status,omitempty"`
}

// VpcConnectionList contains a list of VpcConnection
type VpcConnectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VpcConnection `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VpcConnection{}, &VpcConnectionList{})
}
