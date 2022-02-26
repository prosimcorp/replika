# Replika

## Description
A Kubernetes operator to replicate a resource across namespaces

## What to expect

## What not to expect

## Deployment

## RBAC

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

## How to package

Test the code
```console
make test
```

Define the package information
```console
export VERSION="0.0.1"
export IMG="prosimcorp/replika:v$VERSION"
```

Generate and push the Docker image (i.e. hub.docker.com)
This will invoke `docker` under the hoods
```console
make docker-build docker-push
```

Generate the standalone manifests to build packages for Helm
```console
make dry-run
```

Generate and push a bundle for OLM (i.e. operatorhub.io)
This will invoke `olm` CLI on under the hoods
```console
make bundle bundle-build bundle-push
```

## Example

# References
[Operator SDK tutorial for Go-based Operators](https://docs.openshift.com/container-platform/4.7/operators/operator_sdk/golang/osdk-golang-tutorial.html)
