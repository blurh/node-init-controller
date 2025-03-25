package controllers

import (
    "context"
    "log"
    "strings"

    apibatchv1 "k8s.io/api/batch/v1"
    apicorev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/labels"
    "k8s.io/client-go/kubernetes"

    "github.com/blurh/node-init-controller/pkg/config"
    "github.com/blurh/node-init-controller/pkg/utils"
)

const (
    JobLabelKey   = "controller"
    JobLabelValue = "node-initer"
    JobNamePrefix = "initialize-"

    Manager = "node-init-controller"
)

func InitalizeJob(jobName, jobNamespace, nodeName string) *apibatchv1.Job {
    volumeInitScriptName := "init-script"
    volumeHostScriptName := "host-script"

    c := config.NewConfig()
    registry := c.GetRegistry()
    patcherImagePullSecret := c.GetImagePullSecret()
    patcherImageName := strings.TrimRight(registry, "/") + "/node-patcher:latest"

    return &apibatchv1.Job{
        ObjectMeta: metav1.ObjectMeta{
            Name:      jobName,
            Namespace: jobNamespace,
            Labels: map[string]string{
                JobLabelKey: JobLabelValue,
            },
        },
        Spec: apibatchv1.JobSpec{
            Completions:  utils.PtrTool(int32(1)),
            Parallelism:  utils.PtrTool(int32(1)),
            BackoffLimit: utils.PtrTool(int32(5)),
            Template: apicorev1.PodTemplateSpec{
                Spec: apicorev1.PodSpec{
                    InitContainers: []apicorev1.Container{
                        {
                            Name:            "sync-initer",
                            Image:           "docker.io/library/alpine",
                            ImagePullPolicy: apicorev1.PullIfNotPresent,
                            Command: []string{
                                "sh",
                                "-c",
                            },
                            Args: []string{
                                `cat /tmp/init/init.sh > /opt/script/init.sh && chmod +x /opt/script/init.sh `,
                            },
                            VolumeMounts: []apicorev1.VolumeMount{
                                {
                                    Name:      volumeInitScriptName,
                                    MountPath: "/tmp/init/",
                                },
                                {
                                    Name:      volumeHostScriptName,
                                    MountPath: "/opt/script/",
                                },
                            },
                            SecurityContext: &apicorev1.SecurityContext{
                                Privileged: utils.PtrTool(true),
                            },
                        },
                    },
                    Containers: []apicorev1.Container{
                        {
                            Name:            "initer",
                            Image:           "docker.io/library/alpine",
                            ImagePullPolicy: apicorev1.PullIfNotPresent,
                            Command: []string{
                                "nsenter",
                                "--target",
                                "1",
                                "--mount",
                                "--uts",
                                "--ipc",
                                "--net",
                                "--pid",
                            },
                            Args: []string{
                                "--",
                                "/opt/script/init.sh",
                            },
                            VolumeMounts: []apicorev1.VolumeMount{
                                {
                                    Name:      volumeInitScriptName,
                                    MountPath: "/tmp/init/",
                                },
                                {
                                    Name:      volumeHostScriptName,
                                    MountPath: "/opt/script/",
                                },
                            },
                            SecurityContext: &apicorev1.SecurityContext{
                                Privileged: utils.PtrTool(true),
                            },
                        },
                        {
                            Name:            "node-patcher",
                            Image:           patcherImageName,
                            ImagePullPolicy: apicorev1.PullAlways,
                            Command: []string{
                                "/opt/app/patcher",
                            },
                            Env: []apicorev1.EnvVar{
                                {
                                    Name: "NODE_ID",
                                    ValueFrom: &apicorev1.EnvVarSource{
                                        FieldRef: &apicorev1.ObjectFieldSelector{
                                            FieldPath: "spec.nodeName",
                                        },
                                    },
                                },
                            },
                            VolumeMounts: []apicorev1.VolumeMount{
                                {
                                    Name:      volumeHostScriptName,
                                    MountPath: "/opt/script/",
                                },
                            },
                        },
                    },
                    HostNetwork:   true,
                    HostPID:       true,
                    RestartPolicy: "Never",
                    Volumes: []apicorev1.Volume{
                        {
                            Name: volumeInitScriptName,
                            VolumeSource: apicorev1.VolumeSource{
                                ConfigMap: &apicorev1.ConfigMapVolumeSource{
                                    LocalObjectReference: apicorev1.LocalObjectReference{
                                        Name: InitCMName,
                                    },
                                    DefaultMode: utils.PtrTool(int32(0777)),
                                },
                            },
                        },
                        {
                            Name: volumeHostScriptName,
                            VolumeSource: apicorev1.VolumeSource{
                                HostPath: &apicorev1.HostPathVolumeSource{
                                    Path: "/opt/script",
                                },
                            },
                        },
                    },
                    NodeName: nodeName,
                    ImagePullSecrets: []apicorev1.LocalObjectReference{
                        {
                            Name: patcherImagePullSecret,
                        },
                    },
                    ServiceAccountName: "node-init-controller",
                    Tolerations: []apicorev1.Toleration{
                        {
                            Effect:   apicorev1.TaintEffectNoSchedule,
                            Operator: apicorev1.TolerationOpExists,
                        },
                        {
                            Operator: apicorev1.TolerationOpExists,
                        },
                    },
                },
            },
        },
    }
}

func CleanOrphanJob(clientset *kubernetes.Clientset) {
    namespace := utils.GetMyNamespace()
    jobs, err := clientset.BatchV1().Jobs(namespace).List(context.TODO(), metav1.ListOptions{
        LabelSelector: labels.SelectorFromSet(
            map[string]string{
                JobLabelKey: JobLabelValue,
            },
        ).String(),
    })
    if err != nil {
        panic(err)
    }
    nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})

    for _, job := range jobs.Items {
        isNotFoundNode := true
        for _, node := range nodes.Items {
            if JobNamePrefix+node.Name == job.Name {
                // found
                isNotFoundNode = false
                break
            }
        }
        if isNotFoundNode {
            if err := clientset.BatchV1().Jobs(namespace).Delete(context.TODO(), job.Name, metav1.DeleteOptions{
                PropagationPolicy: utils.PtrTool(metav1.DeletePropagationBackground),
            }); err != nil {
                log.Printf("clean orphan job: %s fail: %v", job.Name, err)
            } else {
                log.Printf("clean orphan job: %s success", job.Name)
            }
        }
    }
}
