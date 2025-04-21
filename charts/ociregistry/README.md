# Pull-only, pull-through caching OCI Distribution Server

The chart installs a pull-only, pull-through caching OCI Distribution server. That means:

1. It exclusively provides _pull_ capability.
2. It provides *caching pull-through* capability to any upstream registry: internal, air-gapped, or public; supporting the following types of access: anonymous, basic auth, HTTP, HTTPS, one-way TLS, and mTLS.

This OCI distribution server is intended to satisfy one use case: the need for a Kubernetes caching pull-through registry that enables a k8s cluster to run reliably in an air-gapped network or in a network with intermittent/degraded connectivity to upstream registries.

## Quick Start (three steps)

## 1) Install the chart

```
helm repo add ociregistry https://aceeric.github.io/ociregistry-helm/
helm -n ociregistry install ociregistry ociregistry --create-namespace
```

## 2) Configure `containerd`

Configure containerd in your Kubernetes cluster to mirror **all** image pulls to the pull-through registry. (This has been tested with containerd v1.7.6):

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
cat <<EOF > /etc/containerd/certs.d/_default/hosts.toml
[host."http://localhost:31080"]
  capabilities = ["pull", "resolve"]
  skip_verify = true
EOF
```

**Key Points:**

1. The _resolve_ capability tells containerd that a HEAD request to the server with a manifest will return a manifest digest. The _pull_ capability indicates to containerd that the image can be pulled.
2. Assuming you installed the caching pull-through OCI registry with the default `NodePort` service option on port `31080`, every host on the cluster will route `31080` to the Pod running the registry.

Then `systemctl restart containerd` and then you can confirm visually that is is mirroring by running the following command on a cluster host:

## 3) Verify

While still shelled into the host:

```
crictl pull docker.io/hello-world:latest
```

External to the cluster, tail the logs on the pull-through registry server and you will see the traffic from containerd. Example:

```
echo server HEAD:/v2/hello-world/manifests/latest?ns=docker.io status=200 latency=2.664780196s host=n.n.n.n:8080 ip=n.n.n.n
```

Notice the `?ns=docker.io` query parameter appended to the API call. The pull-through server uses this to determine which upstream registry to get images from.

## More information

More information, including how to configure access to upstream registries for authentication and TLS, as well as additional registry features and capabilities can be found at: https://github.com/aceeric/ociregistry.

## Chart Details

![Version: 1.5.0](https://img.shields.io/badge/Version-1.5.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 1.5.0](https://img.shields.io/badge/AppVersion-1.5.0-informational?style=flat-square)

## Chart Values

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Defines Pod affinity |
| configs | object | See sub-fields | Causes command-line args to be templated into the server Deployment spec to change the default server behavior. |
| configs.configPath | string | `""` | The configPath does not need to be defined if `upstreamConfig` is defined. However, if you specify both, the `configPath` value must match `upstreamConfig.mountPath`. |
| configs.imagePath | string | `""` | Allows overriding the default image path of `/var/lib/ociregistry`. By default the server will use this path for image storage. If you mount the storage at some other path you must change the `configs.imagePath` to match. |
| configs.logLevel | string | `"info"` | specifies the log level. The default log level coded into the binary is `error` but we use `info` here to get started. |
| configs.port | string | `nil` | Allows overriding the default port of 8080. If you change the port, you must also change the `service.port` value to match. |
| configs.pullTimeout | string | `""` | Enables overriding the default timeout of 60000 milliseconds (6 minutes) to pull from an upstream server. |
| fullnameOverride | string | `""` | Overrides the default naming logic that concats the release and chart name. |
| image | object | see sub-fields | Specifies the image. You can populate `tag` or `digest` or both. |
| image.digest | string | `nil` | Specify a digest to use instead of the tag |
| image.pullPolicy | string | `"IfNotPresent"` | The image pull policy |
| image.registry | string | `"quay.io"` | The image registry |
| image.repository | string | `"appzygy/ociregistry"` | The image repository |
| image.tag | string | `"1.5.0"` | The image tag |
| imagePullSecrets | list | `[]` | Supports pullng the image from a registry that requires authentication |
| ingress | object | See sub-fields | Configures an ingress for access to the registry outside the cluster. (Could be used to run the registry in one cluster to cache for multiple other clusters.) |
| nameOverride | string | `""` | Overrides the default naming logic that concats the release and chart name. |
| nodeSelector | object | `{}` | Defines a node selector |
| persistence | object | See sub-fields | Persistence establishes the persistence strategy. For ephemeral storage (i.e. for testing or experimentation) enable the `emptyDir` option. If you have a storage provisioner, enable the persistentVolumeClaim option. The `hostPath` option uses host storage. `emptyDir` is enabled by default. |
| persistence.emptyDir | object | See sub-fields | Implements Empty Dir storage for the server. Suitable for testing and a quick capability evaluation. |
| persistence.emptyDir.enabled | bool | `true` | This is the default option to facilitate a quick start |
| persistence.emptyDir.sizeLimit | string | `"2Gi"` | Provides a size limit to the storage |
| persistence.hostPath | object | See sub-fields | Implements host path storage for the server. Suitable for testing and a quick capability evaluation. |
| persistence.hostPath.enabled | bool | `false` | Host path is disabled by default |
| persistence.hostPath.path | string | `"/var/lib/ociregistry"` | By default the server will use this path for image storage. If you mount the storage at some other path you must change the `configs.imagePath` to match. |
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
| resources | object | `{}` | Specify requests and limits. Manifests are cached in memory to speed response time. Manifests vary greatly in size but - if an average manifest is 3K and your cluster has 100 images this would result in the server using 300K of RAM. |
| securityContext | object | `{}` | Provide any additional deployment security context |
| service | object | See sub-fields | service defines the Service resource. Since the intended use case is for you to configure containerd to mirror to the registry server, the server has to be reachable by containerd running on the host. Therefore the default configuration is to create a NodePort service, which makes the registry available to containerd on each host. If you change the port, you must also specify the `configs.port` above. |
| serviceAccount | object | See sub-fields | Defines the service account configuration |
| serviceAccount.annotations | object | `{}` | Provide any additional annotations you need |
| serviceAccount.automount | bool | `true` | Automounts a token |
| serviceAccount.create | bool | `true` | Creates a service account for the server |
| serviceAccount.name | string | `""` | Overrides the default service name |
| tolerations | list | `[]` | Defines Pod tolerations |
| upstreamConfig | object | `{}` | Supports providing auth and TLS configuration for upstream registries. If not provided, then all upstreams will be tried by the server as anonymous with server cert verification using the OS trust store on the host. The values shows a fictional configuration with all allowed hash keys populated. |
| volumeMounts | list | See sub-fields | Volume Mounts provides the container mount paths. Since this is a caching registry it needs a place to store image data. |
| volumeMounts[0] | object | `{"mountPath":"/var/lib/ociregistry","name":"images","readOnly":false}` | When you select a `persistence` type, a volume will be templated named `images`. By default the server will look in the `mountPath`. If you mount the storage at some other path you must change the `configs.imagePath` to match. |
| volumeMounts[0].mountPath | string | `"/var/lib/ociregistry"` | Shows the default value hard-coded into the server |
| volumes | list | `[]` | Use this to mount other volumes. |

