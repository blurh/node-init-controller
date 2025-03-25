package controllers

import (
    "context"
    "net/http"
    "os"
    "sync"
    "time"

    apierrors "k8s.io/apimachinery/pkg/api/errors"
    "k8s.io/apimachinery/pkg/types"
    "k8s.io/client-go/util/retry"

    batchv1 "k8s.io/api/batch/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/event"
    logf "sigs.k8s.io/controller-runtime/pkg/log"
    "sigs.k8s.io/controller-runtime/pkg/log/zap"
    "sigs.k8s.io/controller-runtime/pkg/manager"
    "sigs.k8s.io/controller-runtime/pkg/manager/signals"
    "sigs.k8s.io/controller-runtime/pkg/metrics/server"
    "sigs.k8s.io/controller-runtime/pkg/predicate"
    "sigs.k8s.io/controller-runtime/pkg/webhook"

    "github.com/blurh/node-init-controller/pkg/utils"
)

const (
    TypeInitialized   = "initialized"
    TypeInitializing  = "initializing"
    TypeReinitialized = "reinit"

    LabelNodeInitKey          = "node-init"
    LabelNodeInitDisableValue = "disable"

    ControllerName = "node-init-controller"
)

var (
    clog = logf.Log.WithName(ControllerName)
)

type Controller struct {
    mgr manager.Manager
}

func NewController() *Controller {
    logf.SetLogger(zap.New(zap.UseDevMode(true)))
    mgr, _ := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
        Metrics: server.Options{
            BindAddress: ":8090",
        },
        WebhookServer: webhook.NewServer(webhook.Options{
            Port: 8443,
        }),
        // LeaderElectionID: ControllerName,
        // LeaderElection:   true,
        // LeaseDuration:    utils.PtrTool(30 * time.Second),
        // RenewDeadline:    utils.PtrTool(10 * time.Second),
        // RetryPeriod:      utils.PtrTool(5 * time.Second),
    })
    mgr.GetWebhookServer().Register("/unschedule", &NodeUnsheduleHandler{})
    mgr.AddHealthzCheck("/healthz", func(req *http.Request) error {
        return nil
    })
    mgr.AddReadyzCheck("/readyz", func(req *http.Request) error {
        return nil
    })
    ctrl.NewControllerManagedBy(mgr).
        For(&corev1.Node{}).
        WithEventFilter(predicate.Funcs{
            CreateFunc: func(e event.CreateEvent) bool {
                labels := e.Object.GetLabels()
                for key, value := range labels {
                    if key == LabelNodeInitKey {
                        if value == LabelNodeInitDisableValue || value == TypeInitialized {
                            return false
                        }
                    }
                }
                return true
            },
            UpdateFunc: func(e event.UpdateEvent) bool {
                // 主要用于判断 reinit
                labels := e.ObjectNew.GetLabels()
                for key, value := range labels {
                    if key == LabelNodeInitKey && value == TypeReinitialized {
                        return true
                    }
                }
                return false
            },
            DeleteFunc: func(e event.DeleteEvent) bool {
                labels := e.Object.GetLabels()
                for key, value := range labels {
                    if key == LabelNodeInitKey && value == LabelNodeInitDisableValue {
                        return false
                    }
                }
                return true
            },
        }).
        Complete(&NodeReconciler{
            Client:      mgr.GetClient(),
            ongoingJobs: map[string]struct{}{},
            lock:        &sync.Mutex{},
        })

    return &Controller{
        mgr: mgr,
    }
}

func (c *Controller) Start() {
    if err := c.mgr.Start(signals.SetupSignalHandler()); err != nil {
        clog.Error(err, "unable to continue running manager")
        os.Exit(1)
    }
}

type NodeReconciler struct {
    client.Client
    ongoingJobs map[string]struct{}
    lock        *sync.Mutex
}

func (r *NodeReconciler) isJobOngoing(nodeName string) bool {
    r.lock.Lock()
    defer r.lock.Unlock()

    if _, ok := r.ongoingJobs[nodeName]; ok {
        return true
    }

    return false
}

func (r *NodeReconciler) setJobOngoingStatus(nodeName string, status bool) {
    r.lock.Lock()
    defer r.lock.Unlock()

    if status {
        r.ongoingJobs[nodeName] = struct{}{}
    } else {
        delete(r.ongoingJobs, nodeName)
    }
}

// TODO

