/*
Copyright 2018 The Scipian Team.

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

// WorkspaceSpec defines the desired state of Workspace
type WorkspaceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Image      string           `json:"image"`
	Secret     string           `json:"secret"`
	WorkingDir string           `json:"workingDir"`
	Backend    WorkspaceBackend `json:"backend"`
	// TODO(NL): Think about renaming this
	RemoteState string            `json:"remoteState"`
	EnvVars     map[string]string `json:"envVars,omitempty"`
	TfVars      map[string]string `json:"tfVars,omitempty"`
}

// WorkspaceBackend defines the desired state of a backend for a Workspace
type WorkspaceBackend struct {
	Type          string `json:"type"`
	Bucket        string `json:"bucket"`
	Key           string `json:"key"`
	Region        string `json:"region"`
	DynamoDBTable string `json:"dynamodb_table"`
}

// WorkspaceStatus defines the observed state of Workspace
type WorkspaceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Workspace is the Schema for the workspaces API
// +k8s:openapi-gen=true
type Workspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkspaceSpec   `json:"spec,omitempty"`
	Status WorkspaceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkspaceList contains a list of Workspace
type WorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workspace `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Workspace{}, &WorkspaceList{})
}