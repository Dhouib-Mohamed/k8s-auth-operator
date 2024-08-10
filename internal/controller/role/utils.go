package role

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	contextv1 "kube-auth.io/api/v1"
	"kube-auth.io/internal/controller/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *RoleReconciler) extractHandledNamespaces(ctx context.Context, roles []contextv1.ClusterRole, namespace string) ([]contextv1.HandledNamespace, error) {
	logger := log.FromContext(ctx)
	var handledNamespaces []contextv1.HandledNamespace
	for _, role := range roles {
		namespaces := []string{}
		for _, contextName := range role.Contexts {
			context := &contextv1.Context{}
			err := r.Get(ctx, types.NamespacedName{Name: contextName, Namespace: namespace}, context)
			if err != nil {
				return nil, err
			}
			for _, ns := range context.Status.SyncedNamespaces {
				utils.AppendNamespace(&namespaces, ns)
			}
		}
		for _, ns := range role.Namespaces {
			utils.AppendNamespace(&namespaces, ns)
		}
		var policies []rbacv1.PolicyRule
		if role.Resources != nil && len(role.Resources) > 0 {
			policies = append(policies, rbacv1.PolicyRule{
				Verbs:     role.Verbs,
				APIGroups: []string{"*"},
				Resources: role.Resources,
			})
		}
		if role.Instances != nil {
			for _, instance := range role.Instances {
				policies = append(policies, rbacv1.PolicyRule{
					Verbs:         role.Verbs,
					APIGroups:     []string{"*"},
					Resources:     []string{instance.Kind},
					ResourceNames: instance.Name,
				})
			}
		}

		if len(policies) == 0 {
			logger.Info("No policies found for role", "role", role)
			continue
		}
		for _, ns := range namespaces {

			found := false
			for _, handledNamespace := range handledNamespaces {
				if handledNamespace.Namespace == ns {
					handledNamespace.Roles = append(handledNamespace.Roles, policies...)
					found = true
					break
				}
			}
			if !found {
				handledNamespaces = append(handledNamespaces, contextv1.HandledNamespace{
					Namespace: ns,
					Roles:     policies,
				})
			}
		}
	}
	return handledNamespaces, nil
}

func (r *RoleReconciler) createNamespacesRole(name string, nsRole contextv1.NamespaceRole) error {
	rule := rbacv1.PolicyRule{
		Verbs:     []string{"get", "list", "watch"},
		APIGroups: []string{""},
		Resources: []string{"namespaces"},
	}
	if nsRole.Create == nil && nsRole.Delete == nil {
		rule.Verbs = append(rule.Verbs, "create", "delete")
	}
	if *nsRole.Create {
		rule.Verbs = append(rule.Verbs, "create")
	}
	if *nsRole.Delete {
		rule.Verbs = append(rule.Verbs, "delete")
	}
	return r.createOrUpdateRole(name, "", []rbacv1.PolicyRule{rule})
}

func (r *RoleReconciler) createOrUpdateRole(roleName string, namespace string, rules []rbacv1.PolicyRule) error {
	var role client.Object
	roleFullName := utils.GetRoleName(roleName, namespace)
	if namespace == "" {
		role = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: roleFullName,
			},
			Rules: rules,
		}
	} else {
		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      roleFullName,
				Namespace: namespace,
			},
			Rules: rules,
		}
	}
	err := r.Create(context.Background(), role)
	if err != nil {
		if errors.IsAlreadyExists(err) {
			err = r.Update(context.Background(), role)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

func (r *RoleReconciler) deleteRole(roleName string, namespace string) error {
	var role client.Object
	fullRoleName := utils.GetRoleName(roleName, namespace)
	if namespace == "" {
		role = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: fullRoleName,
			},
		}
	} else {
		namespaceObj := &corev1.Namespace{}
		err := r.Get(context.Background(), types.NamespacedName{Name: namespace}, namespaceObj)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if namespaceObj.Status.Phase == corev1.NamespaceTerminating {
			return nil
		}
		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fullRoleName,
				Namespace: namespace,
			},
		}
	}
	err := r.Delete(context.Background(), role)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}
