# MindSpore Operator

#### Experimental notice: This project is still experimental and only serves as a proof of concept for running MindSpore on Kubernetes. The current version of ms-operator is based on an early version of [PyTorch Operator](https://github.com/kubeflow/pytorch-operator) and [TF Operator](https://github.com/kubeflow/tf-operator). Right now MindSpore supports running LeNet with MNIST dataset on a single node, distributed training examples are expected in the near future.

- [MindSpore Operator](#mindspore-operator)
  - [Introduction of MindSpore and ms-operator](#introduction-of-mindspore-and-ms-operator)
    - [MindSpore docker image](#mindspore-docker-image)
    - [Design](#Design)
    - [Overview of MindSpore in Kubeflow ecosystem](#overview-of-mindspore-in-kubeflow-ecosystem)
  - [Getting Started](#getting-started)
    - [Prerequisites](#prerequisites)
    - [Steps of running the example](#steps-of-running-the-example)
  - [Future Work](#future-work)
  - [Appendix: Example yaml file](#appendix:-example-yaml-file)
  - [Community](#community)
  - [Contributing](#contributing)
  - [License](#license)

## Introduction of MindSpore and ms-operator

MindSpore is a new open source deep learning training/inference framework that
could be used for mobile, edge and cloud scenarios. MindSpore is designed to
provide development experience with friendly design and efficient execution for
the data scientists and algorithmic engineers, native support for Ascend AI
processor, and software hardware co-optimization.

This project contains the specification and implementation of MSJob custom
resource definition. We will demonstrate running a walkthrough of creating
ms-operator, as well as MNIST training job on Kubernetes with MindSpore
`0.1.0-alpha` image (x86 CPU build version) on a single node. More completed
features will be developed in the coming days.

This project defines the following:
- The ms-operator
- A way to deploy the operator
- MindSpore LeNet MNIST training example
- Future goal: distributed MindSpore training example

### MindSpore docker image

Please refer to MindSpore [docker image introduction](https://gitee.com/mindspore/mindspore/blob/master/README.md#docker-image)
for details.

### Design

The yaml file we used to create our MNIST training job is defined as follows:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: msjob-mnist
spec:
  containers:
  - image: mindspore/mindspore-cpu:0.1.0-alpha
    imagePullPolicy: IfNotPresent
    name: msjob-mnist
    command: ["/bin/bash", "-c", "python /tmp/test/MNIST/lenet.py"]
    volumeMounts:
      - name: training-result
        mountPath: /tmp/result
      - name: ms-mnist
        mountPath: /tmp/test
  restartPolicy: OnFailure
  volumes:
    - name: training-result
      emptyDir: {}
    - name: ms-mnist
      hostPath:
        path: /root/gopath/src/gitee.com/mindspore/ms-operator/examples/
```

### Overview of MindSpore in Kubeflow ecosystem

<img src="./docs/pics/ms-operator-in-kubeflow.png" alt="ms-operator in Kubeflow" width=600/>

The high-level view of how MindSpore fits in the ecosystem of Kubeflow and its
components.

## Getting Started

### Prerequisites

- [Ubuntu](http://releases.ubuntu.com/16.04/): `16.04.6 LTS`
- [Helm and Tiller](https://github.com/helm/helm/releases/tag/v2.9.0): `v2.9.0`
- [go](https://github.com/golang/go/releases/tag/go1.12.1): `go1.12.1`
- [docker](https://github.com/docker/docker-ce/releases/tag/v18.06.1-ce): `v18.06.1-ce`
- [Kubernetes](https://github.com/kubernetes/kubernetes/releases/tag/v1.14.0): `v1.14.0`

### Steps of running the example

First, pull the ms-operator image from [Docker Hub](https://hub.docker.com/r/mindspore):
```
docker pull mindspore/ms-operator:latest
```

Or you can build the ms-operator image on local machine:
```
go build -ldflags '-w -s' -o ms-operator cmd/ms-operator.v1/main.go
docker build -t mindspore/ms-operator .
```

After the installation, check the image status using `docker images` command:
```
REPOSITORY                        TAG                   IMAGE ID            CREATED             SIZE
mindspore/ms-operator             latest                4a17028de3d3        5 minutes ago       97.8MB
```

The MindSpore image we download from docker hub is `0.1.0-alpha` version:
```
REPOSITORY                        TAG                   IMAGE ID            CREATED             SIZE
mindspore/mindspore-cpu           0.1.0-alpha           ef443be923bc        3 hours ago         1.05GB
```

MindSpore supports heterogeneous computing including multiple hardware and
backends (`CPU`, `GPU`, `Ascend`), the device_target of MindSpore is
`Ascend` by default but we will use the CPU version here.

Install the msjob crd, ms-operator deployment and pod:
```
RBAC=true # set false if you do not have an RBAC cluster
helm install ms-operator-chart/ -n ms-operator --set rbac.install=${RBAC} --wait --replace
```

Using `helm status ms-operator` command to check generated resources:
```
LAST DEPLOYED: Tue Mar 24 11:36:51 2020
NAMESPACE: default
STATUS: DEPLOYED

RESOURCES:
==> v1beta1/CustomResourceDefinition
NAME                 AGE
msjobs.kubeflow.org  1d

==> v1beta1/Deployment
NAME         DESIRED  CURRENT  UP-TO-DATE  AVAILABLE  AGE
ms-operator  1        1        1           1          1d

==> v1/Pod(related)
NAME                          READY  STATUS   RESTARTS  AGE
ms-operator-7b5b457d69-dpd2b  1/1    Running  0         1d
```

We will do a MNIST training to check the eligibility of MindSpore running on
Kubernetes:
```
cd examples/ && kubectl apply -f ms-mnist.yaml
```

The job is simply importing MindSpore packages, the dataset is already included in the `MNIST_Data` folder, executing only one epoch and printing result which should only consume little time. After the job completed, you should be able to check the job status and see the result logs. You can check the source training code in `examples/` folder.
```
kubectl get pod msjob-mnist && kubectl logs msjob-mnist
```
```
NAME          READY   STATUS      RESTARTS   AGE
msjob-mnist   0/1     Completed   0          3h53m
============== Starting Training ==============
epoch: 1 step: 1, loss is 2.3005836
epoch: 1 step: 2, loss is 2.2978227
epoch: 1 step: 3, loss is 2.3004227
epoch: 1 step: 4, loss is 2.3054247
epoch: 1 step: 5, loss is 2.3068798
epoch: 1 step: 6, loss is 2.298408
epoch: 1 step: 7, loss is 2.3055573
epoch: 1 step: 8, loss is 2.2998955
epoch: 1 step: 9, loss is 2.3028255
epoch: 1 step: 10, loss is 2.2972553
```

Since MindSpore is in the early stage of open source, the whole community is
still working on implementing distributed training of LeNet with MNIST dataset
on Kubernetes, together with the distributed training on different backends
(GPU || `Ascend`) are also expected in the near future.

## Future Work

[Kubeflow](https://github.com/kubeflow/kubeflow) just announced its first major
1.0 release recently with the graduation of a core set of stable applications
including:
- [Kubeflow's UI](https://www.kubeflow.org/docs/components/central-dash/overview/)
- [Jupyter notebook controller](https://github.com/kubeflow/kubeflow/tree/master/components/notebook-controller) and [web app](https://www.kubeflow.org/docs/notebooks/why-use-jupyter-notebook/)
- [Tensorflow Operator](https://www.kubeflow.org/docs/components/training/tftraining/)(TFJob), and [PyTorch Operator](https://www.kubeflow.org/docs/components/training/pytorch/) for distributed training
- [kfctl](https://www.kubeflow.org/docs/other-guides/kustomize/) for deployment and upgrade
- etc.

The MindSpore community is driving to collaborate with the Kubeflow community
as well as making the ms-operator more complex, well-organized and its
dependencies up-to-date. All these components make it easy for machine learning
engineers and data scientists to leverage cloud assets (public or on-premise)
for machine learning workloads.

MindSpore is also looking forward to enable users to use Jupyter to develop
models. Users in the future can use Kubeflow tools like fairing (Kubeflow’s
python SDK) to build containers and create Kubernetes resources to train their
MindSpore models.

Once training completed, users can use [KFServing](https://github.com/kubeflow/kfserving)
to create and deploy a server for inference thus completing the life cycle of
machine learning.

Distributed training is another field MindSpore will be focusing on. There are
two major distributed training strategies nowadays: one based on parameter
servers and the other based on collective communication primitives such as
allreduce. [MPI Operator](https://github.com/kubeflow/mpi-operator) is one of
the core components of Kubeflow which makes it easy to run synchronized,
allreduce-style distributed training on Kubernetes. MPI Operator provides a crd
for defining a training job on a single CPU/GPU, multiple CPU/GPUs, and multiple
nodes. It also implements a custom controller to manage the CRD, create
dependent resources, and reconcile the desired states. If MindSpore can leverage
MPI Operator together with the high performance `Ascend` processor, it is
possible that MindSpore will bring distributed training to an even higher level.

## Appendix: Example yaml file

The yaml file to create distributed training MSJob expected to be like this:
```yaml
# WIP example for distributed training
apiVersion: "kubeflow.org/v1"
kind: "MSJob"
metadata:
  name: "msjob-mnist"
spec:
  backend: "tcp"
  masterPort: "23456"
  replicaSpecs:
    - replicas: 1
      replicaType: MASTER
      template:
        spec:
          containers:
          - image: mindspore/mindspore-cpu:0.1.0-alpha
            imagePullPolicy: IfNotPresent
            name: msjob-mnist
            command: ["/bin/bash", "-c", "python /tmp/test/MNIST/lenet.py"]
            volumeMounts:
              - name: training-result
                mountPath: /tmp/result
              - name: ms-mnist-local-file
                mountPath: /tmp/test
          restartPolicy: OnFailure
          volumes:
            - name: training-result
              emptyDir: {}
            - name: entrypoint
              configMap:
                name: dist-train
                defaultMode: 0755
          restartPolicy: OnFailure
    - replicas: 3
      replicaType: WORKER
      template:
        spec:
          containers:
          - image: mindspore/mindspore-cpu:0.1.0-alpha
            imagePullPolicy: IfNotPresent
            name: msjob-mnist
            command: ["/bin/bash", "-c", "python /tmp/test/MNIST/lenet.py"]
            volumeMounts:
              - name: training-result
                mountPath: /tmp/result
              - name: ms-mnist-local-file
                hostPath:
                    path: /root/gopath/src/gitee.com/mindspore/ms-operator/examples
          restartPolicy: OnFailure
          volumes:
            - name: training-result
              emptyDir: {}
            - name: entrypoint
              configMap:
                name: dist-train
                defaultMode: 0755
          restartPolicy: OnFailure
```

The MSJob currently is designed based on the TF Job and PyTorch Job,
and is subject to change in future versions.

We define `backend` protocol which the MS workers will use to communicate when
initializing the worker group. MindSpore supports heterogeneous computing
including multiple hardware and backends (`CPU`, `GPU`, `Ascend`),
the device_target of MindSpore is `Ascend` by default.

We define `masterPort` that groups will use to communicate with master service.

## Community

- [MindSpore Slack](https://join.slack.com/t/mindspore/shared_invite/enQtOTcwMTIxMDI3NjM0LTNkMWM2MzI5NjIyZWU5ZWQ5M2EwMTQ5MWNiYzMxOGM4OWFhZjI4M2E5OGI2YTg3ODU1ODE2Njg1MThiNWI3YmQ) - Ask questions and find answers.

## Contributing

Welcome contributions. See our [Contributor Wiki](https://gitee.com/mindspore/mindspore/blob/master/CONTRIBUTING.md) for more details.

## License

[Apache License 2.0](LICENSE)
