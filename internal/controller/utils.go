package controller

import (
	errorsHandler "errors"
	corev1 "k8s.io/api/core/v1"
	"regexp"
)

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
