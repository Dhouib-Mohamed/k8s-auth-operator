package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ContextCondition struct {
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	LastUpdateTime     metav1.Time `json:"lastUpdateTime,omitempty"`
	Message            string      `json:"message,omitempty"`
	Reason             string      `json:"reason,omitempty"`
	Status             StatusEnum  `json:"status,omitempty"`
	Type               TypeEnum    `json:"type,omitempty"`
}

type StatusEnum string

const (
	StatusTrue    StatusEnum = "True"
	StatusFalse   StatusEnum = "False"
	StatusUnknown StatusEnum = "Unknown"
)

type TypeEnum string

const (
	TypeReady    TypeEnum = "Ready"
	TypeNotReady TypeEnum = "NotReady"
)
