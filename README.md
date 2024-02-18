# EXPERIMENTAL Pull-only Pull-through Caching OCI Distribution Server

This project is a **pull-only**, **pull-through**, **caching** OCI Distribution server. That means:

1. It exclusively provides _pull_ capability. You can't push images to it, it doesn't support the `/v2/_catalog` endpoint, etc.
2. It provides *caching pull-through* capability to any upstream registry: internal, air-gapped, or public; supporting the following types of access: anonymous, basic auth, HTTP, HTTPS, one-way TLS, and mTLS.

> This is a POC. As such it is very rough. I'll be cleaning it up over time. (See the TODO file)

This distribution server is intended to satisfy **one** use case: the need for an in-cluster Kubernetes caching pull-through registry that enables the k8s cluster to run reliably in a network context with no-, intermittent-, or low latency connectivity to upstream registries - or - an environment where the upstream registries serving the k8s cluster have less than 5 nines availability.

The goals of the project are:

1. Implement one use case
2. Be simple

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

## Configuring `containerd`

The following snippet shows how to configure `containerd` in your Kubernetes cluster to mirror **all** image pulls to the pull-through registry:

Add a `config_path` entry to `/etc/containerd/config.toml` to tell `containerd` to load all registry mirror configurations from that directory:

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

## Configuring the OCI Registry Server

The OCI Registry server may need configuration information to connect to upstream registries. By default, it will attempt anonymous plain HTTP access. Many OCI Distribution servers will reject HTTP and fail over to HTTPS. Then you're in the realm of TLS and PKI. Some servers require auth as well. To address all of these concerns the OCI Registry server accepts an optional command line parameter `--config-path` which identifies a configuration file in the following format:

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
  description: Something that makes sense to you (or omit it - its optional)
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

| Option        | Default              | Meaning                                                      |
| ------------- | -------------------- | ------------------------------------------------------------ |
| --log-level   | error                | Valid values: debug, info, warn, error                       |
| --image-path  | /var/lib/ociregistry | The root directory of the image store                        |
| --config-path | Empty                | Path a file providing remote registry auth and TLS config. If empty then every upstream will be tried with anonymous HTTP access failing over to HTTPS using the OS Trust store to validate the remote registry. |
| --port        | 8080                 | Server port. E.g. `crane pull localhost:8080 foo.tar`        |

## Quick Start

### Build the server
```
make
```

### Run the server
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

### Curl a manifest list
```
curl localhost:8080/v2/kube-scheduler/manifests/v1.29.1?ns=registry.k8s.io | jq
```

### Result (partial)
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
images/pulls
images/mflst
images/mflst/a4afe5bf0eefa56aebe9b754cdcce26c88bebfa89cb12ca73808ba1d701189d7
images/blobs
images/blobs/4873874c08efc72e9729683a83ffbb7502ee729e9a5ac097723806ea7fa13517
images/blobs/fcb6f6d2c9986d9cd6a2ea3cc2936e5fc613e09f1af9042329011e43057f3265
images/blobs/9457426d68990df190301d2e20b8450c4f67d7559bdb7ded6c40d41ced6731f7
images/blobs/e5dbef90bae3c9df1dfd4ae7048c56226f6209d538c91f987aff4f54e888f566
images/blobs/e8c73c638ae9ec5ad70c49df7e484040d889cca6b4a9af056579c3d058ea93f0
images/blobs/65efb1cabba44ca8eefa2058ebdc19b7f76bbb48400ff9e32b809be25f0cdefa
images/blobs/6523efc24f16435b7507a67c2a1f21828c9d58531902856b294bf49d04b96bbe
images/blobs/aba5379b9c6dc7c095628fe6598183d680b134c7f99748649dddf07ff1422846
images/blobs/fbe9343cb4af98ca5a60b6517bf45a5a4d7f7172fb4793d4b55c950196089cda
images/blobs/53f492e4d27a1a1326e593efdaffcb5e2b0230dc661b20a81a04fa740a37cb4c
images/blobs/13547472c521121fc04c8fa473757115ef8abe698cc9fa67e828371feeff40e7
images/blobs/1e3d9b7d145208fa8fa3ee1c9612d0adaac7255f1bbc9ddea7e461e0b317805c
images/blobs/4aa0ea1413d37a58615488592a0b827ea4b2e48fa5a77cf707d0e35f025e613f
images/blobs/406945b5115423a8c1d1e5cd53222ef2ff0ce9d279ed85badbc4793beebebc6c
images/imgmf
images/imgmf/019d7877d15b45951df939efcb941de9315e8381476814a6b6fdf34fc1bee24c
```

The manifest list was saved in `images/mflst/a4afe5bf0eefa56aebe9b754cdcce26c88bebfa89cb12ca73808ba1d701189d7` and the image manifest was saved in `images/imgmf/019d7877d15b45951df939efcb941de9315e8381476814a6b6fdf34fc1bee24c`.

### Stop and restart the server and repeat

You will notice that the manifest list and the image manifest are now being returned from cache.
