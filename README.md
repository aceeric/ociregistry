# Pull-only, Pull-through Caching OCI Distribution Server

This project is a **pull-only**, **pull-through**, **caching** OCI Distribution server. That means:

1. It exclusively provides _pull_ capability. You can't push images to it, it doesn't support the `/v2/_catalog` endpoint, etc.
2. It provides *caching pull-through* capability to any upstream registry: internal, air-gapped, or public; supporting the following types of access: anonymous, basic auth, HTTP, HTTPS, one-way TLS, and mTLS.

This OCI distribution server is intended to satisfy one use case: the need for a Kubernetes caching pull-through registry that enables a k8s cluster to run reliably in an air-gapped network or in a network with intermittent/degraded connectivity to upstream registries. (However, it also nicely mitigates rate-limiting issues when doing local Kubernetes development.)

The goals of the project are:

1. Implement one use case
2. Be simple

## Quick Start 1 - on your desktop

After git cloning the project:

### Build the server
```
make desktop
```

This command compiles the server and creates a binary called `server` in the `bin` directory relative to the project root.

### Run the server

You provide an image storage location with the `--image-path` arg
```
mkdir /tmp/images && bin/server --image-path /tmp/images
```

### Result
```
----------------------------------------------------------------------
OCI Registry: pull-only, pull-through, caching OCI Distribution Server
Started: 2024-02-17 20:49:56.516302625 -0500 EST (port 8080)
----------------------------------------------------------------------
```

### In another terminal

Curl a manifest list. Note the `ns` query parameter in the URL which tells the server to go to that upstream if the image isn't already locally cached (this is exactly how `containerd` does it):

```
curl localhost:8080/v2/kube-scheduler/manifests/v1.29.1?ns=registry.k8s.io | jq
```

### Result
```
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
    etc...
```

### Curl an image manifest

Pick the first manifest from the list above - the `amd64/linux` manifest:

```
curl localhost:8080/v2/kube-scheduler/manifests/sha256:019d7877d15b45951df939efcb941de9315e8381476814a6b6fdf34fc1bee24c?ns=registry.k8s.io | jq
```

### Inspect the files created by the two curl calls

```
find /tmp/images
```

### Result:
```images
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

### Stop and restart the server and repeat

You will notice that the manifest list and the image manifest are now being returned from cache.

## Quick Start 2 - in your Kubernetes cluster

Install from ArtifactHub: https://artifacthub.io/packages/helm/ociregistry/ociregistry

## Design

The following image describes the design:

![design](resources/design.jpg)

Narrative:

1. A client initiates an image pull. In this case: containerd. The image pull consists of a series of REST API calls.
2. The API calls are handled by the REST API, which implements a portion of the [OCI Distribution Spec](https://github.com/opencontainers/distribution-spec). The API is just a veneer that delegates to the server implementation.
3. The server checks the local cache and if the image is in cache it is returned from cache.
4. If the image is not in cache, the server calls embedded [Google Crane](https://github.com/google/go-containerregistry/blob/main/cmd/crane/doc/crane.md) Go code to pull the image from the upstream registry. The way the server knows which upstream to pull from is: containerd appends a query parameter to each API call. (More on this below.)
5. The Google Crane code pulls the image from the upstream registry and returns it to the server.
6. The server adds the image to cache for the next pull, and returns the image to the caller.


## API Implementation

The OCI Distribution API is built by first creating an Open API spec using Swagger. See `ociregistry.yaml` in the project root. Then the [oapi-codegen](https://github.com/deepmap/oapi-codegen) tool is used to generate the API code and model code using configuration in the `api` directory. This approach was modeled after the OAPI-Codegen [Petstore](https://github.com/deepmap/oapi-codegen/tree/master/examples/petstore-expanded/echo) example.

The key components of the API scaffolding supported by OAPI-Codegen are shown below:

```shell
​```
├── api
│   ├── models
│   │   └──models.gen.go (generated)
│   ├── models.cfg.yaml (modeled from pet store)
│   ├── ociregistry.gen.go (generated)
│   └── server.cfg.yaml (modeled from pet store)
├── cmd
│   └── ociregistry.go (this is the server - which embeds the Echo server)
└── ociregistry.yaml (the openapi spec built with swagger)
​```
```

I elected to use the [Echo](https://echo.labstack.com/) option to run the API.

## Configuring `containerd`

The following shows how to configure containerd in your Kubernetes cluster to mirror **all** image pulls to the pull-through registry. This has been tested with containerd v1.7.6:

Add a `config_path` entry to `/etc/containerd/config.toml` to tell containerd to load all registry mirror configurations from that directory:

```shell
   ...
   [plugins."io.containerd.grpc.v1.cri".registry]
      config_path = "/etc/containerd/certs.d"
   ...
```

Then create a configuration directory and file that tells containerd to pull from the caching pull-through registry server. This is an example for `_default_` which indicates that **all** images should be mirrored. The file is `/etc/containerd/certs.d/_default/hosts.toml`. In this example, the caching pull-through registry server is running on `192.168.0.49:8080`:

```shell
[host."http://192.168.0.49:8080"]
  capabilities = ["pull", "resolve"]
  skip_verify = true
```

The _resolve_ capability tells containerd that a HEAD request to the server with a manifest will return a manifest digest. The _pull_ capability indicates to containerd that the image can be pulled.

After restarting containerd, you can confirm visually that is is mirroring by running the following command on a cluster host:

```
crictl pull quay.io/appzygy/ociregistry:1.3.0
```

Enable `debug` logging on the pull-through registry server and you will see the traffic from containerd. Example:

```
echo server HEAD:/v2/appzygy/ociregistry/manifests/1.3.0?ns=quay.io status=200 latency=2.664780196s host=192.168.0.49:8080 ip=192.168.0.49
```

Notice the `?ns=quay.io` query parameter appended to the API call. The pull-through server uses this to determine which upstream registry to get images from.

## Configuring the OCI Registry Server

The OCI Registry server may need configuration information to connect to upstream registries. If run with no upstream registry config, it will attempt anonymous plain HTTP access. Many OCI Distribution servers will reject HTTP and fail over to HTTPS. Then you're in the realm of TLS and PKI. Some servers require auth as well. To address all of these concerns the OCI Registry server accepts an optional command line parameter `--config-path` which identifies a configuration file in the following format:

```
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

