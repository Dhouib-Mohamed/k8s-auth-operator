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

package context

import (
	"context"
	corev1 "k8s.io/api/core/v1"
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

// ContextReconciler reconciles a Context object
type ContextReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=context.kube-auth,resources=contexts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=context.kube-auth,resources=contexts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=context.kube-auth,resources=contexts/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

func (r *ContextReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch the NamespaceContext instance
	namespaceContext := &contextv1.Context{}
	err := r.Get(ctx, req.NamespacedName, namespaceContext)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.UpdateStatus(ctx, nil, nil, utils.BasicCondition{
				Type:    contextv1.TypeNotReady,
				Status:  contextv1.StatusFalse,
				Reason:  "Context not found",
				Message: "Context not found",
			}, nil)
		}
		return r.UpdateStatus(ctx, nil, nil, utils.BasicCondition{
			Type:    contextv1.TypeNotReady,
			Status:  contextv1.StatusFalse,
			Reason:  "Error fetching context",
			Message: err.Error(),
		}, err)
	}

	// List all namespaces
	namespaceList := &corev1.NamespaceList{}
	err = r.List(ctx, namespaceList)
	if err != nil {
		return r.UpdateStatus(ctx, namespaceContext, nil, utils.BasicCondition{
			Type:    contextv1.TypeNotReady,
			Status:  contextv1.StatusFalse,
			Reason:  "Error listing namespaces",
			Message: err.Error(),
		}, err)
	}

	var matchedNamespaces []string

	if err := utils.CheckNamespaces(namespaceList.Items, namespaceContext.Spec.Namespaces, &matchedNamespaces); err != nil {
		return r.UpdateStatus(ctx, namespaceContext, nil, utils.BasicCondition{
			Type:    contextv1.TypeNotReady,
			Status:  contextv1.StatusFalse,
			Reason:  "Error checking namespaces",
			Message: err.Error(),
		}, err)
	}

	if err := utils.FindNamespaces(namespaceList.Items, namespaceContext.Spec.Find, &matchedNamespaces); err != nil {
		return r.UpdateStatus(ctx, namespaceContext, nil, utils.BasicCondition{
			Type:    contextv1.TypeNotReady,
			Status:  contextv1.StatusFalse,
			Reason:  "Error finding namespaces",
			Message: err.Error(),
		}, err)
	}

	if len(matchedNamespaces) == 0 {
		return r.UpdateStatus(ctx, namespaceContext, nil, utils.BasicCondition{
			Type:    contextv1.TypeNotReady,
			Status:  contextv1.StatusFalse,
			Reason:  "No namespaces found",
			Message: "No namespaces found",
		}, nil)
	}
	return r.UpdateStatus(ctx, namespaceContext, matchedNamespaces, utils.BasicCondition{
		Type:    contextv1.TypeReady,
		Status:  contextv1.StatusTrue,
		Reason:  "Context synced",
		Message: "Context synced",
	}, nil)
}

func (r *ContextReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// Watch for changes on namespaces creation or deletion
	return ctrl.NewControllerManagedBy(mgr).
		For(&contextv1.Context{}).
		Watches(&corev1.Namespace{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, object client.Object) []reconcile.Request {
				// List all Context resources and create a request for each
				contextList := &contextv1.ContextList{}
				if err := mgr.GetClient().List(ctx, contextList); err != nil {
					log.Log.Error(err, "Failed to list Context resources")
					return nil
				}

				var requests []reconcile.Request
				for _, context := range contextList.Items {
					requests = append(requests, reconcile.Request{
						NamespacedName: client.ObjectKey{
							Name:      context.Name,
							Namespace: context.Namespace,
						},
					})
				}
				return requests
			},
		)).
		WithEventFilter(utils.FilterFuncs([]string{"*v1.Namespace"})).
		Complete(r)
}

func (r *ContextReconciler) UpdateStatus(ctx context.Context, context *contextv1.Context, namespaces []string, condition utils.BasicCondition, Error error) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if context == nil {
		return utils.HandleError(logger, Error, condition.Message)
	}

	newConditions := utils.SyncConditions(context.Status.Conditions, condition)
	if reflect.DeepEqual(context.Status.Conditions, newConditions) && utils.NamespacesEqual(namespaces, context.Status.SyncedNamespaces) {
		return ctrl.Result{}, nil
	}
	context.Status.SyncedNamespaces = namespaces
	context.Status.Conditions = newConditions
	context.Status.ObservedGeneration = context.Status.ObservedGeneration + 1

	updateErr := r.Status().Update(ctx, context)
	if updateErr != nil {
		if errors.IsConflict(updateErr) {
			logger.Info("Conflict while updating status, retrying")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, updateErr
	}
	err := r.Status().Update(ctx, context)
	if err != nil {
		return ctrl.Result{}, err
	}
	return utils.HandleError(logger, Error, condition.Message)
}
