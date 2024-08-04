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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
type ResourceInstances struct {
	Kind string   `json:"kind"`
	Name []string `json:"name"`
}

type NamespaceRole struct {
	Create *bool `json:"create"`
	Delete *bool `json:"delete"`
}

type ClusterRole struct {
	Contexts   []string            `json:"contexts"`
	Namespaces []string            `json:"namespaces"`
	Resources  []string            `json:"resources"`
	Instances  []ResourceInstances `json:"instances"`
	Verbs      []string            `json:"verbs"`
}

// RoleSpec defines the desired state of Role
type RoleSpec struct {
	NamespaceRole NamespaceRole `json:"namespaceRole"`
	ClusterRole   []ClusterRole `json:"clusterRole"`
}

type HandledNamespace struct {
	Namespace string              `json:"namespace"`
	Roles     []rbacv1.PolicyRule `json:"roles"`
}

// RoleStatus defines the observed state of Role
type RoleStatus struct {
	HandledNamespaces  []HandledNamespace `json:"handledNamespaces,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         []ContextCondition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Role is the Schema for the roles API
type Role struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RoleSpec   `json:"spec,omitempty"`
	Status RoleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RoleList contains a list of Role
type RoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Role `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Role{}, &RoleList{})
}
