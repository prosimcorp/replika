# Replika

![GitHub Release](https://img.shields.io/github/v/release/prosimcorp/replika)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/prosimcorp/replika)
[![Go Report Card](https://goreportcard.com/badge/github.com/prosimcorp/replika)](https://goreportcard.com/report/github.com/prosimcorp/replika)
![image pulls](https://img.shields.io/badge/+2k-brightgreen?label=image%20pulls)
![GitHub License](https://img.shields.io/github/license/prosimcorp/replika)

![GitHub User's stars](https://img.shields.io/github/stars/prosimcorp?label=Prosimcorp%20Stars)
![GitHub followers](https://img.shields.io/github/followers/prosimcorp?label=Prosimcorp%20Followers)

> **ATTENTION:** From v0.4.0+ bundled Kubernetes deployment manifests are built and uploaded to the releases.
> We do this to keep them atomic between versions. Due to this, `deploy` directory will be removed from repository.
> Please, read [related section](#deployment)

## Description
A Kubernetes operator to replicate a resource across namespaces

## Motivation

The GitOps approach has demonstrated being the best way to keep the traceability and reproducibility of a deployment
for any project. Not only for developers' applications but for the SRE tools inside the cluster too. As always, challenges
have appeared around that way of doing things. 

1. Credentials are sensitive and no one should manipulate them by using oiled hands. Some solutions for this kind of use 
   cases have appeared at the same time. For example, credentials can be stored on a vault provider and the retrieval can 
   be automated using External Secrets, which can create Secrets inside the cluster using CRs `kind: ExternalSecret`. 
   The credentials can be templated before producing a Secret and that is powerful to create different type of Secrets, 
   such as those with `type: kubernetes.io/dockerconfigjson` to get images from private registries. In most cases SRE 
   members are in charge of that ownership, and they would have to deploy the same exact ExternalSecret resource inside 
   all namespaces to produce the same exact Secret with the same credentials. 
   We can solve this case using Replica, **creating exactly one ExternalSecret to produce the Secret only once, and 
   replicate it across all namespaces**, always keeping them synchronized to the source.


2. Another problem is about limitations. Some companies create fully automated Kubernetes clusters. One of the most 
   automated things out there is the monitoring stack, in most cases including the famous kube-prometheus-stack Helm chart,
   a meta-chart to deploy several things, such as Prometheus or Alertmanager by using the Prometheus Operator. Alertmanager
   can be configured using a CR of `kind: AlertmanagerConfig` to send notifications to Slack, mail, etc. The limitation here
   is about Prometheus Operator fixing a parameter called `matchers`, allowing only to send notifications produced inside 
   the same namespace where the `AlertmanagerConfig` CR is deployed. This is done for security reasons but the behaviour 
   can not be changed. This limitation, once again, can be handled just by deploying the same resource across namespaces,
   allowing to monitor all of them. **Using this operator you could simply create an `AlertmanagerConfig` and a `Replika`
   to replicate it across all namespaces**, being even able to exclude some of them. 

## Deployment

We have designed the deployment of this project to allow remote deployment using Kustomize. This way it is possible
to use it with a GitOps approach, using tools such as ArgoCD or FluxCD. Just make a Kustomization manifest referencing 
the tag of the version you want to deploy as follows:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- https://github.com/prosimcorp/replika/releases/download/v0.4.0/bundle.yaml
```

> 🧚🏼 **Hey, listen! If you prefer to deploy using Helm, go to the [Helm registry](https://github.com/prosimcorp/helm-charts)**

## RBAC

We designed the operator to be able to replicate any kind of resource in a Kubernetes cluster, but by design, Kubernetes
permissions are always only additive. This means that we had to grant only some resources to be replicated by default,
such as Secrets and ConfigMaps. But you can replicate other kind of resources just granting some permissions to the 
ServiceAccount of the controller as follows:

```yaml
# clusterRole-replika-custom-resources.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
   name: replika-custom-resources
rules:
   - apiGroups:
        - ""
     resources:
        - AlertmanagerConfigs
     verbs:
        - create
        - delete
        - get
        - list
        - patch
        - update
        - watch
---
# clusterRoleBinding-replika-custom-resources.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
   name: replika-custom-resources
roleRef:
   apiGroup: rbac.authorization.k8s.io
   kind: ClusterRole
   name: replika-custom-resources
subjects:
   - kind: ServiceAccount
     name: replika-controller-manager
     namespace: replika
---
# kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
   - https://github.com/prosimcorp/replika/releases/download/v0.4.0/bundle.yaml
   
   # Add your custom resources
   - clusterRole-replika-custom-resources.yaml
   - clusterRoleBinding-replika-custom-resources.yaml
```

## Example

To replicate resources using this operator you will need to create a CR of kind Replika. You can find the spec samples
for all the versions of the resource in the [examples directory](configamples)

You may prefer to learn directly from an example, so let's explain it replicating a ConfigMap resource:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: sample-configmap
data:
  example-key: value
```

Now use a Replika CR to replicate this resource across all namespaces, excluding some sensitive ones:

```yaml
apiVersion: replika.prosimcorp.com/v1beta1
kind: Replika
metadata:
  name: replika-sample
spec:
  # Some configuration features
  synchronization:
    time: "20s"

  # Defines the resource to sync through namespaces
  source:
    group: ""
    version: v1
    kind: ConfigMap
    name: sample-configmap
    namespace: &sourceNamespace default

  # Defines the resources that will be generated
  target:
    namespaces:
      # List of namespaces where to replicate the resources when 'matchAll' is disabled
      replicateIn: []

      # Replicate the resource in all namespaces, some of them are excluded
      matchAll: true
      excludeFrom:
        - kube-system
        - kube-public
        - kube-node-lease
        - *sourceNamespace
```

Replika is done thinking about reliability first, and due to it is designed to modify resources across namespaces, we
have contemplated several risky situations where Replika could break your environment and designed the operator to simply
ignores your destruction desires. For example, it will not replicate sources of `kind: Namespace`. Another risky situation
could happen when the target namespace is the same as the source namespace, because it would overwrite the source.
Don't worry, at ProsimCorp we are used to failing a lot, so we design our tools to avoid out own failures.

## How to develop

> We recommend you to use a development tool like [Kind](https://kind.sigs.k8s.io/) or [Minikube](https://minikube.sigs.k8s.io/docs/start/)
> to launch a lightweight Kubernetes on your local machine for development purposes

For learning purposes, we will suppose you are going to use Kind. So the first step is to create a Kubernetes cluster
on your local machine executing the following command:

```console
kind create cluster
```

Once you have launched a safe play place, execute the following command. It will install the custom resource definitions 
(CRDs) in the cluster configured in your ~/.kube/config file and run the Operator locally against the cluster:

```console
make install run
```

> Remember that your `kubectl` is pointing to your Kind cluster. However, you should always review the context your 
> kubectl CLI is pointing to

## How releases are created

Each release of this operator is done following several steps carefully in order not to break the things for anyone.
Reliability is important to us, so we automated all the process of launching a release. For a better understanding of 
the process, the steps are described in the following recipe:

1. Test the changes on the code:

    ```console
    make test
    ```

    > A release is not done if this stage fails


2. Define the package information

    ```console
    export VERSION="0.0.1"
    export IMG="ghcr.io/prosimcorp/replika:v$VERSION"
    ```

3. Generate and push the Docker image (published on Docker Hub).
   
    ```console
    make docker-build docker-push
    ```

4. Generate the manifests for deployments using Kustomize

   ```console
    make bundle-build
    ```

## How to collaborate

This project is done on top of Kubebuilder, so read about that project before collaborating. Of course, we are open to 
external collaborations for this project. For doing it you must fork the repository, make your changes to the code and 
open a PR. The code will be reviewed and tested (always)

> We are developers and hate bad code. For that reason we ask you the highest quality on each line of code to improve
> this project on each iteration.

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
