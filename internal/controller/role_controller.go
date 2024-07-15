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

package controller

import (
	"context"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	rbacv1 "k8s.io/api/rbac/v1"
	rolev1 "kube-auth.io/api/v1"
)

// RoleReconciler reconciles a Role object
type RoleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=context.kube-auth,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=context.kube-auth,resources=roles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=context.kube-auth,resources=roles/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=roles,verbs=get;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *RoleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	CRDRole := &rolev1.Role{}
	logger.Info("Fetching Role", "Role.Namespace", req.Namespace, "Role.Name", req.Name)
	err := r.Get(ctx, req.NamespacedName, CRDRole)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Error(err, "Role resource not found")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Role")
		return ctrl.Result{}, err
	}

	// Create a Role if not existent
	role := &rbacv1.ClusterRole{}
	newRole := r.createK8sRole(CRDRole)
	err = nil
	if CRDRole.Status.Role == "" {
		logger.Info("Creating a new Role", "Role.Namespace", newRole.Namespace, "Role.Name", newRole.Name)
		err = r.Create(ctx, newRole)
		if err != nil {
			return ctrl.Result{}, err
		}
		CRDRole.Status.Role = newRole.Name
		err = r.Status().Update(ctx, CRDRole)
		if err != nil {
			logger.Error(err, "Failed to update Role status")
			return ctrl.Result{}, err
		}
	} else {
		err = r.Get(ctx, types.NamespacedName{Name: CRDRole.Status.Role, Namespace: CRDRole.Namespace}, role)
		if err != nil {
			logger.Error(err, "Failed to get Role")
			return ctrl.Result{}, err
		}
		logger.Info("Updating Role", "Role.Namespace", newRole.Namespace, "Role.Name", newRole.Name)
		err = r.Update(ctx, newRole)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil

	//
	//// Create a Role Binding if not existent
	//roleBinding := &rbacv1.ClusterRoleBinding{}
	//err = r.Get(ctx, types.NamespacedName{Name: CRDRole.Name, Namespace: CRDRole.Namespace}, roleBinding)
	//if err != nil && errors.IsNotFound(err) {
	//	// Define a new role binding
	//	newRoleBinding := r.createK8sRoleBinding(CRDRole)
	//	logger.Info("Creating a new RoleBinding", "RoleBinding.Namespace", newRoleBinding.Namespace, "RoleBinding.Name", newRoleBinding.Name)
	//	err = r.Create(ctx, newRoleBinding)
	//	if err != nil {
	//		return ctrl.Result{}, err
	//	}
	//	// RoleBinding created successfully - return and requeue
	//	CRDRole.Status.RoleBinding = newRoleBinding.Name
	//} else if err != nil {
	//	return ctrl.Result{}, err
	//}
	//
	//return ctrl.Result{}, nil
}

func (r RoleReconciler) createK8sRole(k *rolev1.Role) *rbacv1.ClusterRole {
	var k8sRules []rbacv1.PolicyRule
	// Create a namespace Rule
	{
		rule := rbacv1.PolicyRule{
			Verbs:     []string{"get", "list", "watch"},
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
		}
		if k.Spec.NamespaceRole.Create == nil || *k.Spec.NamespaceRole.Create {
			rule.Verbs = append(rule.Verbs, "create")
		}
		if k.Spec.NamespaceRole.Delete == nil || *k.Spec.NamespaceRole.Delete {
			rule.Verbs = append(rule.Verbs, "delete")
		}
		k8sRules = append(k8sRules, rule)
	}
	// Create a rule for each cluster role
	{
		for _, clusterRole := range k.Spec.ClusterRole {
			rule := rbacv1.PolicyRule{
				Verbs:     clusterRole.Verbs,
				APIGroups: []string{""},
				Resources: clusterRole.Include.Resources,
			}
			k8sRules = append(k8sRules, rule)
		}
	}
	var role *rbacv1.ClusterRole
	role = &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k.Name + "-role",
			Namespace: k.Namespace,
		},
		Rules: k8sRules,
	}
	if k.Status.Role != "" {
		role.Name = k.Status.Role
	}

	err := ctrl.SetControllerReference(k, role, r.Scheme)
	if err != nil {
		return nil
	}
	return role
}

//func (r *RoleReconciler) createK8sRoleBinding(k *rolev1.Role) *rbacv1.ClusterRoleBinding {
//	roleBinding := &rbacv1.ClusterRoleBinding{
//		ObjectMeta: metav1.ObjectMeta{
//			Name:      k.Name,
//			Namespace: k.Namespace,
//		},
//		Subjects: k.Spec.Subjects,
//		RoleRef: rbacv1.RoleRef{
//			Kind: "Role",
//			Name: k.Name,
//		},
//	}
//	ctrl.SetControllerReference(k, roleBinding, r.Scheme)
//	return roleBinding
//
//}

// SetupWithManager sets up the controller with the Manager.
func (r *RoleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rolev1.Role{}).
		//Owns(&rbacv1.ClusterRole{}).
		//Owns(&rbacv1.ClusterRoleBinding{}).
		Complete(r)
}
