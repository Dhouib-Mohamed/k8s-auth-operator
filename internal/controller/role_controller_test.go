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
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	contextv1 "kube-auth.io/api/v1"
	userController "kube-auth.io/internal/controller/context"
	controller "kube-auth.io/internal/controller/role"
	"kube-auth.io/internal/controller/utils"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Role Controller", func() {
	Context("When reconciling a resource", func() {

		const resourceName = "test-resource"
		const namespaceName = "default"

		ctx := context.Background()
		typeNamespacedName := types.NamespacedName{Name: resourceName, Namespace: namespaceName}
		role := &contextv1.Role{}

		var controllerReconciler *controller.RoleReconciler
		var contextReconciler *userController.ContextReconciler

		BeforeEach(func() {
			controllerReconciler = &controller.RoleReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			contextReconciler = &userController.ContextReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("creating the custom resource for the Kind Role")
			err := k8sClient.Get(ctx, typeNamespacedName, role)
			if err != nil && errors.IsNotFound(err) {
				resource := &contextv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: namespaceName,
					},
					Spec: contextv1.RoleSpec{
						NamespaceRole: contextv1.NamespaceRole{
							Create: &[]bool{true}[0],
							Delete: &[]bool{true}[0],
						},
						ClusterRole: []contextv1.ClusterRole{
							{
								Contexts:   []string{},
								Namespaces: []string{namespaceName},
								Resources:  []string{"pods"},
								Instances:  []contextv1.ResourceInstances{},
								Verbs:      []string{"get", "list"},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &contextv1.Role{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, resource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			By("Verifying the resource status")
			Expect(k8sClient.Get(ctx, typeNamespacedName, role)).To(Succeed())
			Expect(role.Status.Conditions).NotTo(BeEmpty())
			Expect(role.Status.HandledNamespaces).NotTo(BeEmpty())
			Expect(role.Status.Conditions[0].Type).To(Equal(contextv1.TypeReady))
			Expect(role.Status.Conditions[0].Status).To(Equal(contextv1.StatusTrue))
			Expect(role.Status.HandledNamespaces).To(ContainElement(contextv1.HandledNamespace{
				Namespace: namespaceName,
				Roles: []rbacv1.PolicyRule{
					{
						Verbs:     []string{"get", "list"},
						APIGroups: []string{"*"},
						Resources: []string{"pods"},
					},
				},
			}))

			By("Checking if a RBAC Role resource is created")
			roleResource := &rbacv1.Role{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      utils.GetRoleName(resourceName, namespaceName),
				Namespace: namespaceName,
			}, roleResource)).To(Succeed())
			Expect(roleResource.Rules).To(ContainElement(rbacv1.PolicyRule{
				Verbs:     []string{"get", "list"},
				APIGroups: []string{"*"},
				Resources: []string{"pods"},
			}))

			By("Checking Cluster Role Existence")
			clusterRoleResource := &rbacv1.ClusterRole{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name: utils.GetRoleName(resourceName, ""),
			}, clusterRoleResource)).To(Succeed())
		})

		It("should handle a role with a not found context", func() {
			Expect(k8sClient.Get(ctx, typeNamespacedName, role)).To(Succeed())
			By("Setting a not found context")
			role.Spec.ClusterRole[0].Contexts = []string{"not-found"}
			Expect(k8sClient.Update(ctx, role)).To(Succeed())

			By("Reconciling the created resource")
			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())

			By("Verifying the resource status")
			Expect(k8sClient.Get(ctx, typeNamespacedName, role)).To(Succeed())
			Expect(role.Status.Conditions).NotTo(BeEmpty())
			Expect(role.Status.HandledNamespaces).To(BeEmpty())
			Expect(role.Status.Conditions[0].Type).To(Equal(contextv1.TypeNotReady))
			Expect(role.Status.Conditions[0].Status).To(Equal(contextv1.StatusFalse))
			Expect(role.Status.Conditions[0].Reason).To(Equal("Error extracting handled namespaces"))
		})

		It("should reconcile when updating a linked context", func() {
			By("Creating a new context")
			newContext := &contextv1.Context{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-context",
					Namespace: namespaceName,
				},
				Spec: contextv1.ContextSpec{
					Namespaces: []string{"kube-system"},
					Find:       []string{},
				},
			}
			Expect(k8sClient.Create(ctx, newContext)).To(Succeed())
			_, err := contextReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "test-context",
			}})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-context",
				Namespace: namespaceName,
			}, newContext))

			By("Linking the context to the role")
			Expect(k8sClient.Get(ctx, typeNamespacedName, role)).To(Succeed())
			role.Spec.ClusterRole[0].Contexts = []string{"test-context"}
			Expect(k8sClient.Update(ctx, role)).To(Succeed())

			By("Reconciling the created resource")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, typeNamespacedName, role)).To(Succeed())

			Expect(role.Status.HandledNamespaces).To(ContainElement(contextv1.HandledNamespace{
				Namespace: "kube-system",
				Roles: []rbacv1.PolicyRule{
					{
						Verbs:     []string{"get", "list"},
						APIGroups: []string{"*"},
						Resources: []string{"pods"},
					},
				},
			}))

			By("Updating the context")
			newContext.Spec.Namespaces = []string{"kube-public"}
			Expect(k8sClient.Update(ctx, newContext)).To(Succeed())
			_, err = contextReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "test-context",
			}})
			Expect(k8sClient.Get(ctx, typeNamespacedName, role)).To(Succeed())
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			log.Log.Info("Role", "role", role)
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-context",
				Namespace: namespaceName,
			}, newContext))
			log.Log.Info("Context", "context", newContext)

		})
	})
})
