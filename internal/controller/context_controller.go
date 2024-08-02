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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	contextv1 "kube-auth.io/api/v1"
	"kube-auth.io/internal/controller/utils"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"slices"
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
	logger := log.FromContext(ctx)

	// Fetch the NamespaceContext instance
	namespaceContext := &contextv1.Context{}
	err := r.Get(ctx, req.NamespacedName, namespaceContext)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.UpdateStatus(ctx, nil, nil, contextv1.TypeNotReady, contextv1.StatusFalse, "Context not found", "Context not found", nil)
		}
		return r.UpdateStatus(ctx, nil, nil, contextv1.TypeNotReady, contextv1.StatusFalse, "Error fetching context", err.Error(), err)
	}

	// List all namespaces
	namespaceList := &corev1.NamespaceList{}
	err = r.List(ctx, namespaceList)
	if err != nil {
		return r.UpdateStatus(ctx, namespaceContext, nil, contextv1.TypeNotReady, contextv1.StatusFalse, "Error listing namespaces", err.Error(), err)
	}

	matchedNamespaces := []string{}

	if err := utils.CheckNamespaces(namespaceList.Items, namespaceContext.Spec.Namespaces, &matchedNamespaces); err != nil {
		//logger.Error(err, "Namespace not found")
		return r.UpdateStatus(ctx, namespaceContext, nil, contextv1.TypeNotReady, contextv1.StatusFalse, "Namespace not found", err.Error(), err)
	}

	if err := utils.FindNamespaces(namespaceList.Items, namespaceContext.Spec.Find, &matchedNamespaces); err != nil {
		logger.Error(err, "Error finding namespaces")
		return r.UpdateStatus(ctx, namespaceContext, nil, contextv1.TypeNotReady, contextv1.StatusFalse, "Error finding namespaces", err.Error(), err)
	}

	if len(matchedNamespaces) == 0 {
		logger.Info("No namespaces found")
		return r.UpdateStatus(ctx, namespaceContext, nil, contextv1.TypeNotReady, contextv1.StatusFalse, "No namespaces found", "No namespaces found", nil)
	}

	return r.UpdateStatus(ctx, namespaceContext, matchedNamespaces, contextv1.TypeReady, contextv1.StatusTrue, "Context synced", "Context synced", nil)
}

func (r *ContextReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// Watch for changes on namespaces creation or deletion
	return ctrl.NewControllerManagedBy(mgr).
		For(&contextv1.Context{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
				return e.Object.GetGeneration() == 1
			},
			DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		}).
		//Watches(&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

func (r *ContextReconciler) UpdateStatus(ctx context.Context, context *contextv1.Context, namespaces []string, status_type contextv1.TypeEnum, status contextv1.StatusEnum, reason string, message string, Error error) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if context == nil {
		if Error != nil {
			logger.Error(Error, message)
			return ctrl.Result{Requeue: false}, nil
		}
		return ctrl.Result{}, nil
	}
	context.Status.ObservedGeneration = context.Status.ObservedGeneration + 1
	logger.Info("Updating context status", "namespaces", namespaces, "syncedNamespaces", context.Status.SyncedNamespaces, "result", utils.NamespacesEqual(namespaces, context.Status.SyncedNamespaces))
	if context.Status.Conditions != nil && utils.NamespacesEqual(namespaces, context.Status.SyncedNamespaces) {
		logger.Info("Context already synced")
		context.Status.Conditions[0].LastUpdateTime = metav1.Now()
	} else {
		if context.Status.Conditions == nil {
			context.Status.Conditions = []contextv1.ContextCondition{}
		}
		if status == contextv1.StatusFalse {
			context.Status.SyncedNamespaces = []string{}
		} else {
			context.Status.SyncedNamespaces = namespaces
		}
		context.Status.Conditions = slices.Insert(context.Status.Conditions, 0,
			contextv1.ContextCondition{
				Type:               status_type,
				Status:             status,
				LastTransitionTime: metav1.Now(),
				LastUpdateTime:     metav1.Now(),
				Reason:             reason,
				Message:            message,
			})
	}
	logger.Info("Updating context status", "context", context)
	err := r.Status().Update(ctx, context)
	if err != nil {
		return ctrl.Result{}, err
	}
	if Error != nil {
		logger.Error(Error, message)
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}
