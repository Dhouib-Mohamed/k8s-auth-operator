package utils

import v1 "kube-auth.io/api/v1"

type BasicCondition struct {
	Type    v1.TypeEnum   `json:"type"`
	Status  v1.StatusEnum `json:"status"`
	Reason  string        `json:"reason"`
	Message string        `json:"message"`
}

func CompareConditions(newCondition BasicCondition, oldCondition v1.ContextCondition) bool {
	return newCondition.Type == oldCondition.Type && newCondition.Status == oldCondition.Status && newCondition.Reason == oldCondition.Reason && newCondition.Message == oldCondition.Message
}
