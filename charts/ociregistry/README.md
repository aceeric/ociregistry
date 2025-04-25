# Pull-only, pull-through, caching OCI Distribution Server

This chart installs a **pull-only**, **pull-through**, **caching** OCI Distribution server. That means:

1. It exclusively provides _pull_ capability. You can't push images to it, it doesn't support the `/v2/_catalog` endpoint, etc. Though you can pre-load it.
2. It provides *caching pull-through* capability to any upstream registry: internal, air-gapped, or public; supporting the following types of access: anonymous, basic auth, HTTP, HTTPS, one-way TLS, and mTLS.

This OCI distribution server is intended to satisfy one use case: the need for a Kubernetes caching pull-through registry that enables a k8s cluster to run reliably in disrupted, disconnected, intermittent and low-bandwidth (DDIL) edge environments. (However, it also nicely mitigates rate-limiting issues when doing local Kubernetes development.)

## Quick Start (three steps)

## 1) Install the chart

```shell
CHARTVER=1.8.1
helm upgrade --install ociregistry oci://quay.io/appzygy/helm-charts/ociregistry\
  --version $CHARTVER\
  --namespace ociregistry\
  --create-namespace\
  --dry-run=server
```

> Remove the `--dry-run=server` arg to actually perform the install.

By default the chart will create a `NodePort` service on port `31080` in your cluster for `containerd` to mirror to. (This is configurable via a values override.)

## 2) Configure `containerd`

Configure containerd in your Kubernetes cluster to mirror **all** image pulls to the pull-through registry. (This has been tested with containerd `v1.7.6`):

First, add a `config_path` entry to `/etc/containerd/config.toml` to tell containerd to load all registry mirror configurations from that directory:

```shell
   ...
   [plugins."io.containerd.grpc.v1.cri".registry]
      config_path = "/etc/containerd/certs.d"
   ...
```

Then create a configuration file that tells containerd to pull from the caching pull-through registry server in the cluster. This is an example for `_default` which indicates to containerd that **all** images should be mirrored:

```shell
mkdir -p /etc/containerd/certs.d/_default && \
cat <<EOF >| /etc/containerd/certs.d/_default/hosts.toml
[host."http://localhost:31080"]
  capabilities = ["pull", "resolve"]
  skip_verify = true
EOF
```

**Key Points:**

1. The _resolve_ capability tells containerd that a HEAD request to the server with a manifest will return a manifest digest. The _pull_ capability indicates to containerd that the image can be pulled.
2. Assuming you installed the caching pull-through OCI registry with the default `NodePort` service option on port `31080`, every host on the cluster will route `31080` to the Pod running the registry.

The `containerd` daemon _should_ detect the change and re-configure itself. If you believe that's not occurring, then `systemctl restart containerd`.

## 3) Verify

### Tail the logs on the pull-through registry pod:

```shell
kubectl -n ociregistry logs -f -l app.kubernetes.io/name=ociregistry
```

### Observe the startup logs

```shell
time="2025-04-24T21:11:41Z" level=info msg="loaded 0 manifest(s) from the file system in 102.136µs"
----------------------------------------------------------------------
OCI Registry: pull-only, pull-through, caching OCI Distribution Server
Version: v1.8.1, build date: 2025-04-23T00:35:28.79Z
Started: 2025-04-24 21:11:41.727569108 +0000 UTC (port 8080)
Running as (uid:gid) 65532:65532
Process id: 1
Command line: /ociregistry/server --config-file /var/ociregistry/config/registry-config.yaml serve
----------------------------------------------------------------------
time="2025-04-24T21:11:41Z" level=info msg="server is running"
```

### Run `hello-world` container

```shell
kubectl run hello-world --image docker.io/hello-world:latest &&\
  sleep 5s &&\
  kubectl logs hello-world
```

### Result

```shell
Hello from Docker!
This message shows that your installation appears to be working correctly.

To generate this message, Docker took the following steps:
 1. The Docker client contacted the Docker daemon.
 2. The Docker daemon pulled the "hello-world" image from the Docker Hub.
    (amd64)
 3. The Docker daemon created a new container from that image which runs the
    executable that produces the output you are currently reading.
```
(remainder redacted for brevity...)

### Observe the **new** ociregistry log entries

