package utils

import (
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func FilterFuncs(specialObjects []string) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc:  createFilter(specialObjects),
		DeleteFunc:  deleteFilter(specialObjects),
		UpdateFunc:  updateFilter(specialObjects),
		GenericFunc: genericFilter(specialObjects),
	}
}

func createFilter(specialObjects []string) func(e event.CreateEvent) bool {
	return func(e event.TypedCreateEvent[client.Object]) bool {
		if e.Object.GetGeneration() == 0 {
			for _, specialObject := range specialObjects {
				if reflect.TypeOf(e.Object).String() == specialObject {
					return true
				}
			}
		}
		return e.Object.GetGeneration() == 1
	}
}

func deleteFilter(specialObjects []string) func(e event.DeleteEvent) bool {
	return func(e event.TypedDeleteEvent[client.Object]) bool {
		return !e.DeleteStateUnknown
	}
}

func getStatus(obj client.Object) interface{} {
	val := reflect.ValueOf(obj).Elem()
	statusField := val.FieldByName("Status")
	if !statusField.IsValid() {
		return nil
	}
	return statusField.Interface()
}

func statusChanged(oldObj, newObj client.Object) bool {
	oldStatus := getStatus(oldObj)
	newStatus := getStatus(newObj)
	return !reflect.DeepEqual(oldStatus, newStatus)
}

func updateFilter(specialObjects []string) func(e event.UpdateEvent) bool {
	return func(e event.TypedUpdateEvent[client.Object]) bool {
		if e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() {
			return true
		}
		if e.ObjectOld.GetGeneration() == 0 && e.ObjectNew.GetGeneration() == 0 {
			for _, specialObject := range specialObjects {
				if reflect.TypeOf(e.ObjectNew).String() == specialObject {
					return true
				}
			}
		}
		for _, specialObject := range specialObjects {
			if reflect.TypeOf(e.ObjectNew).String() == specialObject {
				return statusChanged(e.ObjectOld, e.ObjectNew)
			}
		}
		return false
	}
}

func genericFilter(specialObjects []string) func(e event.GenericEvent) bool {
	return func(e event.GenericEvent) bool {
		return false
	}
}
