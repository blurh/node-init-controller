package utils

import (
	"os"
	"reflect"

	corev1 "k8s.io/api/core/v1"
)

var (
    ENVPodNamespaceKey = "POD_NAMESPACE"
    ENVPodNameKey      = "POD_NAME"
    ENVNodeNameKey     = "NODE_ID"
)

func IsNodeStatusReady(node *corev1.Node) bool {
    for _, condition := range node.Status.Conditions {
        if condition.Status == corev1.ConditionTrue {
            if condition.Type == corev1.NodeReady {
                return true
            }
        }
    }

    return false
}

func PtrTool[T any](p T) *T {
    return &p
}

func IsTaintsExists(arr []corev1.Taint, a corev1.Taint) bool {
    for _, v := range arr {
        if reflect.DeepEqual(v, a) {
            return true
        }
    }

    return false
}

func GetMyNamespace() string {
    return os.Getenv(ENVPodNamespaceKey)
}

func GetMyPodName() string {
    return os.Getenv(ENVPodNameKey)
}

func GetMyNodeName() string {
    return os.Getenv(ENVNodeNameKey)
}
