package utils

import (
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	contextv1 "kube-auth.io/api/v1"
	v1 "kube-auth.io/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"slices"
)

type BasicCondition struct {
	Type    v1.TypeEnum   `json:"type"`
	Status  v1.StatusEnum `json:"status"`
	Reason  string        `json:"reason"`
	Message string        `json:"message"`
}

func compareConditions(newCondition BasicCondition, oldCondition v1.ContextCondition) bool {
	return newCondition.Type == oldCondition.Type && newCondition.Status == oldCondition.Status && newCondition.Reason == oldCondition.Reason && newCondition.Message == oldCondition.Message
}

func incrementConditions(conditions []v1.ContextCondition, newCondition BasicCondition) []v1.ContextCondition {
	newConditions := conditions
	if newConditions == nil {
		newConditions = []contextv1.ContextCondition{}
	}

	newConditions = slices.Insert(newConditions, 0,
		contextv1.ContextCondition{
			LastTransitionTime: metav1.Now(),
			LastUpdateTime:     metav1.Now(),
			Type:               newCondition.Type,
			Status:             newCondition.Status,
			Reason:             newCondition.Reason,
			Message:            newCondition.Message,
		})
	return newConditions
}

func HandleError(logger logr.Logger, err error, message string) (ctrl.Result, error) {
	if err != nil {
		logger.Error(err, message)
		return ctrl.Result{Requeue: false}, nil
	}
	return ctrl.Result{}, nil
}

func SyncConditions(conditions []v1.ContextCondition, newCondition BasicCondition) []v1.ContextCondition {
	newConditions := conditions
	if newConditions != nil && compareConditions(newCondition, newConditions[0]) {
		newConditions[0].LastUpdateTime = metav1.Now()
	} else {
		newConditions = incrementConditions(newConditions, newCondition)
	}

	return newConditions
}