```shell
time="2025-04-24T21:13:54Z" level=info msg="pulling manifest from upstream: \"docker.io/library/hello-world:latest\""
time="2025-04-24T21:13:54Z" level=info msg="echo server HEAD:/v2/library/hello-world/manifests/latest?ns=docker.io status=200 latency=501.131286ms host=localhost:31080 ip=10.200.0.232"
time="2025-04-24T21:13:54Z" level=info msg="serving manifest from cache: \"docker.io/library/hello-world@sha256:c41088499908a59aae84b0a49c70e86f4731e588a737f1637e73c8c09d995654\""
time="2025-04-24T21:13:54Z" level=info msg="echo server GET:/v2/library/hello-world/manifests/sha256:c410884999?ns=docker.io status=200 latency=2.267101ms host=localhost:31080 ip=10.200.0.232"
time="2025-04-24T21:13:54Z" level=info msg="pulling manifest from upstream: \"docker.io/library/hello-world@sha256:03b62250a3cb1abd125271d393fc08bf0cc713391eda6b57c02d1ef85efcc25c\""
time="2025-04-24T21:13:55Z" level=info msg="echo server GET:/v2/library/hello-world/manifests/sha256:03b62250a3?ns=docker.io status=200 latency=612.467708ms host=localhost:31080 ip=10.200.0.232"
time="2025-04-24T21:13:55Z" level=info msg="echo server GET:/v2/library/hello-world/blobs/sha256:74cc54e27d?ns=docker.io status=200 latency=234.948µs host=localhost:31080 ip=10.200.0.232"
time="2025-04-24T21:13:55Z" level=info msg="echo server GET:/v2/library/hello-world/blobs/sha256:e6590344b1?ns=docker.io status=200 latency=232.113µs host=localhost:31080 ip=10.200.0.232"
```

### Run `hello-world` Pod again with a different name

This time, also specify a pull policy to force `containerd` to pull the image:

```shell
kubectl run hello-world-2 --image docker.io/hello-world:latest --image-pull-policy=Always
```

### Observe new ociregistry log entries

These entries indicate that the _ociregistry_ server is serving from the image cache instead of re-pulling from Docker Hub:

```shell
time="2025-04-24T21:34:35Z" level=info msg="serving manifest from cache: \"docker.io/library/hello-world:latest\""
time="2025-04-24T21:34:35Z" level=info msg="echo server HEAD:/v2/library/hello-world/manifests/latest?ns=docker.io status=200 latency=204.413µs host=localhost:31080 ip=10.200.0.232"
time="2025-04-24T21:34:36Z" level=info msg="serving manifest from cache: \"docker.io/library/hello-world:latest\""
time="2025-04-24T21:34:36Z" level=info msg="echo server HEAD:/v2/library/hello-world/manifests/latest?ns=docker.io status=200 latency=358.099µs host=localhost:31080 ip=10.200.0.232"
time="2025-04-24T21:34:51Z" level=info msg="serving manifest from cache: \"docker.io/library/hello-world:latest\""
time="2025-04-24T21:34:51Z" level=info msg="echo server HEAD:/v2/library/hello-world/manifests/latest?ns=docker.io status=200 latency=313.859µs host=localhost:31080 ip=10.200.0.232"
time="2025-04-24T21:35:09Z" level=info msg="serving manifest from cache: \"docker.io/library/hello-world:latest\""
time="2025-04-24T21:35:09Z" level=info msg="echo server HEAD:/v2/library/hello-world/manifests/latest?ns=docker.io status=200 latency=836.294µs host=localhost:31080 ip=10.200.0.232"
```

## More information

More information, including how to configure access to upstream registries for authentication and TLS, as well as additional registry features and capabilities can be found at: https://github.com/aceeric/ociregistry.

## Chart Details