```
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

   ```
   tls:
     insecure_skip_verify: true
   ```

2. One-way **secure** TLS, in which client certs are not provided to the remote, and the remote server cert **is** validated using the OS trust store:

   ```
   tls:
     insecure_skip_verify: false (or simply omit since it defaults to false)
   ```

3. One-way **secure** TLS, in which client certs are not provided to the remote, and the remote server cert is validate using a **provided** CA cert:

   ```
   tls:
     ca: /my/ca.crt
   ```

4. mTLS (client certs are provided to the remote):

   ```
   tls:
     cert: /my/client.cert
     key: /my/client.key
   ```
mTLS can be implemented **with** and **without** remote server cert validation as described above in the various one-way TLS scenarios. Examples:

   ```
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

## Command line options

The following options are supported:

### To run as a server

| Option | Default | Meaning |
|-|-|-|
| `--preload-images` | n/a | Loads images enumerated in the specified file into cache at startup and then continues to serve. (See _Pre-loading the registry_ below) |
| `--port`| 8080 | Server port. E.g. `crane pull localhost:8080/foo:v1.2.3 foo.tar` |

### To run as a CLI

| Option | Default | Meaning |
|-|-|-|
| `--load-images` | n/a | Loads images enumerated in the specified file into cache and then exits. (See _Pre-loading the registry_ below) |
| `--list-cache` | n/a | Lists the cached images and exits |
| `--version` | n/a | Displays the version and exits |

### Common

| Option | Default | Meaning |
|-|-|-|
| `--image-path`  | /var/lib/ociregistry | The root directory of the image and metadata store |
| `--log-level`   | error | Valid values: trace, debug, info, warn, error |
| `--config-path` | Empty | Path a file providing remote registry auth and TLS config. If empty then every upstream will be tried with anonymous HTTP access failing over to 1-way HTTPS using the OS Trust store to validate the remote registry certs. |
| `--pull-timeout`| 60000 (one minute)   | Time in milliseconds to wait for an upstream registry |
| `--arch`        | n/a | used with `--load-images` and `--preload-images` |
| `--os`          | n/a | used with `--load-images` and `--preload-images` |


## Pre-loading the registry

Pre-loading supports the air-gapped use case of populating the registry in a connected environment, and then moving it into an air-gapped environment. The registry normally runs as a service. But  you can also run it as a CLI to pre-load itself and you can start it with the `--preload-images` arg to pre-load images as part of starting up when running as a service or a k8s workload. To do this, you create a file with a list of image references. Example:

```
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

(You can also use the `@sha256:nnn` ref format instead in addition to the `:tag` format.) The key point here is to specify the _fat_ manifest in the list. (When you pull by tag, you get the fat manifest.) Pullers implement the following sequence:

1. HEAD the manifest by tag
2. Receive digest of the manifest in a response header (or Not Found)
3. GET the same (fat) manifest by digest
4. Pick an image manifest digest from the list of manifests in the _fat_ manifest received in step 2
5. GET the image manifest by digest
6. GET the blobs for the image

To support this, the registry server caches both the fat manifest and the image manifest. (Two manifests for every one pull.) The pre-loader does the same so you need to provide the tags or digest of the fat manifest in your list. You should understand though that - if you cache a fat manifest by digest and later run a workload in an air-gapped environment that attempts to get the fat manifest by tag, the registry will not know the tag and so will not be able to provide that image.

The pre-loader logic is similar to the client pull logic:

1. Get the fat manifest by tag from the upstream registry and cache it
2. Pick the digest from the image manifest list in the fat manifest that matches the requested architecture and os
3. Get the image manifest by digest and the blobs and cache them.

Once you've configured your image list file, then:
```
bin/server --image-path=/var/ociregistry/images --log-level=info --load-images=$PWD/imagelist --arch=amd64 --os=linux
```

The registry executable will populate the cache and then exit. Or:

```
bin/server --image-path=/var/ociregistry/images --log-level=info --preload-images=$PWD/imagelist --arch=amd64 --os=linux
```

In the example above, the server will populate the cache as a startup activity and then continue to run and serve images.

## File system structure

State is exclusively persisted to the file system. Let's say you run the server with `--image-path=/var/ociregistry/images`. Then:

```
.../images
├── blobs
├── fat
├── img
└── pulls
```

1. `blobs` are where the blobs are stored
2. `fat` is where the fat manifests are stored: the manifests with lists of image manifests
3. `img` stores the image manifests
3. `pulls` is temp storage for image downloads that should be empty unless a pull is in progress

Manifests are all stored by digest. The program uses a data structure called a `ManifestHolder` to hold all the image metadata and the actual manifest from the upstream registry. These are simply serialized to the file system as JSON. When the server starts it loads everything into an in-memory representation. Each new pull through the server while it is running updates both the in-memory representation of the image store as well as the persistent state on the file system.

## Code structure

```
├── api
│   └── models
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
│   └── upstream
│       ├── v1oci
│       └── v2docker
└── mock
```

1. `api` is mostly generated by `oapi-codegen`.
2. `bin` has the compiled server
3. `cmd` is the entry point
4. `impl` has the implementation of the server
5. `mock` runs a mock OCI Distribution server for unit testing
