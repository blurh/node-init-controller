package config

import (
    "io"
    "os"
    "testing"

    corev1 "k8s.io/api/core/v1"
)

func TestConfig(t *testing.T) {
    tmpDirName := "config"
    os.Mkdir(tmpDirName, 0600)
    tmpfile, _ := os.OpenFile("config/config.yaml", os.O_CREATE|os.O_RDWR, 0600)
    testfile, _ := os.Open("../../test/config.yaml")
    io.Copy(tmpfile, testfile)

    defer func() {
        testfile.Close()
        os.Remove(tmpfile.Name())
        os.Remove(tmpDirName)
    }()

    type T any
    assert := func(result, expect, errmsg T) {
        if result != expect {
            t.Error(errmsg)
        }
    }

    c := NewConfig()
    assert(c.GetInitScript(), "#!/bin/bash\n", "test get initscript fail")

    labels := c.GetLabels()
    assert(labels[0].Key, "label-key", "test get labels key fail")
    assert(labels[0].Value, "label-value", "test get labels value fail")

    taints := c.GetTaints()
    assert(taints[0].Key, "taint-key", "test get taints key fail")
    assert(taints[0].Value, "taint-value", "test get taints value fail")
    assert(taints[0].Effect, corev1.TaintEffect("NoSchedule"), "test get taints effect fail")

    assert(c.GetRegistry(), "docker.io/", "test get registry fail")
    assert(c.GetImagePullSecret(), "default-registry-secret", "test get image pull secret fail")
}
