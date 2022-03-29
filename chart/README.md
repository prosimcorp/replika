# replika

A Kubernetes operator to replicate a resource across namespaces

## Prerequisites

* Helm 3+

## Install Chart

```console
# Clone repository
$ git clone https://github.com/prosimcorp/replika.git

# Helm
$ helm install [RELEASE_NAME] replika/chart
```

This install all the Kubernetes components associated with the chart and creates the release.

_See [helm install](https://helm.sh/docs/helm/helm_install/) for command documentation._

## Uninstall Chart

```console
# Helm
$ helm uninstall [RELEASE_NAME] replika/chart
```

This removes all the Kubernetes components associated with the chart and deletes the release.

_See [helm uninstall](https://helm.sh/docs/helm/helm_uninstall/) for command documentation._

CRDs created by this chart are not removed by default and should be manually cleaned up:

```console
kubectl delete crd replikas.replika.prosimcorp.com
```

## Configuration

See [Customizing the chart before installing](https://helm.sh/docs/intro/using_helm/#customizing-the-chart-before-installing). To see all configurable options with comments:

```console
helm show values replika/chart
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| args[0] | string | `"--health-probe-bind-address=:8081"` |  |
| args[1] | string | `"--metrics-bind-address=127.0.0.1:8080"` |  |
| args[2] | string | `"--leader-elect"` |  |
| args[3] | string | `"--zap-log-level=debug"` |  |
| autoscaling.enabled | bool | `false` |  |
| customResourceDefinitions | list | `[]` |  |
| fullnameOverride | string | `""` |  |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.repository | string | `"prosimcorp/replika"` |  |
| image.tag | string | `"v0.2.4"` |  |
| kubeRbacProxy.args[0] | string | `"--secure-listen-address=0.0.0.0:8443"` |  |
| kubeRbacProxy.args[1] | string | `"--upstream=http://127.0.0.1:8080/"` |  |
| kubeRbacProxy.args[2] | string | `"--logtostderr=true"` |  |
| kubeRbacProxy.args[3] | string | `"--v=0"` |  |
| kubeRbacProxy.image.pullPolicy | string | `"IfNotPresent"` |  |
| kubeRbacProxy.image.repository | string | `"gcr.io/kubebuilder/kube-rbac-proxy"` |  |
| kubeRbacProxy.image.tag | string | `"v0.8.0"` |  |
| kubeRbacProxy.ports[0].port | int | `8443` |  |
| kubeRbacProxy.ports[0].portName | string | `"https"` |  |
| kubeRbacProxy.ports[0].protocol | string | `"TCP"` |  |
| kubeRbacProxy.resources | object | `{}` |  |
| kubeRbacProxy.securityContext | object | `{}` |  |
| kubeRbacProxy.service.enabled | bool | `false` |  |
| nameOverride | string | `""` |  |
| nodeSelector | object | `{}` |  |
| podAnnotations | object | `{}` |  |
| replicaCount | int | `1` |  |
| replikaSources | object | `{}` |  |
| resources | object | `{}` |  |
| securityContext.allowPrivilegeEscalation | bool | `false` |  |
| securityContext.runAsNonRoot | bool | `true` |  |
| serviceAccount.annotations | object | `{}` |  |
| serviceAccount.create | bool | `true` |  |
| serviceAccount.name | string | `""` |  |
| tolerations | list | `[]` |  |

## Examples

### Replicate on all namespaces

When `matchAll` field is `true`, the resource will replicated on all namespaces except in the namespaces which you choice in `excludeFrom` (_optional_) list and `namespace` field.

```yaml
replikaSources:
  # List of sources
  sources:
    - name: sample-traefik-middleware
      group: "traefik.containo.us"
      version: v1alpha1
      kind: Middleware
      namespace: source-namespace
      target:
        namespaces:
          matchAll: true
          excludeFrom:
            - default
      synchronization: "60s"
```

### Replicate on some namespaces

When `matchAll` field is `false`, you **must** declare `replicateIn` field and choice in which namespaces replicate the source resouce.

```yaml
replikaSources:
  # List of sources
  sources: 
    - name: sample-configmap
      group: ""
      version: v1
      kind: ConfigMap
      namespace: source-namespace
      target:
        namespaces:
          replicateIn:
          - destination-namespace
          matchAll: false
      synchronization: "10s"
```