![Version: 1.8.1](https://img.shields.io/badge/Version-1.8.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 1.8.1](https://img.shields.io/badge/AppVersion-1.8.1-informational?style=flat-square)

## Chart Values

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Defines Pod affinity |
| fullnameOverride | string | `""` | Overrides the default naming logic. |
| image | object | see sub-fields | Specifies the images. You can populate `tag` or `digest` or both. |
| image.ociregistry.digest | string | `nil` | Specify a digest to use instead of the tag |
| image.ociregistry.pullPolicy | string | `"IfNotPresent"` | The image pull policy |
| image.ociregistry.registry | string | `"quay.io"` | The image registry |
| image.ociregistry.repository | string | `"appzygy/ociregistry"` | The image repository |
| image.ociregistry.tag | string | `"1.8.1"` | The image tag |
| imagePullSecrets | list | `[]` | Supports pulling the image from a registry that requires authentication |
| ingress | object | `enabled: false` | Configures an ingress for access to the registry outside the cluster. (Could be used to run the registry in one cluster to cache for multiple other clusters.) |
| nameOverride | string | `""` | Overrides the default naming logic. |
| nodeSelector | object | `{}` | Defines a node selector |
| persistence | object | See sub-fields | Persistence establishes the persistence strategy. For ephemeral storage (i.e. for testing or experimentation) the `emptyDir` option is enabled by default. If you have a storage provisioner, enable the `persistentVolumeClaim` option. The `hostPath` option uses host storage. |
| persistence.emptyDir | object | See sub-fields | Implements Empty Dir storage for the server. Suitable for testing and a quick capability evaluation. |
| persistence.emptyDir.enabled | bool | `true` | This is the default option to facilitate a quick start |
| persistence.emptyDir.sizeLimit | string | `"2Gi"` | Provides a size limit to the storage |
| persistence.hostPath | object | See sub-fields | Implements host path storage for the server. Suitable for testing and a quick capability evaluation. |
| persistence.hostPath.enabled | bool | `false` | Host path is disabled by default |
| persistence.hostPath.path | string | `"/var/lib/ociregistry"` | By default the server will use this path for image storage. If you mount the storage at some other path you must match it in the `imagePath` value in `serverConfig.config` below. |
| persistence.persistentVolumeClaim | object | See sub-fields | Creates a PVC for persistent storage |
| persistence.persistentVolumeClaim.enabled | bool | `false` | Persistent storage is disabled by default. Set to `true` to enable persistent storage |
| persistence.persistentVolumeClaim.existingClaimName | string | `""` | If you will bind to an existing PVC, specify the name here, otherwise leave the name blank and fill in the `newClaimSpec` hash. |
| persistence.persistentVolumeClaim.newClaimSpec | object | See sub-fields | Supply the parameters for a new PVC |
| persistence.persistentVolumeClaim.newClaimSpec.accessModes | list | `["ReadWriteOnce"]` | Access mode(s) supported by the storage class |
| persistence.persistentVolumeClaim.newClaimSpec.resources | object | `{"requests":{"storage":"2Gi"}}` | Required storage capacity |
| persistence.persistentVolumeClaim.newClaimSpec.selector | object | `{}` | specify any necessary storage selectors. |
| persistence.persistentVolumeClaim.newClaimSpec.storageClassName | string | `""` | Leave the storage class empty to select the default cluster storage class, or specify a class if multiple are available |
| persistence.persistentVolumeClaim.newClaimSpec.volumeMode | string | `"Filesystem"` | Volume mode supported by the storage class |
| podAnnotations | object | `{}` | Provide any additional pod annotations |
| podLabels | object | `{}` | Provide any additional pod labels |
| podSecurityContext | object | `{}` | Provide any additional pod security context |
| resources | object | `{}` | Specify requests and limits. Manifests are cached in memory to speed response time. Manifests vary greatly in size but - if an average manifest is 3K and your cluster has 100 images this would result in the server using 300K of RAM. Blob digests are also cached (only the digests) which consumes about 80 bytes per blob. |
| securityContext | object | `{}` | Provide any additional deployment security context |
| serverConfig | object | See sub-fields | Supports overriding default (built-in) configuration. If you change `serverConfig.config.imagePath` then change `volumeMounts['image'].mountPath` to match. The values shown are the defaults hard-coded in the binary, except for `logLevel` which defaults to `error` and is overridden below as `info`. The explicit values are provided mainly for documentation purposes. OS and arch by default are determined by the system hosting the server. |
| service | object | See sub-fields | service defines the Service resource. To serve inside the cluster for other cluster workloads, specify `NodePort`. To serve behind an ingress for workloads outside the cluster, specify `ClusterIP` |
| serviceAccount | object | See sub-fields | Defines the service account configuration |
| serviceAccount.annotations | object | `{}` | Provide any additional annotations you need |
| serviceAccount.automount | bool | `true` | Automounts a token |
| serviceAccount.create | bool | `true` | Creates a service account for the server |
| serviceAccount.name | string | `""` | Overrides the default service name |
| tolerations | list | `[]` | Defines Pod tolerations |
| volumeMounts | list | See sub-fields | Volume Mounts provides the container mount paths. Since this is a caching registry it needs a place to store image data. |
| volumeMounts[0] | object | `{"mountPath":"/var/lib/ociregistry","name":"images","readOnly":false}` | Specifies  where to mount the storage for the image cache. |
| volumeMounts[0].mountPath | string | `"/var/lib/ociregistry"` | Shows the default value hard-coded into the server unless overridden |
| volumes | list | `[]` | Use this to mount other volumes. |

