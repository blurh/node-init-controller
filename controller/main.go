package main

import (
    "time"

    "github.com/blurh/node-init-controller/pkg/config"
    "github.com/blurh/node-init-controller/pkg/controllers"
)

func main() {
    c := config.NewConfig()
    initScript := c.GetInitScript()

    clientset := controllers.NewClientSet()
    controllers.ApplyInitScriptConfigMap(clientset, initScript)

    go func() {
        ticker := time.NewTicker(time.Minute * 5)
        for {
            select {
            case <-ticker.C:
                controllers.ApplyInitScriptConfigMap(clientset, initScript)
            }
        }
    }()

    go func() {
        ticker := time.NewTicker(time.Minute * 5)
        for {
            select {
            case <-ticker.C:
                controllers.CleanOrphanJob(clientset)
            }
        }
    }()

    controller := controllers.NewController()
    controller.Start()
}
