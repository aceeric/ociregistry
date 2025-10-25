# Quick Start (Kubernetes in three steps)

The chart is hosted on [Artifacthub](https://artifacthub.io/packages/helm/ociregistry/ociregistry). For the basic configuration you can install with no `values.yaml` file.

## 1) Install the chart

```shell
CHARTVER=n.n.n
helm upgrade --install ociregistry oci://quay.io/appzygy/helm-charts/ociregistry\
  --version $CHARTVER\
  --namespace ociregistry\
  --create-namespace
```

By default the chart will create a `NodePort` service on port `31080` in your cluster for `containerd` to mirror to. (This is configurable via a values 
override.) If you recall, a NodePort service enables every node in the cluster to route traffic received on that node and port to the pod bound to that service, regardless of what node is running the pod. We will take advantage of this in the next step.

## 2) Configure `containerd`

Configure containerd in your Kubernetes cluster to mirror **all** image pulls to the pull-through registry. (This has been tested with containerd >= `v1.7.6`):

First, add a `config_path` entry to `/etc/containerd/config.toml` to tell containerd to load all registry mirror configurations from that directory:

```toml
   ...
   [plugins."io.containerd.grpc.v1.cri".registry]
      config_path = "/etc/containerd/certs.d"
   ...
```

Then create a configuration file that tells containerd to pull from the caching pull-through registry server in the cluster. This is an example for `_default` which indicates to containerd that **all** images should be mirrored. Notice that the configuration simply uses `localhost` and the NodePort port value:

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
2. Assuming you installed the caching pull-through OCI registry with the default `NodePort` service option on port `31080`, every host on the cluster will route `localhost:31080` to the Pod running the registry.

The `containerd` daemon _should_ detect the change and re-configure itself. If you believe that's not occurring, then `systemctl restart containerd`.

## 3) Verify

### Tail the logs on the _Ociregistry_ pod:

```shell
kubectl -n ociregistry logs -f -l app.kubernetes.io/name=ociregistry
```

### Observe the startup logs

```text
time="2025-04-24T21:11:41Z" level=info msg="loaded 0 manifest(s) from the file system in 102.136µs"
----------------------------------------------------------------------
OCI Registry: pull-only, pull-through, caching OCI Distribution Server
Version: vZ.Z.Z, build date: 2025-04-23T00:35:28.79Z
Started: 2025-04-24 21:11:41.727569108 +0000 UTC (port 8080)
Running as (uid:gid) 65532:65532
Process id: 1
Tls: none
Command line: /ociregistry/server --config-file /var/ociregistry/config/registry-config.yaml serve
----------------------------------------------------------------------
time="2025-04-24T21:11:41Z" level=info msg="server is running"
```

### Run the `hello-world` container

```shell
kubectl run hello-world --image docker.io/hello-world:latest &&\
  sleep 5s &&\
  kubectl logs hello-world
```

### Result

```text
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

### Observe the **new** Ociregistry log entries

```text
time="2025-04-24T21:13:54Z" level=info msg="pulling manifest from upstream: \"docker.io/library/hello-world:latest\""
time="2025-04-24T21:13:54Z" level=info msg="echo server HEAD:/v2/library/hello-world/manifests/latest?ns=docker.io status=200 latency=501.131286ms host=localhost:31080 ip=10.200.0.232"
time="2025-04-24T21:13:54Z" level=info msg="serving manifest from cache: \"docker.io/library/hello-world@sha256:c41088499908a59aae84b0a49c70e86f4731e588a737f1637e73c8c09d995654\""
time="2025-04-24T21:13:54Z" level=info msg="echo server GET:/v2/library/hello-world/manifests/sha256:c410884999?ns=docker.io status=200 latency=2.267101ms host=localhost:31080 ip=10.200.0.232"
time="2025-04-24T21:13:54Z" level=info msg="pulling manifest from upstream: \"docker.io/library/hello-world@sha256:03b62250a3cb1abd125271d393fc08bf0cc713391eda6b57c02d1ef85efcc25c\""
time="2025-04-24T21:13:55Z" level=info msg="echo server GET:/v2/library/hello-world/manifests/sha256:03b62250a3?ns=docker.io status=200 latency=612.467708ms host=localhost:31080 ip=10.200.0.232"
time="2025-04-24T21:13:55Z" level=info msg="echo server GET:/v2/library/hello-world/blobs/sha256:74cc54e27d?ns=docker.io status=200 latency=234.948µs host=localhost:31080 ip=10.200.0.232"
time="2025-04-24T21:13:55Z" level=info msg="echo server GET:/v2/library/hello-world/blobs/sha256:e6590344b1?ns=docker.io status=200 latency=232.113µs host=localhost:31080 ip=10.200.0.232"
```

### Run the `hello-world` container again with a different name

This time, also specify a pull policy to force `containerd` to pull the image:

```shell
kubectl run hello-world-2 --image docker.io/hello-world:latest\
  --image-pull-policy=Always
```

### Observe new Ociregistry log entries

These entries indicate that the _Ociregistry_ server is serving from the image cache instead of re-pulling from DockerHub:

```text
time="2025-04-24T21:34:35Z" level=info msg="serving manifest from cache: \"docker.io/library/hello-world:latest\""
time="2025-04-24T21:34:35Z" level=info msg="echo server HEAD:/v2/library/hello-world/manifests/latest?ns=docker.io status=200 latency=204.413µs host=localhost:31080 ip=10.200.0.232"
time="2025-04-24T21:34:36Z" level=info msg="serving manifest from cache: \"docker.io/library/hello-world:latest\""
time="2025-04-24T21:34:36Z" level=info msg="echo server HEAD:/v2/library/hello-world/manifests/latest?ns=docker.io status=200 latency=358.099µs host=localhost:31080 ip=10.200.0.232"
time="2025-04-24T21:34:51Z" level=info msg="serving manifest from cache: \"docker.io/library/hello-world:latest\""
time="2025-04-24T21:34:51Z" level=info msg="echo server HEAD:/v2/library/hello-world/manifests/latest?ns=docker.io status=200 latency=313.859µs host=localhost:31080 ip=10.200.0.232"
time="2025-04-24T21:35:09Z" level=info msg="serving manifest from cache: \"docker.io/library/hello-world:latest\""
time="2025-04-24T21:35:09Z" level=info msg="echo server HEAD:/v2/library/hello-world/manifests/latest?ns=docker.io status=200 latency=836.294µs host=localhost:31080 ip=10.200.0.232"
```

## Summary

In this quick start, you Helm-installed _Ociregistry_ in the cluster bound to a NodePort. You configured all containerd instances in the cluster to mirror to the in-cluster registry. You verified that an initial pull causes the _Ociregistry_ server to pull through to the upstream (Docker Hub in this case) and that subsequent image pulls by containerd were served by the _Ociregistry_ server from its cache, avoiding any subsequent round trips to the upstream.