func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    ns := utils.GetMyNamespace()
    nodeName := req.Name

    jobName := JobNamePrefix + nodeName // 约定的 job name
    nodeNsName := types.NamespacedName{
        Name: nodeName,
    }

    jobNsName := types.NamespacedName{
        Name:      jobName,
        Namespace: ns,
    }

    // 判断是否清理 job
    node := &corev1.Node{}
    err := r.Get(ctx, nodeNsName, node)
    if err != nil {
        // node 不存在, 直接清理 job
        if apierrors.IsNotFound(err) {
            err := r.deleteJob(ctx, jobNsName)
            if err != nil {
                if apierrors.IsNotFound(err) {
                    clog.Info("job: " + jobName + " not found, done")
                    return ctrl.Result{}, nil
                }
                clog.Info("delete job: " + jobName + " error: " + err.Error() + ", requeue")
                return ctrl.Result{Requeue: true}, nil
            }
        }
        clog.Info("get node: " + nodeName + " error: " + err.Error() + ", requeue")
        return ctrl.Result{Requeue: true}, nil
    }
    if node.GetDeletionTimestamp() != nil {
        if err := r.deleteJob(ctx, jobNsName); err != nil {
            if apierrors.IsNotFound(err) {
                return ctrl.Result{}, nil
            }
            clog.Info("delete job: " + jobName + " error: " + err.Error() + ", requeue")
            return ctrl.Result{Requeue: true}, nil
        }
        return ctrl.Result{}, nil
    }

    // 不是删除事件则进行创建
    if r.isJobOngoing(nodeName) {
        clog.Info("node: " + nodeName + " already initializing, skip")
        return ctrl.Result{}, nil
    }
    job := InitalizeJob(jobName, ns, nodeName)
    err = r.Get(ctx, jobNsName, &batchv1.Job{})
    if err != nil {
        // 不存在则进行创建
        if apierrors.IsNotFound(err) {
            err = r.Create(ctx, job, &client.CreateOptions{})
            // 创建 job 失败
            if err != nil {
                clog.Error(err, "create job fail: "+jobName)
                r.setJobOngoingStatus(nodeName, false)
                return ctrl.Result{}, nil
            }
            // 创建 job 成功
            clog.Info("create job success: " + jobName)
            r.setJobOngoingStatus(nodeName, true)
            go checkJobStatus(r, nodeName, jobName, ns)
            return ctrl.Result{}, nil
        }
        // 其他异常
        r.setJobOngoingStatus(nodeName, false)
        clog.Error(err, "create job fail: "+jobName+", requeue")
        return ctrl.Result{Requeue: true}, nil
    }

    // 判断是否是 reinit
    if node.Labels[LabelNodeInitKey] == TypeReinitialized {
        clog.Info("node: " + nodeName + " reinit, requeue")
        err := r.deleteJob(ctx, jobNsName)
        if err != nil {
            clog.Info("delete job: " + jobName + " error: " + err.Error() + ", requeue")
        }
        retry.RetryOnConflict(retry.DefaultRetry, func() error {
            delete(node.Labels, LabelNodeInitKey)
            return r.Update(context.TODO(), node, &client.UpdateOptions{})
        })

        return ctrl.Result{Requeue: true}, nil
    }

    return ctrl.Result{}, nil
}

func (r *NodeReconciler) deleteJob(ctx context.Context, jobNsName types.NamespacedName) error {
    job := &batchv1.Job{}
    if err := r.Get(ctx, jobNsName, job); err != nil {
        return err
    }

    if err := r.Delete(context.TODO(), job, &client.DeleteOptions{
        PropagationPolicy: utils.PtrTool(metav1.DeletePropagationBackground),
    }); err != nil {
        return err
    }
    r.setJobOngoingStatus(jobNsName.Name, false)
    return nil
}

func checkJobStatus(r *NodeReconciler, nodeName, jobName, ns string) {
    // 10s 检查一次是否完成了
    ticker := time.NewTicker(time.Second * 10)

    r.setJobOngoingStatus(nodeName, true)
    defer r.setJobOngoingStatus(nodeName, false)

    job := &batchv1.Job{}
    nsName := types.NamespacedName{
        Name:      jobName,
        Namespace: ns,
    }
    for {
        select {
        case <-ticker.C:
            nodeNsName := types.NamespacedName{
                Name: nodeName,
            }
            err := r.Get(context.TODO(), nsName, job, &client.GetOptions{})
            if err != nil {
                if apierrors.IsNotFound(err) {
                    // 如果相应的节点已经不存在了, 就退出
                    err := r.Get(context.TODO(), nodeNsName, &corev1.Node{}, &client.GetOptions{})
                    if apierrors.IsNotFound(err) {
                        clog.Info("checking job, node: " + nodeName + " not found, exit the goroutine")
                        return
                    }
                }
                clog.Error(err, "get job: "+jobName+" error")
                continue
            }
            clog.Info("check job: " + jobName)
            for _, condition := range job.Status.Conditions {
                if condition.Type == batchv1.JobComplete {
                    // 正常退出
                    clog.Info("check job: " + nodeName + " success")
                    return
                }
            }
        }
    }
}
