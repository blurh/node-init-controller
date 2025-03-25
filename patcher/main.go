package main

import (
    "context"
    "log"
    "time"

    "github.com/blurh/node-init-controller/pkg/config"
    "github.com/blurh/node-init-controller/pkg/controllers"
    "github.com/blurh/node-init-controller/pkg/utils"

    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

    "k8s.io/client-go/util/retry"
)

func main() {
    // 小睡一会
    time.Sleep(5 * time.Second)

    clientset := controllers.NewClientSet()
    nodeName := utils.GetMyNodeName()
    log.Println(nodeName)

    c := config.NewConfig()
    labels := c.GetLabels()
    taints := c.GetTaints()

    err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
        node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
        if err != nil {
            log.Printf("get node %s fail: %v", nodeName, err)
            panic(err)
        }
        newNode := node.DeepCopy()
        // 允许调度
        newNode.Spec.Unschedulable = false
        // 打上标签
        newNode.Labels[controllers.LabelNodeInitKey] = controllers.TypeInitialized
        for _, l := range labels {
            // 不能使用 `node-init` 这个预留标签
            if l.Key == controllers.LabelNodeInitKey {
                log.Printf("label key %s is reserved", controllers.LabelNodeInitKey)
                continue
            }
            newNode.Labels[l.Key] = l.Value
        }
        // 打上污点
        for _, t := range taints {
            newTaint := corev1.Taint{
                Key:    t.Key,
                Value:  t.Value,
                Effect: t.Effect,
            }
            if !utils.IsTaintsExists(node.Spec.Taints, newTaint) {
                newNode.Spec.Taints = append(node.Spec.Taints, newTaint)
            }
        }
        _, err = clientset.CoreV1().Nodes().Update(context.TODO(), newNode, metav1.UpdateOptions{})
        return err
    })

    if err != nil {
        log.Printf("get node %s fail: %v", nodeName, err)
        panic(err)
    }

    log.Printf("patch node: %s success", nodeName)
}
