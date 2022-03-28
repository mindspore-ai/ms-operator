# MindSpore Operator
MindSpore Operator 是Mindspore在Kubernetes上进行分布式训练的插件。CRD中定义了Scheduler、PS、Worker三种角色，用户只需配置yaml文件，即可轻松实现分布式训练。

# 安装
安装方法可以有以下几种
## 1. 使用yaml直接安装
```
kubectl apply -f deploy/v1/ms-operator.yaml
```
安装后：
使用`kubectl get pods --all-namespaces`,即可看到namespace为ms-operator-system的部署任务。
使用`kubectl describe pod me-operator-controller-manager-xxx-xxx -n ms-operator-sysytem`，可查看pod的详细信息。
## 2. 使用make deploy安装
```
make deploy IMG=swr.cn-south-1.myhuaweicloud.com/mindspore/ms-operator:latest
```

## 3. 本地调试环境
```
make run
```

# Samples
当然ms-operator支持普通单Worker训练、PS模式的单Worker训练以及数据并行的Scheduler、Worker非MPI启动。
在`config/samples/`中有运行样例。
以数据并行的Scheduler、Worker非MPI启动为例，其中数据集和网络脚本需提前准备：
```
kubectl apply -f config/samples/ms_wide_deep_dataparallel.yaml
```
使用`kubectl get all -o wide`即可看到集群中启动的Scheduler和Worker，以及Scheduler对应的Service。
# 开发指南
## 核心代码：
`pkg/apis/v1/msjob_types.go`中为MSJob的CRD定义。
`pkg/controllers/v1/msjob_controller.go`中为MSJob controller的核心逻辑。
## 镜像制作、上传
```
make docker-build IMG=swr.cn-south-1.myhuaweicloud.com/mindspore/ms-operator:latest
docker push swr.cn-south-1.myhuaweicloud.com/mindspore/ms-operator:latest
```
## 常见问题
- 镜像构建过程中若发现gcr.io/distroless/static无法拉取，可参考[issue](https://github.com/anjia0532/gcr.io_mirror/issues/169)
- 安装部署过程中发现gcr.io/kubebuilder/kube-rbac-proxy无法拉取，参考[issue](https://github.com/anjia0532/gcr.io_mirror/issues/153)

