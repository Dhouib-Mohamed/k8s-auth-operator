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

package role

import (
	"context"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	contextv1 "kube-auth.io/api/v1"
	"kube-auth.io/internal/controller/utils"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
			return r.UpdateStatus(ctx, CRDRole, nil, utils.BasicCondition{
				Type:    contextv1.TypeNotReady,
				Status:  contextv1.StatusFalse,
				Reason:  "Role not found",
				Message: "Role not found",
			}, nil)
		}
		return r.UpdateStatus(ctx, CRDRole, nil, utils.BasicCondition{
			Type:    contextv1.TypeNotReady,
			Status:  contextv1.StatusFalse,
			Reason:  "Error fetching role",
			Message: err.Error(),
		}, err)
	}

	err = r.createNamespacesRole(CRDRole.Name, CRDRole.Spec.NamespaceRole)
	if err != nil {
		return r.UpdateStatus(ctx, CRDRole, nil, utils.BasicCondition{
			Type:    contextv1.TypeNotReady,
			Status:  contextv1.StatusFalse,
			Reason:  "Error creating namespace role",
			Message: err.Error(),
		}, err)
	}

	handledNamespaces, err := r.extractHandledNamespaces(ctx, CRDRole.Spec.ClusterRole, req.NamespacedName.Namespace)
	if err != nil {
		return r.UpdateStatus(ctx, CRDRole, nil, utils.BasicCondition{
			Type:    contextv1.TypeNotReady,
			Status:  contextv1.StatusFalse,
			Reason:  "Error extracting handled namespaces",
			Message: err.Error(),
		}, err)
	}
	for _, handledNs := range handledNamespaces {
		err = r.createOrUpdateRole(CRDRole.Name+"-"+handledNs.Namespace, handledNs.Namespace, handledNs.Roles)
		if err != nil {
			return r.UpdateStatus(ctx, CRDRole, nil, utils.BasicCondition{
				Type:    contextv1.TypeNotReady,
				Status:  contextv1.StatusFalse,
				Reason:  "Error creating role",
				Message: err.Error(),
			}, err)
		}
	}
	for _, existentNs := range CRDRole.Status.HandledNamespaces {
		found := false
		for _, handledNs := range handledNamespaces {
			if existentNs.Namespace == handledNs.Namespace {
				found = true
				break
			}
		}
		if !found {
			err = r.deleteRole(CRDRole.Name+"-"+existentNs.Namespace, existentNs.Namespace)
			if err != nil {
				return r.UpdateStatus(ctx, CRDRole, nil, utils.BasicCondition{
					Type:    contextv1.TypeNotReady,
					Status:  contextv1.StatusFalse,
					Reason:  "Error deleting role",
					Message: err.Error(),
				}, err)
			}
		}
	}
	return r.UpdateStatus(ctx, CRDRole, handledNamespaces, utils.BasicCondition{
		Type:    contextv1.TypeReady,
		Status:  contextv1.StatusTrue,
		Reason:  "Role reconciled",
		Message: "Role reconciled",
	}, nil)
}

func (r *RoleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&contextv1.Role{}).
		Watches(&contextv1.Context{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, object client.Object) []reconcile.Request {
				roles := &contextv1.RoleList{}
				err := r.List(ctx, roles)
				if err != nil {
					return nil
				}
				var requests []reconcile.Request
				for _, item := range roles.Items {
					for _, role := range item.Spec.ClusterRole {
						pass := false
						for _, context := range role.Contexts {
							if context == object.GetName() {
								requests = append(requests, reconcile.Request{
									NamespacedName: client.ObjectKey{
										Namespace: item.Namespace,
										Name:      item.Name,
									},
								})
								pass = true
								break
							}
						}
						if pass {
							break
						}
					}
				}
				return requests
			}),
		).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		WithEventFilter(utils.FilterFuncs([]string{"*v1.Context"})).
		Complete(r)
}

func (r *RoleReconciler) UpdateStatus(ctx context.Context, role *contextv1.Role, handledNamespaces []contextv1.HandledNamespace, condition utils.BasicCondition, Error error) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if role == nil {
		return utils.HandleError(logger, Error, condition.Message)
	}

	newConditions := utils.SyncConditions(role.Status.Conditions, condition)
	if reflect.DeepEqual(role.Status.Conditions, newConditions) && reflect.DeepEqual(handledNamespaces, role.Status.HandledNamespaces) {
		return ctrl.Result{}, nil
	}
	role.Status.HandledNamespaces = handledNamespaces
	role.Status.Conditions = newConditions
	role.Status.ObservedGeneration = role.Status.ObservedGeneration + 1
	updateErr := r.Status().Update(ctx, role)
	if updateErr != nil {
		if errors.IsConflict(updateErr) {
			logger.Info("Conflict while updating status, retrying")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, updateErr
	}
	return utils.HandleError(logger, Error, condition.Message)
}
