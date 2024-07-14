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
	errorsHandler "errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	contextv1 "kube-auth.io/api/v1"
	"regexp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
	log := log.FromContext(ctx)

	// Fetch the NamespaceContext instance
	namespaceContext := &contextv1.Context{}
	err := r.Get(ctx, req.NamespacedName, namespaceContext)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// List all namespaces
	namespaceList := &corev1.NamespaceList{}
	err = r.List(ctx, namespaceList)
	if err != nil {
		return ctrl.Result{}, err
	}

	matchedNamespaces := []string{}

	if err := checkNamespaces(namespaceList.Items, namespaceContext.Spec.Namespaces, &matchedNamespaces); err != nil {
		log.Error(err, "Namespace not found")
		return ctrl.Result{}, nil
	}

	if err := findNamespaces(namespaceList.Items, namespaceContext.Spec.Find, &matchedNamespaces); err != nil {
		log.Error(err, "Error finding namespaces")
		return ctrl.Result{}, nil
	}

	if len(matchedNamespaces) == 0 {
		log.Info("No namespaces found")
		return ctrl.Result{}, nil
	}

	// Update the status with the matched namespaces
	namespaceContext.Status.SyncedNamespaces = matchedNamespaces
	err = r.Status().Update(ctx, namespaceContext)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ContextReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Watch for changes on namespaces creation or deletion
	return ctrl.NewControllerManagedBy(mgr).
		For(&contextv1.Context{}).
		//Watches(&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

func checkNamespaces(AllNamespaces []corev1.Namespace, namespaceList []string, matchedNamespaces *[]string) error {
	for _, ns := range namespaceList {
		found := false
		for _, allNs := range AllNamespaces {
			if ns == allNs.Name {
				found = true
				appendNamespace(matchedNamespaces, ns)
				break
			}
		}
		if !found {
			return errorsHandler.New("Namespace not found")
		}
	}
	return nil
}

func findNamespaces(allNamespaces []corev1.Namespace, namespaceFind []string, matchedNamespaces *[]string) error {
	for _, find := range namespaceFind {
		regex, err := regexp.Compile(find)
		if err != nil {
			return err
		}
		for _, ns := range allNamespaces {
			if regex.MatchString(ns.Name) {
				appendNamespace(matchedNamespaces, ns.Name)
			}
		}
	}
	return nil
}

func appendNamespace(matchedNamespaces *[]string, namespace string) {
	for _, ns := range *matchedNamespaces {
		if ns == namespace {
			return
		}
	}
	*matchedNamespaces = append(*matchedNamespaces, namespace)
}
