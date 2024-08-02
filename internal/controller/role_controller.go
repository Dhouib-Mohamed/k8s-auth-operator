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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"kube-auth.io/internal/controller/utils"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	rbacv1 "k8s.io/api/rbac/v1"
	contextv1 "kube-auth.io/api/v1"
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
	CRDRole := &contextv1.Role{}
	err := r.Get(ctx, req.NamespacedName, CRDRole)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.UpdateStatus(ctx, CRDRole, "", "", utils.BasicCondition{
				Type:    contextv1.TypeNotReady,
				Status:  contextv1.StatusFalse,
				Reason:  "Role not found",
				Message: "Role not found",
			}, nil)
		}
		return r.UpdateStatus(ctx, CRDRole, "", "", utils.BasicCondition{
			Type:    contextv1.TypeNotReady,
			Status:  contextv1.StatusFalse,
			Reason:  "Error fetching role",
			Message: err.Error(),
		}, err)
	}

	// Create a Role if not existent
	role := &rbacv1.ClusterRole{}
	newRole := r.createK8sRole(CRDRole)
	err = nil
	if CRDRole.Status.Role == "" {
		err = r.Create(ctx, newRole)
		if err != nil {
			return r.UpdateStatus(ctx, CRDRole, "", "", utils.BasicCondition{
				Type:    contextv1.TypeNotReady,
				Status:  contextv1.StatusFalse,
				Reason:  "Error creating role",
				Message: err.Error(),
			}, err)
		}
	} else {
		err = r.Get(ctx, types.NamespacedName{Name: CRDRole.Status.Role, Namespace: CRDRole.Namespace}, role)
		if err != nil {
			return r.UpdateStatus(ctx, CRDRole, "", "", utils.BasicCondition{
				Type:    contextv1.TypeNotReady,
				Status:  contextv1.StatusFalse,
				Reason:  "Error fetching role",
				Message: err.Error(),
			}, err)
		}
		err = r.Update(ctx, newRole)
		if err != nil {
			return r.UpdateStatus(ctx, CRDRole, "", "", utils.BasicCondition{
				Type:    contextv1.TypeNotReady,
				Status:  contextv1.StatusFalse,
				Reason:  "Error updating role",
				Message: err.Error(),
			}, err)
		}
	}

	return r.UpdateStatus(ctx, CRDRole, newRole.Name, "", utils.BasicCondition{
		Type:    contextv1.TypeReady,
		Status:  contextv1.StatusTrue,
		Reason:  "Role created",
		Message: "Role created",
	}, nil)

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

func (r RoleReconciler) createK8sRole(k *contextv1.Role) *rbacv1.ClusterRole {
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

//func (r *RoleReconciler) createK8sRoleBinding(k *contextv1.Role) *rbacv1.ClusterRoleBinding {
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
		For(&contextv1.Role{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
				return e.Object.GetGeneration() == 1
			},
			DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool { return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() },
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		}).
		//Owns(&rbacv1.ClusterRole{}).
		//Owns(&rbacv1.ClusterRoleBinding{}).
		Complete(r)
}

func (r *RoleReconciler) UpdateStatus(ctx context.Context, role *contextv1.Role, roleName string, roleBindingName string, condition utils.BasicCondition, Error error) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if role == nil {
		return utils.HandleError(logger, Error, condition.Message)
	}
	role.Status.Role = roleName
	role.Status.RoleBinding = roleBindingName
	role.Status.ObservedGeneration = role.Status.ObservedGeneration + 1
	role.Status.Conditions = utils.SyncConditions(role.Status.Conditions, condition)
	err := r.Status().Update(ctx, role)
	if err != nil {
		return ctrl.Result{}, err
	}
	return utils.HandleError(logger, Error, condition.Message)
}
