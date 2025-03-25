package controllers

import (
    "os"
    "sync"

    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/clientcmd"
    "k8s.io/client-go/util/flowcontrol"
)

var (
    ClientSet *kubernetes.Clientset
    once      = sync.Once{}
)

func NewClientSet() *kubernetes.Clientset {
    once.Do(func() {
        kubeconfig, _ := rest.InClusterConfig()
        if os.Getenv("cluster") == "false" {
            kubeconfig, _ = clientcmd.BuildConfigFromFlags("", "/root/.kube/config")
        }
        kubeconfig.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(1000, 1000)
        clientset, err := kubernetes.NewForConfig(kubeconfig)
        if err != nil {
            panic(err)
        }
        ClientSet = clientset
    })

    return ClientSet
}
