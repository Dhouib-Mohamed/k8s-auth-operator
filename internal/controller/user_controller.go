package controller

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/big"
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

// +kubebuilder:rbac:groups=context.kube-auth,resources=users,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=context.kube-auth,resources=users/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=context.kube-auth,resources=users/finalizers,verbs=update

func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	user := &contextv1.User{}
	err := r.Get(ctx, req.NamespacedName, user)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("User resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get User")
		return ctrl.Result{}, err
	}

	var roles []*contextv1.Role
	for _, role := range user.Spec.Roles {
		fetchedRole := &contextv1.Role{}
		err := r.Get(ctx, client.ObjectKey{
			Namespace: req.Namespace,
			Name:      role,
		}, fetchedRole)
		if err != nil {
			logger.Error(err, "Failed to get Role", "role", role)
			return ctrl.Result{}, err
		}
		roles = append(roles, fetchedRole)
	}

	err = r.createOrUpdateClusterRoleBinding(user, roles)
	if err != nil {
		logger.Error(err, "Failed to create or update ClusterRoleBinding")
		return ctrl.Result{}, err
	}

	if user.Status.KubeConfig == "" {
		certPEM, keyPEM, err := r.createSelfSignedCert(user.Name)
		if err != nil {
			logger.Error(err, "Failed to create self-signed certificate")
			return ctrl.Result{}, err
		}

		kubeConfig, err := r.createKubeConfig(user.Name, certPEM, keyPEM)
		if err != nil {
			logger.Error(err, "Failed to create kubeconfig")
			return ctrl.Result{}, err
		}

		user.Status.KubeConfig = kubeConfig
		err = r.Status().Update(ctx, user)
		if err != nil {
			logger.Error(err, "Failed to update User status")
			return ctrl.Result{}, err
		}
	}

	logger.Info("Successfully reconciled User", "namespace", req.Namespace, "name", req.Name)
	return ctrl.Result{}, nil
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

func (r *UserReconciler) createOrUpdateClusterRoleBinding(user *contextv1.User, roles []*contextv1.Role) error {
	roleBinding := &rbacv1.ClusterRoleBinding{}
	err := r.Get(context.TODO(), client.ObjectKey{
		Name: user.Name + "-binding",
	}, roleBinding)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if apierrors.IsNotFound(err) {
		roleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: user.Name + "-binding",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:     rbacv1.UserKind,
					Name:     user.Name,
					APIGroup: rbacv1.GroupName,
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "ClusterRole",
				Name: "cluster-admin", // or use a specific role based on your requirement
			},
		}
		return r.Create(context.TODO(), roleBinding)
	}

	roleBinding.Subjects = []rbacv1.Subject{
		{
			Kind:     rbacv1.UserKind,
			Name:     user.Name,
			APIGroup: rbacv1.GroupName,
		},
	}
	roleBinding.RoleRef = rbacv1.RoleRef{
		Kind: "ClusterRole",
		Name: "cluster-admin", // or use a specific role based on your requirement
	}
	return r.Update(context.TODO(), roleBinding)
}

func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&contextv1.User{}).
		Complete(r)
}
