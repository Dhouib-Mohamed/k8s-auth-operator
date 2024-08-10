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

type ResourceInstances struct {
	// +kubebuilder:validation:Enum=pod;deployment;service;configmap;secret;ingress;persistentvolumeclaim;persistentvolume;serviceaccount;role;rolebinding;clusterrole;clusterrolebinding;namespace;networkpolicy;podsecuritypolicy;limitrange;resourcequota;horizontalpodautoscaler;poddisruptionbudget;priorityclass;storageclass;volumeattachment;csidriver;csinode;customresourcedefinition;mutatingwebhookconfiguration;validatingwebhookconfiguration;customresourcedefinitionlist;mutatingwebhookconfigurationlist;validatingwebhookconfigurationlist;podlist;deploymentlist;servicelist;configmaplist;secretlist;ingresslist;persistentvolumeclaimlist;persistentvolumelist;serviceaccountlist;rolelist;rolebindinglist;clusterrolelist;clusterrolebindinglist;namespacelist;networkpolicylist;podsecuritypolicylist;limitrangelist;resourcequotalist;horizontalpodautoscalerlist;poddisruptionbudgetlist;priorityclasslist;storageclasslist;volumeattachmentlist;csidriverlist;csinodelist;customresourcedefinitionlistlist;mutatingwebhookconfigurationlistlist;validatingwebhookconfigurationlistlist
	Kind string `json:"kind"`
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:UniqueItems=true
	Name []string `json:"name"`
}

type NamespaceRole struct {
	Create *bool `json:"create"`
	Delete *bool `json:"delete"`
}

// +kubebuilder:validation:Enum=get;list;watch;create;update;patch;delete;deletecollection
type RoleVerb string

type ClusterRole struct {
	// +kubebuilder:default:=[]
	Contexts []string `json:"contexts,omitempty"`
	// +kubebuilder:default:=[]
	Namespaces []string `json:"namespaces,omitempty"`
	// +kubebuilder:default:=[]
	// +kubebuilder:validation:UniqueItems=true
	Resources []string `json:"resources,omitempty"`
	// +kubebuilder:default:=[]
	// +kubebuilder:validation:UniqueItems=true
	Instances []ResourceInstances `json:"instances,omitempty"`
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:UniqueItems=true
	Verbs []RoleVerb `json:"verbs"`
}

type RoleSpec struct {
	NamespaceRole NamespaceRole `json:"namespaceRole"`
	ClusterRole   []ClusterRole `json:"clusterRole"`
}

type HandledNamespace struct {
	Namespace string              `json:"namespace"`
	Roles     []rbacv1.PolicyRule `json:"roles"`
}

type RoleStatus struct {
	HandledNamespaces  []HandledNamespace `json:"handledNamespaces,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         []ContextCondition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

type Role struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RoleSpec   `json:"spec,omitempty"`
	Status RoleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type RoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Role `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Role{}, &RoleList{})
}
