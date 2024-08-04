package utils

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func FilterFuncs() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc:  createFilter,
		DeleteFunc:  deleteFilter,
		UpdateFunc:  updateFilter,
		GenericFunc: genericFilter,
	}
}

func createFilter(e event.TypedCreateEvent[client.Object]) bool {
	return e.Object.GetGeneration() == 1
}

func deleteFilter(e event.TypedDeleteEvent[client.Object]) bool {
	return false
}

func updateFilter(e event.UpdateEvent) bool {
	return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
}

func genericFilter(e event.GenericEvent) bool {
	return false
}
