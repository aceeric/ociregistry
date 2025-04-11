# Pull-only, Pull-through Caching OCI Distribution Server

This project is a **pull-only**, **pull-through**, **caching** OCI Distribution server. That means:

1. It exclusively provides _pull_ capability. You can't push images to it, it doesn't support the `/v2/_catalog` endpoint, etc. (Though you can pre-load it. More on that below.)
2. It provides *caching pull-through* capability to any upstream registry: internal, air-gapped, or public; supporting the following types of access: anonymous, basic auth, HTTP, HTTPS, one-way TLS, and mTLS.

This OCI distribution server is intended to satisfy one use case: the need for a Kubernetes caching pull-through registry that enables a k8s cluster to run reliably in disrupted, disconnected, intermittent and low-bandwidth (DDIL) edge environments. (However, it also nicely mitigates rate-limiting issues when doing local Kubernetes development.)

The goals of the project are:

1. Implement one use case
2. Be simple and reliable

----

**Table of Contents**

- [Quick Start - Desktop](#quick-start---desktop)
- [Quick Start - Kubernetes](#quick-start---kubernetes)
- [Configuring `containerd`](#configuring-containerd)
- [Configuring the Server](#configuring-the-server)
- [Command Line Options](#command-line-options)
- [Pre-Loading the Server](#pre-loading-the-server)
- [Load and Pre-Load Concurrency](#load-and-pre-load-concurrency)
- [Image Store](#image-store)
- [Pruning the cache](#pruning-the-cache)
- [Design](#design)
- [Image Pull Sequence Diagram](#image-pull-sequence-diagram)
- [Code Structure](#code-structure)
- [REST API Implementation](#rest-api-implementation)
- [Kubernetes Considerations](#kubernetes-considerations)

## Quick Start - Desktop

After git cloning the project:

### Build the Server
```shell
make desktop
```

This command compiles the server and creates a binary called `server` in the `bin` directory relative to the project root.

### Run the Server

You provide an image storage location with the `--image-path` arg. If the directory doesn't exist the server will create it. The default is `/var/lib/ociregistry` but to kick the tires it makes more sense to use the system temp directory. By default the server listens on `8080`. If you have something running that is already bound to that port, specify `--port`. We'll specify it explicitly here with the default value:
```shell
bin/server --image-path /tmp/images --port 8080
```

### Result
```shell
----------------------------------------------------------------------
OCI Registry: pull-only, pull-through, caching OCI Distribution Server
Started: 2024-02-17 20:49:56.516302625 -0500 EST (port 8080)
----------------------------------------------------------------------
```

### In Another Terminal

Curl a manifest list. Note the `ns` query parameter in the URL below which tells the server to go to that upstream if the image isn't already locally cached. This is exactly how `containerd` does it when you configure it to mirror:

```shell
curl localhost:8080/v2/kube-scheduler/manifests/v1.29.1?ns=registry.k8s.io | jq
```

### Result
```json
  "schemaVersion": 2,
  "mediaType": "application/vnd.docker.distribution.manifest.list.v2+json",
  "manifests": [
    {
      "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "size": 2612,
      "digest": "sha256:019d7877d15b45951df939efcb941de9315e8381476814a6b6fdf34fc1bee24c",
      "platform": {
        "architecture": "amd64",
        "os": "linux"
      }
    },
```

### Curl an Image Manifest

Pick the first manifest from the list above - the `amd64/linux` manifest - and curl the manifest by SHA:

```shell
curl localhost:8080/v2/kube-scheduler/manifests/sha256:019d7877d15b45951df939efcb941de9315e8381476814a6b6fdf34fc1bee24c?ns=registry.k8s.io | jq
```

### Inspect the Image Cache

```shell
find /tmp/images
```

### Result
```shell
images
images/blobs
images/blobs/4873874c08efc72e9729683a83ffbb7502ee729e9a5ac097723806ea7fa13517
images/blobs/fcb6f6d2c9986d9cd6a2ea3cc2936e5fc613e09f1af9042329011e43057f3265
images/blobs/9457426d68990df190301d2e20b8450c4f67d7559bdb7ded6c40d41ced6731f7
etc...
images/fat
images/fat/a4afe5bf0eefa56aebe9b754cdcce26c88bebfa89cb12ca73808ba1d701189d7
images/img
images/img/019d7877d15b45951df939efcb941de9315e8381476814a6b6fdf34fc1bee24c
images/pulls
```

The manifest list was saved in: `images/fat/4afe5bf0ee...` and the image manifest was saved in: `images/img/019d7877d1...`.

### Restart the Server and Repeat

This time run with debug logging for more visibility into what the server is doing:

```shell
bin/server --image-path /tmp/images --port 8080 --log-level debug
```

Run the same two curl commands. You will notice that the manifest list and the image manifest are now being returned from cache. You can see this in the logs:

```shell
DEBU[0010] serving manifest from cache: registry.k8s.io/kube-scheduler:v1.29.1
DEBU[0148] serving manifest from cache: registry.k8s.io/kube-scheduler@sha256:019d7877...
```

## Quick Start - Kubernetes

Install from ArtifactHub: https://artifacthub.io/packages/helm/ociregistry/ociregistry.

> See [Kubernetes considerations](#kubernetes-considerations) below in this document for important things to be aware of to support your edge Kubernetes cluster.

## Configuring `containerd`

This section shows how to configure containerd in your Kubernetes cluster to mirror **all** image pulls to the pull-through registry. This has been tested with containerd >= v1.7.x:

Add a `config_path` entry to `/etc/containerd/config.toml` to tell containerd to load all registry mirror configurations from that directory:

```shell
   ...
   [plugins."io.containerd.grpc.v1.cri".registry]
      config_path = "/etc/containerd/certs.d"
   ...
```

Then create a configuration directory and file that tells containerd to pull from the caching pull-through registry server. This is an example for `_default` which indicates that **all** images should be mirrored. The file is `/etc/containerd/certs.d/_default/hosts.toml`. In this example, the caching pull-through registry server is running on `192.168.0.49:8080`:

```shell
[host."http://192.168.0.49:8080"]
  capabilities = ["pull", "resolve"]
  skip_verify = true
```

The _resolve_ capability tells containerd that a HEAD request to the server with a manifest will return a manifest digest. The _pull_ capability indicates to containerd that the image can be pulled.

After restarting containerd, you can confirm visually that containerd is mirroring by running the following command on a cluster host:

```shell
crictl pull quay.io/appzygy/ociregistry:1.3.0
```

Enable `debug` logging on the pull-through registry server and you will see the traffic from containerd. Example:

```shell
echo server HEAD:/v2/appzygy/ociregistry/manifests/1.3.0?ns=quay.io status=200 latency=2.664780196s host=192.168.0.49:8080 ip=192.168.0.49
```

Notice the `?ns=quay.io` query parameter appended to the API call. This query parameter is added by containerd. The pull-through server uses this to determine which upstream registry to get images from.

## Configuring the Server

The OCI Distribution server may need configuration information to connect to upstream registries. If run with no upstream registry config, it will attempt anonymous plain HTTP access. Many OCI Distribution servers will reject HTTP and fail over to HTTPS. Then you're in the realm of TLS and PKI. Some servers require auth as well. To address all of these concerns the OCI Distribution server accepts an optional command line parameter `--config-path` which identifies a configuration file in the following format:

```yaml
- name: upstream one
  description: foo
  auth: {}
  tls: {}
- name: upstream two
  description: bar
  auth: {}
  tls: {}
- etc...
```

The configuration file is a yaml list of upstream registry entries. Each entry supports the following configuration structure:

```yaml
- name: my-upstream (or my-upstream:PORT)
  description: Something that makes sense to you (or omit it - it is optional)
  auth:
    user: theuser
    password: thepass
  tls:
    ca: /my/ca.crt
    cert: /my/client.cert
    key: /my/client.key
    insecure_skip_verify: true/false
```

The `auth` section implements basic auth, just like your `~/.docker/config.json` file.

The `tls` section can implement multiple scenarios:

1. One-way insecure TLS, in which client certs are not provided to the remote, and the remote server cert is not validated:

   ```yaml
   tls:
     insecure_skip_verify: true
   ```

2. One-way **secure** TLS, in which client certs are not provided to the remote, and the remote server cert **is** validated using the OS trust store:

   ```yaml
   tls:
     insecure_skip_verify: false (or simply omit since it defaults to false)
   ```

3. One-way **secure** TLS, in which client certs are not provided to the remote, and the remote server cert is validate using a **provided** CA cert:

   ```yaml
   tls:
     ca: /my/ca.crt
   ```

4. mTLS (client certs are provided to the remote):

   ```yaml
   tls:
     cert: /my/client.cert
     key: /my/client.key
   ```
   mTLS can be implemented **with** and **without** remote server cert validation as described above in the various one-way TLS scenarios. Examples:

   ```yaml
   - name foo.bar.1.io
     description: mTLS, don't verify server cert
     tls:
       cert: /my/client.cert
       key: /my/client.key
       insecure_skip_verify: true
   - name foo.bar.2.io
     description: mTLS, verify server cert from OS trust store
     tls:
       cert: /my/client.cert
       key: /my/client.key
       insecure_skip_verify: false
   - name foo.bar.3.io
     description: mTLS, verify server cert from provided CA
     tls:
       cert: /my/client.cert
       key: /my/client.key
       ca: /remote/ca.crt
       insecure_skip_verify: false
   ```

## Command Line Options

The following options are supported:

### Server Options

| Option | Default | Meaning |
|-|-|-|
| `--preload-images` | n/a | Loads images enumerated in the specified file into cache at startup and then continues to serve. (See _Pre-Loading the Server_ below) |
| `--port`| 8080 | Server port. E.g. `crane pull localhost:8080/foo:v1.2.3 foo.tar` |
| `--always-pull-latest` | false | Causes the server to **always** pull from the upstream whenever the `latest` tag is specified for an image. |

### CLI Options

| Option | Default | Meaning |
|-|-|-|
| `--dry-run` | n/a | See _Pruning the cache_ below. |
| `--load-images` | n/a | Loads images enumerated in the specified file into cache and then exits. (See _Pre-Loading the Server_ below.) |
| `--list-cache` | n/a | Lists the cached images and exits. See _Listing the image cache_ below. |
| `--prune` | n/a | See _Pruning the cache_ below. |
| `--prune-before` | n/a | See _Pruning the cache_ below. |
| `--version` | n/a | Displays the version and exits |

### Common Options

| Option | Default | Meaning |
|-|-|-|
| `--image-path`  | /var/lib/ociregistry | The root directory of the image and metadata store |
| `--concurrent`  | 1 | The number of concurrent goroutines for `--load-images` and `--preload-images` |
| `--log-level`   | error | Valid values: trace, debug, info, warn, error |
| `--config-path` | Empty | Path and file providing remote registry auth and TLS config formatted as described above. If empty then every upstream will be tried with anonymous HTTP access failing over to 1-way HTTPS using the OS Trust store to validate the remote registry certs. (I.e. works fine for `docker.io`) |
| `--pull-timeout`| 60000 (one minute)   | Time in milliseconds to wait for a pull to complete from an upstream distribution server |
| `--arch`        | n/a | Architecture - used with `--load-images` and `--preload-images` |
| `--os`          | n/a | OS - used with `--load-images` and `--preload-images` |

## Listing the image cache

The `--list-cache` option lists all the image manifests in the cache.

The output has three columns:

1. The manifest URL, like `docker.io/hello-world:latest`
2. A literal value `list` or `image` indicating the manifest type
3. The create date of the corresponding image manifest from the file system. E.g.:

E.g.:
```shell
docker.io/library/hello-world:latest              list   2024-03-01T22:17:30
docker.io/library/hello-world@sha256:e2fc4e50...  image  2024-03-01T22:17:30
```

In the example, the tagged item is the manifest _list_ (of a multi-platform image), and the item with the digest is the image manifest. To filter, sort and format, use the Linux `grep`, `sort` and `column` utils. Examples are given below:

### Sort by create date, most recent on top

```shell
bin/server --image-path /var/lib/ociregistry/images --list-cache\
  | column -t | sort -k3 -r | head
```

### Sort by URL

```shell
bin/server --image-path /var/lib/ociregistry/images --list-cache\
  | column -t | sort
```

### Find all the `kube-scheduler` images

```shell
bin/server --image-path /var/lib/ociregistry/images --list-cache\
  | grep kube-scheduler | column -t
```

### List everything cached on a given date

```shell
bin/server --image-path /var/lib/ociregistry/images --list-cache\
  | grep 2024-03-01 | sort | column -t
```

## Pre-Loading the Server

Pre-loading supports the air-gapped use case of populating the registry in a connected environment, and then moving it into an air-gapped environment. The registry normally runs as a service. But  you can also run it as a CLI to pre-load itself and you can start it with the `--preload-images` arg to pre-load images as part of starting up when running as a service or a k8s workload. To do this, you create a file with a list of image references. Example:

```shell
cat <<EOF >| imagelist
quay.io/jetstack/cert-manager-cainjector:v1.11.2
quay.io/jetstack/cert-manager-controller:v1.11.2
quay.io/jetstack/cert-manager-webhook:v1.11.2
registry.k8s.io/metrics-server/metrics-server:v0.6.2
registry.k8s.io/ingress-nginx/controller:v1.8.1
registry.k8s.io/pause:3.8
docker.io/kubernetesui/dashboard-api:v1.0.0
docker.io/kubernetesui/metrics-scraper:v1.0.9
docker.io/kubernetesui/dashboard-web:v1.0.0
EOF
```

Once you've configured your image list file, then:
```shell
bin/server --image-path=/var/ociregistry/images --log-level=info --load-images=$PWD/imagelist --arch=amd64 --os=linux
```

The registry executable will populate the cache and then exit.

Or, do the same thing as a startup task and leave the server running to serve the loaded images:
```shell
bin/server --image-path=/var/ociregistry/images --log-level=info --preload-images=$PWD/imagelist --arch=amd64 --os=linux
```

In the second example, the server populates the cache as a startup task and then continues to run and serve images.

## Load and Pre-Load Concurrency

The server supports concurrency on loading and pre-loading. By default, only a single image at a time is loaded. In other words omitting the `--concurrent` arg is the same as specifying `--concurrent=1`. One thing to be aware of is the balance between concurrency and network utilization. Using a large number of goroutines also requires increasing the pull timeout since the concurrency increases network utilization. For example in desktop testing, the following command has been tested without experiencing image pull timeouts:
```shell
bin/server --log-level=debug --image-path=/tmp/frobozz\
  --load-images=./hack/image-list\
  --pull-timeout=200000\
  --concurrent=50
```

The example above runs 50 concurrent goroutines but needs to increase the timeout to 200000 milliseconds to consistently avoid timeouts. Pull timeouts are logged as an error.

## Image Store

The image store is persisted to the file system. This includes blobs and manifests. Let's say you run the server with `--image-path=/var/ociregistry/images`, which is the default. Then:

```shell
/var/ociregistry/images
├── blobs
├── fat
├── img
└── pulls
```

1. `blobs` are where the blobs are stored
2. `fat` is where the fat manifests are stored: the manifests with lists of image manifests
3. `img` stores the image manifests
3. `pulls` is temp storage for image downloads that should be empty unless a pull is in progress. (If a pull times out - the corrupted tar remains in this directory.)

Manifests are all stored by digest. When the server starts it loads everything into an in-memory representation. Each new pull through the server while it is running updates both the in-memory representation of the image store as well as the persistent state on the file system.

The program uses a data structure called a `ManifestHolder` to hold all the image metadata and the actual manifest from the upstream registry. These are simply serialized to the file system as JSON. (So you can find and inspect them if needed for troubleshooting with `grep`, `cat`, and `jq`.)

## Pruning the cache

The compiled binary can be used as a CLI to prune the cache.

> The server must be stopped while pruning because the CLI only manipulates the file system, not the in-memory representation of the cache.

Pruning removes manifest lists, manifests, and possibly blobs (more on blobs below.) Three options support pruning:

### Dry Run

It is strongly recommended to use the `--dry-run` arg to develop your pruning expression. Then remove `--dry-run` to actually prune the cache. When `--dry-run` is specified, the CLI shows you exactly what will be pruned but does not actually modify the file system.

### By Pattern

The `--prune` option accepts a single parameter consisting of one or more patterns separated by commas. The patterns are Golang regular expressions as documented in the [regexp/syntax](https://pkg.go.dev/regexp/syntax) package documentation. The expressions on the command line are passed _directly_ to the Golang `regex` parser _as received from the shell_, and are matched to manifest URLs. As such, shell expansion and escaping must be taken into consideration. Simplicity wins the day here. Examples:

```shell
bin/server --image-path /var/lib/ociregistry/images --dry-run\
  --prune kubernetesui/dashboard:v2.7.0
bin/server --image-path /var/lib/ociregistry/images --dry-run\
  --prune docker.io
bin/server --image-path /var/lib/ociregistry/images --dry-run\
  --prune curl,cilium
```

### By Date/time

The `--prune-before` option accepts a single parameter consisting of a local date/time in the form `YYYY-MM-DDTHH:MM:SS`. All manifests created **before** that time stamp will be selected. Example:

```shell
bin/server --image-path /var/lib/ociregistry/images --dry-run\
  --prune-before 2024-03-01T22:17:31
```

The intended workflow is to use the CLI with `--list-cache` and sorting as described above to determine desired a cutoff date and then to use that date as an input to the `--prune-before` arg. For example, suppose a number of images were created on and earlier than `2024-03-01T22:17:30`. Then you would specify that time plus one second. E.g.: `--prune-before 2024-03-01T22:17:31`.

### Important to know about pruning

Generally, but not always, image list manifests have tags, and image manifests have digests. This is because in most cases, upstream images are multi-architecture. For example, this command specifies a tag:
```shell
bin/server --image-path /var/lib/ociregistry/images --dry-run --prune calico/typha:v3.27.0
```

In this case, on a Linux/amd64 machine running the server the CLI will find **two** manifests:
```shell
docker.io/calico/typha:v3.27.0
docker.io/calico/typha@sha256:eca01eab...
```

The first is a multi-arch image list manifest, and the second is the image manifest matching the OS and Architecture that was selected for download. In all cases, only image manifests reference blobs. If your search finds only an image list manifest, the CLI logic will **also** look for cached image manifests (and associated blobs) for the specified image list manifest since that's probably the desired behavior. (The blobs consume the storage.)

### Blob removal

Internally, the CLI begins by building a blob list with ref counts. As each image manifest is removed its referenced blobs have their count decremented. After all manifests are removed, any blob with zero refs is also removed. Removing an image manifest therefore won't remove blobs that are still referenced by un-pruned manifests.

## Design

The following image describes the design of the project:

![design](resources/design.jpg)

Narrative:

1. A client (in this case - containerd) initiates an image pull. The image pull consists of a series of REST API calls. The API calls are handled by the server REST API, which implements a portion of the [OCI Distribution Spec](https://github.com/opencontainers/distribution-spec).
2. The API is just a veneer that delegates to the server implementation.
3. The server checks the local cache and if the image is in cache it is immediately returned from cache.
4. If the image is not in cache, the server calls the embedded [Google Crane](https://github.com/google/go-containerregistry/blob/main/cmd/crane/doc/crane.md) code to pull the image from the upstream registry. The server knows which upstream to pull from because containerd appends a query parameter (e.g. `?ns=registry.k8s.io`) to each API call.
5. The Google Crane code pulls the image from the upstream registry and returns it to the server.
6. The server adds the image to cache and returns the image to the caller from the newly updated cache.

## Image Pull Sequence Diagram

By way of background, a typical image pull sequence is:

![design](resources/pull-seq-diagram.png)

To support this, the server caches both the fat manifest and the image manifest. (Two manifests for every one pull.) The pre-loader does the same so you need to provide the tags or digest of the fat manifest in your list.

>  **Gotcha**: If you cache a fat manifest by digest and later run a workload in an air-gapped environment that attempts to get the fat manifest by tag, the registry will not know the tag and so will not be able to provide that image from cache.

The pre-loader logic is similar to the client pull logic:

1. Get the fat manifest by tag from the upstream registry and cache it
2. Pick the digest from the image manifest list in the fat manifest that matches the requested architecture and OS
3. Get the image manifest by digest and the blobs and cache them.

## Code Structure

The source code is organized as shown:
```shell
project root
├── api
├── bin
├── cmd
├── impl
│   ├── extractor
│   ├── globals
│   ├── helpers
│   ├── memcache
│   ├── preload
│   ├── pullrequest
│   ├── serialize
│   ├── upstream
│   │   ├── v1oci
│   │   └── v2docker
│   ├── handlers.go
│   └── ociregistry.go
└── mock
```

| Package | Description |
|-|-|
| `api`  | Mostly generated by `oapi-codegen`. |
| `bin`  | Has the compiled server after `make desktop`. |
| `cmd`  | Entry point. |
| `impl` | Has the implementation of the server. |
| `impl/extractor` | Extracts blobs from downloaded image tarballs. |
| `impl/globals` | Globals and the logging implementation (uses [Logrus](https://github.com/sirupsen/logrus)). |
| `impl/helpers` | Helpers. |
| `impl/memcache` | The in-memory representation of the image metadata. If a "typical" image manifest is about 3K, and two manifests are cached per image then a cache with 100 images would consume 3000 x 2 x 100 bytes, or 600K of RAM. (The blobs are not stored in memory.) |
| `impl/preload` | Implements the pre-load capability. |
| `impl/pullrequest` | Abstracts an image pull. |
| `impl/serialize` | Reads/writes from/to the file system. |
| `impl/upstream` | Talks to the upstream registries, like Docker Hub, Quay, etc. |
| `impl/handlers.go` | Has the code for the subset of the OCI Distribution Server API spec that the server implements. |
| `impl/ociregistry.go` | A veneer that the embedded [Echo](https://echo.labstack.com/) server calls that simply delegates to `impl/handlers.go`. See the next section - _REST API Implementation_ for some details on the REST API. |
| `mock` | Runs a mock OCI Distribution server used by the unit tests. |

## REST API Implementation

# TODO https://github.com/oapi-codegen/oapi-codegen#action-required-the-repository-for-this-project-has-changed

go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest


The OCI Distribution API is built by first creating an Open API spec using Swagger. See `ociregistry.yaml` in the project root. Then the [oapi-codegen](https://github.com/deepmap/oapi-codegen) tool is used to generate the API code and the Model code using configuration in the `api` directory. This approach was modeled after the OAPI-Codegen [Petstore](https://github.com/deepmap/oapi-codegen/tree/master/examples/petstore-expanded/echo) example.

The key components of the API scaffolding supported by OAPI-Codegen are shown below:

```shell
├── api
│   ├── models
│   │   └──models.gen.go   (generated)
│   ├── models.cfg.yaml    (modeled from pet store)
│   ├── ociregistry.gen.go (generated)
│   └── server.cfg.yaml    (modeled from pet store)
├── cmd
│   └── ociregistry.go     (this is the server - which embeds the Echo server)
└── ociregistry.yaml       (the openapi spec built with swagger)
```

I elected to use the [Echo](https://echo.labstack.com/) option to run the API.

## Kubernetes Considerations

As stated above, one main objective of the project is to support running Kubernetes in DDIL edge environments. To accomplish this, you will likely adopt one of two approaches:

1. Stand the cluster up in a connected environment, pre-load the distribution server with all required images, then disconnect and ship the cluster to its edge location.
2. Ship the cluster to its edge location and pre-load the distribution server there using stable comms. If comms are later degraded or lost then the required images remain cached.

There are two general ways to run the distribution server in support of this objective. These are now discussed.

### As a Kubernetes Workload

Using the helm chart described above in this guide you can install the server as a Kubernetes `Deployment`, and configure containerd on each Node to access the distribution server on a `NodePort`. In order to make this work, two things are needed:

1. The cluster needs persistent storage that is redundant and reliable. This means that when you down the cluster, ship it, and start it up at the edge, the persistent storage has to come up with the image cache intact.
2. You need a tarball of the distribution server image available in the edge environment for the following reason: when you start the cluster for the first time, if containerd has been configured to pull from the distribution server, the distribution server image may not be in cache and so that Pod won't start. This is a classic deadlock scenario. Therefore your cluster startup procedure will be to start Kubernetes and then load the distribution server image tarball into each cluster's containerd cache. Now Kubernetes can start the distribution server workload which in turn can serve images from its cache. Ideally your Kubernetes distribution of choice will support the ability to pre-load containerd from a image tarballs at a configuration file system location.

### As a `systemd` Service

You can also run the distribution server as a systemd service on one of the cluster nodes. This avoids the deadlock scenario associated with running the distribution server as a Kubernetes workload because the service will come up when the cluster instances are started and will therefore immediately be available to serve cached images.

The risk is that the image cache only exists on one cluster instance. If this instance is lost, then the cluster is down until comms are re-established. This can be mitigated by replicating the cache across multiple cluster nodes. There are multiple tools available to support this. For example [Syncthing](https://syncthing.net/) could be run on each node to ensure that each node has a full copy of the image cache. If the node running the distribution server is lost then the nodes can have their containerd configuration modified to point to any one of the other nodes, and the distribution server systemd service can be started on that node.

