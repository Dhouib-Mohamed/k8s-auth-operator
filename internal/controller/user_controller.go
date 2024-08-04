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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kube-auth.io/internal/controller/utils"
	"math/big"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	rbacv1 "k8s.io/api/rbac/v1"
	contextv1 "kube-auth.io/api/v1"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type RoleBinding struct {
	client.Object
	Subjects []rbacv1.Subject
}

// +kubebuilder:rbac:groups=context.kube-auth,resources=users,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=context.kube-auth,resources=users/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=context.kube-auth,resources=users/finalizers,verbs=update

func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	user := &contextv1.User{}
	err := r.Get(ctx, req.NamespacedName, user)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.UpdateStatus(ctx, nil, "", utils.BasicCondition{
				Type:    contextv1.TypeNotReady,
				Status:  contextv1.StatusFalse,
				Reason:  "User not found",
				Message: "User not found",
			}, nil)
		}
		return r.UpdateStatus(ctx, nil, "", utils.BasicCondition{
			Type:    contextv1.TypeNotReady,
			Status:  contextv1.StatusFalse,
			Reason:  "Error fetching user",
			Message: err.Error(),
		}, err)
	}

	var roles []*contextv1.Role
	for _, role := range user.Spec.Roles {
		fetchedRole := &contextv1.Role{}
		err := r.Get(ctx, client.ObjectKey{
			Namespace: req.Namespace,
			Name:      role,
		}, fetchedRole)
		if err != nil {
			return r.UpdateStatus(ctx, user, "", utils.BasicCondition{
				Type:    contextv1.TypeNotReady,
				Status:  contextv1.StatusFalse,
				Reason:  "Error fetching role",
				Message: err.Error(),
			}, err)
		}
		roles = append(roles, fetchedRole)
	}

	for _, role := range roles {
		err := r.linkUserToRoleBinding(role.Name+"-namespace-role", "", rbacv1.Subject{
			Kind:     "User",
			Name:     user.Name,
			APIGroup: "rbac.authorization.k8s.io",
		})
		if err != nil {
			return r.UpdateStatus(ctx, user, "", utils.BasicCondition{
				Type:    contextv1.TypeNotReady,
				Status:  contextv1.StatusFalse,
				Reason:  "Error creating or updating cluster role binding",
				Message: err.Error(),
			}, err)
		}
		for _, namespace := range role.Status.HandledNamespaces {
			err := r.linkUserToRoleBinding(role.Name+"-"+namespace.Namespace, namespace.Namespace, rbacv1.Subject{
				Kind:     "User",
				Name:     user.Name,
				APIGroup: "rbac.authorization.k8s.io",
			})
			if err != nil {
				return r.UpdateStatus(ctx, user, "", utils.BasicCondition{
					Type:    contextv1.TypeNotReady,
					Status:  contextv1.StatusFalse,
					Reason:  "Error creating or updating cluster role binding",
					Message: err.Error(),
				}, err)
			}
		}
	}

	kubeConfig := user.Status.KubeConfig
	if kubeConfig == "" {
		certPEM, keyPEM, err := r.createSelfSignedCert(user.Name)
		if err != nil {
			return r.UpdateStatus(ctx, user, "", utils.BasicCondition{
				Type:    contextv1.TypeNotReady,
				Status:  contextv1.StatusFalse,
				Reason:  "Failed to create certificate",
				Message: err.Error(),
			}, err)
		}

		kubeConfig, err = r.createKubeConfig(user.Name, certPEM, keyPEM)
		if err != nil {
			return r.UpdateStatus(ctx, user, "", utils.BasicCondition{
				Type:    contextv1.TypeNotReady,
				Status:  contextv1.StatusFalse,
				Reason:  "Failed to create kubeconfig",
				Message: err.Error(),
			}, err)
		}
	}
	return r.UpdateStatus(ctx, user, kubeConfig, utils.BasicCondition{
		Type:    contextv1.TypeReady,
		Status:  contextv1.StatusTrue,
		Reason:  "User synced",
		Message: "User synced",
	}, nil)
}

func (r *UserReconciler) createSelfSignedCert(userName string) ([]byte, []byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: userName,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
		},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	return certPEM, keyPEM, nil
}

func (r *UserReconciler) createKubeConfig(userName string, certPEM []byte, keyPEM []byte) (string, error) {
	kubeConfig := `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: <CA_CERT>
    server: <API_SERVER>
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: ` + userName + `
  name: ` + userName + `@kubernetes
current-context: ` + userName + `@kubernetes
kind: Config
preferences: {}
users:
- name: ` + userName + `
  user:
    client-certificate-data: ` + string(certPEM) + `
    client-key-data: ` + string(keyPEM)

	return kubeConfig, nil
}

func (r *UserReconciler) linkUserToRoleBinding(roleName string, namespace string, subject rbacv1.Subject) error {
	var roleBinding client.Object
	roleBindingName := roleName + "-binding"
	var err error
	if namespace == "" {
		roleBinding = &rbacv1.ClusterRoleBinding{}
		err = r.Get(context.TODO(), client.ObjectKey{
			Name: roleBindingName,
		}, roleBinding)
	} else {
		roleBinding = &rbacv1.RoleBinding{}
		err = r.Get(context.TODO(), client.ObjectKey{
			Namespace: namespace,
			Name:      roleBindingName,
		}, roleBinding)
	}
	if err != nil {
		if apierrors.IsNotFound(err) {
			if namespace == "" {
				roleBinding = &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: roleBindingName,
					},
					Subjects: []rbacv1.Subject{subject},
					RoleRef: rbacv1.RoleRef{
						Kind:     "ClusterRole",
						Name:     roleName,
						APIGroup: "rbac.authorization.k8s.io",
					},
				}
			} else {
				roleBinding = &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      roleBindingName,
					},
					Subjects: []rbacv1.Subject{subject},
					RoleRef: rbacv1.RoleRef{
						Kind:     "Role",
						Name:     roleName,
						APIGroup: "rbac.authorization.k8s.io",
					},
				}
			}
			err := r.Create(context.TODO(), roleBinding)
			if err != nil {
				return err
			}
			return nil
		}
		return err
	}
	if namespace == "" {
		roleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: roleBindingName,
			},
			Subjects: append(roleBinding.(*rbacv1.ClusterRoleBinding).Subjects, subject),
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				Name:     roleName,
				APIGroup: "rbac.authorization.k8s.io",
			},
		}
	} else {
		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      roleBindingName,
			},
			Subjects: append(roleBinding.(*rbacv1.RoleBinding).Subjects, subject),
			RoleRef: rbacv1.RoleRef{
				Kind:     "Role",
				Name:     roleName,
				APIGroup: "rbac.authorization.k8s.io",
			},
		}
	}
	return r.Update(context.TODO(), roleBinding)
}

func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&contextv1.User{}).
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
		Complete(r)
}

func (r *UserReconciler) UpdateStatus(ctx context.Context, user *contextv1.User, kubeConfig string, condition utils.BasicCondition, Error error) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if user == nil {
		return utils.HandleError(logger, Error, condition.Message)
	}
	user.Status.KubeConfig = kubeConfig
	user.Status.ObservedGeneration = user.Status.ObservedGeneration + 1
	user.Status.Conditions = utils.SyncConditions(user.Status.Conditions, condition)
	err := r.Status().Update(ctx, user)
	if err != nil {
		return ctrl.Result{}, err
	}
	return utils.HandleError(logger, Error, condition.Message)
}
