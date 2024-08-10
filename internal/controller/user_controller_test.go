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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	controller "kube-auth.io/internal/controller/user"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	contextv1 "kube-auth.io/api/v1"
)

var _ = Describe("User Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"
		const roleName = "test-role"
		const namespace = "default"

		var (
			ctx                = context.Background()
			typeNamespacedName = types.NamespacedName{Name: resourceName, Namespace: namespace}
			user               = &contextv1.User{}
		)

		createRole := func() {
			role := &contextv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleName,
					Namespace: namespace,
				},
				Spec: contextv1.RoleSpec{
					NamespaceRole: contextv1.NamespaceRole{
						Create: &[]bool{true}[0],
						Delete: &[]bool{true}[0],
					},
					ClusterRole: []contextv1.ClusterRole{
						{
							Namespaces: []string{namespace},
							Resources:  []string{"pods"},
							Verbs:      []string{"get", "list"},
							Contexts:   []string{},
							Instances:  []contextv1.ResourceInstances{},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, role)).To(Succeed())
		}

		createUser := func() {
			err := k8sClient.Get(ctx, typeNamespacedName, user)
			if err != nil && errors.IsNotFound(err) {
				user := &contextv1.User{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: namespace,
					},
					Spec: contextv1.UserSpec{
						Roles: []string{roleName},
					},
				}
				Expect(k8sClient.Create(ctx, user)).To(Succeed())
			}
		}

		deleteUser := func() {
			err := k8sClient.Get(ctx, typeNamespacedName, user)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, user)).To(Succeed())
		}

		deleteRole := func() {
			role := &contextv1.Role{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: roleName, Namespace: namespace}, role)
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Delete(ctx, role)).To(Succeed())
		}

		reconcileUser := func() {
			reconciler := &controller.UserReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
		}

		verifyUserStatus := func(expectedCondition contextv1.StatusEnum, kubeConfigExistent bool) {
			Expect(k8sClient.Get(ctx, typeNamespacedName, user)).To(Succeed())
			Expect(user.Status.Conditions).To(Not(BeEmpty()))
			Expect(user.Status.Conditions[0].Status).To(Equal(expectedCondition))
			if kubeConfigExistent {
				Expect(user.Status.KubeConfig).To(Not(Equal("")))
			} else {
				Expect(user.Status.KubeConfig).To(Equal(""))
			}
		}

		//verifyRoleBinding := func(roleBindingName, roleBindingNamespace string) {
		//	roleBindingList := &rbacv1.RoleBindingList{}
		//	Expect(k8sClient.List(ctx, roleBindingList)).To(Succeed())
		//	for _, roleBinding := range roleBindingList.Items {
		//		log.Log.Info("RoleBinding", "Name", roleBinding.Name, "Namespace", roleBinding.Namespace)
		//		if roleBinding.Name == roleBindingName && roleBinding.Namespace == roleBindingNamespace {
		//			log.Log.Info("RoleBinding found", "Name", roleBinding.Name, "Namespace", roleBinding.Namespace)
		//		}
		//	}
		//	roleBinding := &rbacv1.RoleBinding{}
		//	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: roleBindingName, Namespace: roleBindingNamespace}, roleBinding)).To(Succeed())
		//}

		BeforeEach(func() {
			createRole()
			createUser()
		})

		AfterEach(func() {
			deleteUser()
			deleteRole()
		})

		It("should successfully reconcile the resource", func() {
			reconcileUser()
			verifyUserStatus(contextv1.StatusTrue, true)
			//verifyRoleBinding(utils.GetRoleBindingName(roleName, ""), namespace)
			//verifyRoleBinding(utils.GetRoleBindingName(roleName, namespace), namespace)
		})

		It("should handle a wrong role", func() {
			reconcileUser()
			Expect(k8sClient.Get(ctx, typeNamespacedName, user)).To(Succeed())
			user.Spec.Roles = append(user.Spec.Roles, "wrong-role")
			Expect(k8sClient.Update(ctx, user)).To(Succeed())
			reconcileUser()
			verifyUserStatus(contextv1.StatusFalse, false)
		})

		It("should reconcile when changing a role", func() {
			role := &contextv1.Role{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: roleName, Namespace: namespace}, role)).To(Succeed())
			role.Spec.ClusterRole[0].Namespaces = []string{"kube-system"}
			Expect(k8sClient.Update(ctx, role)).To(Succeed())
			reconcileUser()
			verifyUserStatus(contextv1.StatusTrue, true)
			//verifyRoleBinding(utils.GetRoleName(roleName, "kube-system"), "kube-system")
		})
	})
})
