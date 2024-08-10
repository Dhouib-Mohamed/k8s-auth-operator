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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	contextv1 "kube-auth.io/api/v1"
	controller "kube-auth.io/internal/controller/context"
)

var _ = Describe("Context Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName    = "test-resource"
			namespaceName   = "default"
			testNonExistent = "test-nonexistent-namespace"
			testNoNamespace = "test-no-namespaces"
			kubeTest        = "kube-test"
			deleteNamespace = "delete-namespace"
		)

		var (
			ctx                  context.Context
			typeNamespacedName   types.NamespacedName
			testContext          *contextv1.Context
			controllerReconciler *controller.ContextReconciler
		)

		BeforeEach(func() {
			ctx = context.Background()
			typeNamespacedName = types.NamespacedName{Name: resourceName, Namespace: namespaceName}
			testContext = &contextv1.Context{}
			controllerReconciler = &controller.ContextReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("creating the custom resource for the Kind Context")
			err := k8sClient.Get(ctx, typeNamespacedName, testContext)
			if err != nil && errors.IsNotFound(err) {
				resource := &contextv1.Context{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: namespaceName,
					},
					Spec: contextv1.ContextSpec{
						Namespaces: []string{namespaceName},
						Find:       []string{"kube-*"},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &contextv1.Context{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("cleaning up the specific resource instance Context")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			By("Verifying the resource status")
			err = k8sClient.Get(ctx, typeNamespacedName, testContext)
			Expect(err).NotTo(HaveOccurred())
			Expect(testContext.Status.Conditions).NotTo(BeEmpty())
			Expect(testContext.Status.SyncedNamespaces).To(ContainElement(namespaceName))
			Expect(testContext.Status.Conditions[0].Type).To(Equal(contextv1.TypeReady))
			Expect(testContext.Status.Conditions[0].Status).To(Equal(contextv1.StatusTrue))
		})

		It("should reconcile when adding a namespace", func() {
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: kubeTest},
			}

			By("creating a new namespace")
			Expect(k8sClient.Create(ctx, namespace)).To(Succeed())

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the resource status after adding namespace")
			err = k8sClient.Get(ctx, typeNamespacedName, testContext)
			Expect(err).NotTo(HaveOccurred())
			Expect(testContext.Status.SyncedNamespaces).To(ContainElement(kubeTest))

			By("Cleaning up the new namespace")
			Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
		})

		It("should reconcile when deleting a namespace", func() {
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: deleteNamespace},
			}

			By("creating a namespace to delete")
			Expect(k8sClient.Create(ctx, namespace)).To(Succeed())

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Deleting the namespace")
			Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the resource status after deleting namespace")
			err = k8sClient.Get(ctx, typeNamespacedName, testContext)
			Expect(err).NotTo(HaveOccurred())
			Expect(testContext.Status.SyncedNamespaces).NotTo(ContainElement(deleteNamespace))
		})

		It("should handle a context with a non-existent namespace", func() {
			resource := &contextv1.Context{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testNonExistent,
					Namespace: namespaceName,
				},
				Spec: contextv1.ContextSpec{
					Namespaces: []string{"not-found"},
					Find:       []string{},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      testNonExistent,
					Namespace: namespaceName,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the resource status")
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      testNonExistent,
				Namespace: namespaceName,
			}, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.Conditions).NotTo(BeEmpty())
			Expect(resource.Status.SyncedNamespaces).To(BeEmpty())
			Expect(resource.Status.Conditions[0].Type).To(Equal(contextv1.TypeNotReady))
			Expect(resource.Status.Conditions[0].Status).To(Equal(contextv1.StatusFalse))
			Expect(resource.Status.Conditions[0].Reason).To(Equal("Error checking namespaces"))

			By("Cleaning up the created resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should handle a context with no namespaces", func() {
			resource := &contextv1.Context{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testNoNamespace,
					Namespace: namespaceName,
				},
				Spec: contextv1.ContextSpec{
					Namespaces: []string{},
					Find:       []string{"hello*"},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      testNoNamespace,
					Namespace: namespaceName,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("Verifying the resource status")
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      testNoNamespace,
				Namespace: namespaceName,
			}, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.Conditions).NotTo(BeEmpty())
			Expect(resource.Status.SyncedNamespaces).To(BeEmpty())
			Expect(resource.Status.Conditions[0].Type).To(Equal(contextv1.TypeNotReady))
			Expect(resource.Status.Conditions[0].Status).To(Equal(contextv1.StatusFalse))
			Expect(resource.Status.Conditions[0].Reason).To(Equal("No namespaces found"))

			By("Cleaning up the created resource")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
	})
})
