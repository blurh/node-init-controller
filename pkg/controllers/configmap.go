package controllers

import (
    "context"

    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
    kubernetes "k8s.io/client-go/kubernetes"

    utils "github.com/blurh/node-init-controller/pkg/utils"
)

var InitCMName = "init-files"

func ApplyInitScriptConfigMap(clientset *kubernetes.Clientset, initScript string) error {
    ns := utils.GetMyNamespace()
    cm := applycorev1.ConfigMap(InitCMName, ns).
        WithData(map[string]string{
            "init.sh": initScript,
        })
    _, err := clientset.CoreV1().ConfigMaps(ns).Apply(context.TODO(), cm, metav1.ApplyOptions{
        FieldManager: Manager,
    })
    if err != nil {
        return nil
    }

    return nil
}
