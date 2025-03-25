
# node-init-controller

## 作用

用于节点初始化, 自动打上配置的 labels 和 taints  
可以用于部分不支持配置自动初始化脚本的托管集群, 如 akamai 的 LKE  

## 构建

pacher

```bash
docker build --build-arg MODULE=patcher . -t swr.ap-southeast-1.myhuaweicloud.com/blurh/node-patcher:latest 
```

controller

```bash
docker build --build-arg MODULE=controller . -t swr.ap-southeast-1.myhuaweicloud.com/blurh/node-controller:v2.2.6
```

## 配置文件

### patcher 的配置镜像仓库和 imagePullSecret

> deploy/prod/config.yaml

```yaml
registry: "docker.io"
imagePullSecret: "default-registry-secret"
```

### controller 的镜像

> deploy/prod/kustomization.yaml

```yaml
images:
- name: node-controller:latest
  newName: swr.ap-southeast-1.myhuaweicloud.com/blurh/node-controller
  newTag: latest
```

### 给节点自动打标签和容忍

> deploy/prod/config.yaml

```yaml
labels:
- key: "node-role.kubernetes.io/worker"
  value: "true"

taints:
- key: "node"
  value: "worker"
  effect: "NoSchedule"
```

### 初始化脚本

> deploy/prod/config.yaml

```yaml
initScript: |
  #!/bin/bash
```

## 部署

```bash
kubectl apply -k deploy/prod
```

## 与动作相关的 label

所有创建出来的节点都默认会被进行初始化, 但可以对节点 label 来执行一些动作  

### 节点不执行初始化操作

给节点打上 `node-init=disable` 标签

```bash
k label no <node-name> node-init=disable --overwrite
```

### 重新 init

将节点 `node-init` 标签设置为 `reinit`  

```bash
k label no <node-name> node-init=reinit --overwrite
```

### 取消污点/标签


```bash
k label no <node-name> node-
```

```bash
k taint no <node-name> node-
```
