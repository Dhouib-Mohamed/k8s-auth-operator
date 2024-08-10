package utils

func GetRoleName(resource, namespace string) string {
	if namespace == "" {
		namespace = "cluster"
	}
	return resource + "-" + namespace + "-role"
}

func GetRoleBindingName(resource, namespace string) string {
	roleName := GetRoleName(resource, namespace)
	return GetRoleBindingNameFromRole(roleName)
}

func GetRoleBindingNameFromRole(roleName string) string {
	return roleName + "-binding"
}
