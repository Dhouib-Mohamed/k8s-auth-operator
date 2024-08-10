package user

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kube-auth.io/internal/controller/utils"
	"math/big"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

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
	roleBindingName := utils.GetRoleBindingName(roleName, namespace)
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
						Name:     utils.GetRoleName(roleName, ""),
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
						Name:     utils.GetRoleName(roleName, namespace),
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
				Name:     utils.GetRoleName(roleName, ""),
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
				Name:     utils.GetRoleName(roleName, namespace),
				APIGroup: "rbac.authorization.k8s.io",
			},
		}
	}
	return r.Update(context.TODO(), roleBinding)
}
