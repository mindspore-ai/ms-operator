# MindSpore Operator

#### Experimental notice: This project is still experimental and only serves as a proof of concept for running MindSpore on Kubernetes. The current version of ms-operator is based on an early version of [PyTorch Operator](https://github.com/kubeflow/pytorch-operator) and [TF Operator](https://github.com/kubeflow/tf-operator). Right now MindSpore supports running LeNet with MNIST dataset on a single node, while distributed training examples are conducted using [Volcano](https://github.com/volcano-sh/volcano).

- [MindSpore Operator](#mindspore-operator)
  - [Introduction of MindSpore and ms-operator](#introduction-of-mindspore-and-ms-operator)
    - [MindSpore docker image](#mindspore-docker-image)
    - [Design](#Design)
    - [Overview of MindSpore in Kubeflow ecosystem](#overview-of-mindspore-in-kubeflow-ecosystem)
  - [MindSpore CPU example](#mindspore-cpu-example)
    - [Prerequisites](#prerequisites)
    - [Steps](#steps)
  - [Distributed GPU example using Volcano](#distributed-gpu-example-using-volcano)
    - [Volcano prerequistites](#volcano-prerequisites)
    - [MindSpore GPU example](#mindspore-gpu-example)
  - [Future Work](#future-work)
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
ms-operator, as well as MNIST training job on Kubernetes with MindSpore-cpu image 
(x86 CPU build version) on a single node. More completed features will be 
developed in the coming days.

This project defines the following:
- The ms-operator
- A way to deploy the operator
- MindSpore LeNet MNIST training example
- MindSpore distributed GPU example using Volcano
- etc.

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

## MindSpore CPU example

### Prerequisites

- [Ubuntu](http://releases.ubuntu.com/16.04/): `16.04.6 LTS`
- [Helm and Tiller](https://github.com/helm/helm/releases/tag/v2.9.0): `v2.9.0`
- [go](https://github.com/golang/go/releases/tag/go1.12.1): `go1.12.1`
- [docker](https://github.com/docker/docker-ce/releases/tag/v18.06.1-ce): `v18.06.1-ce`
- [Kubernetes](https://github.com/kubernetes/kubernetes/releases/tag/v1.14.0): `v1.14.0`

### Steps

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
The expected output is:

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


## Distributed GPU example using Volcano

The source code of the example can be found [here](https://github.com/volcano-sh/volcano/tree/master/example/MindSpore-example).

### Volcano Prerequisites

- Kubernetes: `v1.16.6`
- NVIDIA Docker: `2.3.0`
- NVIDIA/k8s-device-plugin: `1.0.0-beta6`
- NVIDIA drivers: `418.39`
- CUDA: `10.1`

Install Volcano: `kubectl apply -f https://raw.githubusercontent.com/volcano-sh/volcano/master/installer/volcano-development.yaml`

### MindSpore GPU example

Using a modified image which the openssh-server is installed from the official MindSpore GPU image. To check the eligibility of MindSpore GPU's ability to communicate with other processes, we leverage the mpimaster and mpiworker task spec of Volcano. In this example, we launch one mpimaster and two mpiworkers, the python script is taken from MindSpore Gitee README, which is also modified to be able to run parallelly.

cd to `example/MindSpore-example/mindspore_gpu` folder, then:  
pull image: `docker pull lyd911/mindspore-gpu-example:0.2.0`  
to run: `kubectl apply -f mindspore-gpu.yaml`  
to check result: `kubectl logs mindspore-gpu-mpimster-0`  

The expected output should be (2*3) of multi-dimensional array.

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

## Community

- [MindSpore Slack](https://join.slack.com/t/mindspore/shared_invite/enQtOTcwMTIxMDI3NjM0LTNkMWM2MzI5NjIyZWU5ZWQ5M2EwMTQ5MWNiYzMxOGM4OWFhZjI4M2E5OGI2YTg3ODU1ODE2Njg1MThiNWI3YmQ) - Ask questions and find answers.

## Contributing

Welcome contributions. See our [Contributor Wiki](https://gitee.com/mindspore/mindspore/blob/master/CONTRIBUTING.md) for more details.

## License

[Apache License 2.0](LICENSE)
