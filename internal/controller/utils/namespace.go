package utils

import (
	errorsHandler "errors"
	corev1 "k8s.io/api/core/v1"
	"regexp"
)

func CheckNamespaces(AllNamespaces []corev1.Namespace, namespaceList []string, matchedNamespaces *[]string) error {
	for _, ns := range namespaceList {
		found := false
		for _, allNs := range AllNamespaces {
			if ns == allNs.Name {
				found = true
				AppendNamespace(matchedNamespaces, ns)
				break
			}
		}
		if !found {
			return errorsHandler.New("Namespace not found")
		}
	}
	return nil
}

func FindNamespaces(allNamespaces []corev1.Namespace, namespaceFind []string, matchedNamespaces *[]string) error {
	for _, find := range namespaceFind {
		regex, err := regexp.Compile(find)
		if err != nil {
			return err
		}
		for _, ns := range allNamespaces {
			if regex.MatchString(ns.Name) {
				AppendNamespace(matchedNamespaces, ns.Name)
			}
		}
	}
	return nil
}

func AppendNamespace(matchedNamespaces *[]string, namespace string) {
	for _, ns := range *matchedNamespaces {
		if ns == namespace {
			return
		}
	}
	*matchedNamespaces = append(*matchedNamespaces, namespace)
}

func NamespacesEqual(a, b []string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	for _, ns := range a {
		found := false
		for _, ns2 := range b {
			if ns == ns2 {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	for _, ns := range b {
		found := false
		for _, ns2 := range a {
			if ns == ns2 {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
