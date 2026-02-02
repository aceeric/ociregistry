![logo](resources/ociregistry.logo.png)

![Version: 1.12.0](https://img.shields.io/badge/Version-1.12.0-informational?style=rounded-square)
[![Unit tests](https://github.com/aceeric/ociregistry/actions/workflows/unit-test.yml/badge.svg)](https://github.com/aceeric/ociregistry/actions/workflows/unit-test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/aceeric/ociregistry)](https://goreportcard.com/report/github.com/aceeric/ociregistry)
[![Go Vuln Check](https://github.com/aceeric/ociregistry/actions/workflows/vulncheck.yml/badge.svg)](https://github.com/aceeric/ociregistry/actions/workflows/vulncheck.yml)
[![Artifact HUB](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/ociregistry)](https://artifacthub.io/packages/search?repo=ociregistry)
![coverage](https://raw.githubusercontent.com/aceeric/ociregistry/badges/.badges/main/coverage.svg)

_Ociregistry_ is a **pull-only**, **pull-through**, **caching** OCI Distribution server. That means:

1. It exclusively provides _pull_ capability. It does this by implementing a subset of the [OCI Distribution Spec](https://github.com/opencontainers/distribution-spec).
2. It provides *caching pull-through* capability to multiple upstream registries: internal, air-gapped, or public; supporting the following types of access: anonymous, basic auth, HTTP, HTTPS (secure & insecure), one-way TLS, and mTLS. In other words, one running instance of this server can simultaneously pull from `docker.io`, `quay.io`, `registry.k8s.io`, `ghcr.io`, your air-gapped registries, in-house corporate mirrors, etc.

## Goals

The goal of the project is to build a performant, simple, reliable edge OCI Distribution server for Kubernetes. One of the overriding goals was simplicity: only one binary is needed to run the server, and all state is persisted as simple files on the file system under one subdirectory. These files can easily be used to inspect the server cache using well-known tools like `grep`, `find`, `jq` and so on.

And because of this design, the entire image store can be tarred up, copied to another location, and un-tarred. And then simply starting the server in that remote location with the `--image-file` arg pointing to the copied directory will serve exactly the same image cache.

## Detailed Documentation

Full documentation is available on the [GitHub Pages](https://aceeric.github.io/ociregistry) site. As you can also see in the badges above, a Helm chart is available on [Artifacthub](https://artifacthub.io/packages/search?repo=ociregistry) for running the registry as a Kubernetes workload.

## Quick Start - Desktop

After git cloning the project:

## Build the Server
```shell
make server
```

This command compiles the server and creates a binary called `ociregistry` in the `bin` directory relative to the project root.

## Run the Server

You provide an image storage location with the `--image-path` arg. If the directory doesn't exist the server will create it. The default is `/var/lib/ociregistry` but to kick the tires it makes more sense to use the system temp directory. By default the server listens on `8080`. If you have something running that is already bound to that port, specify `--port`. We'll specify it explicitly here with the default value:

```shell
bin/ociregistry --log-level info --image-path /tmp/images serve --port 8080
```

## Server startup logs

```shell
----------------------------------------------------------------------
OCI Registry: pull-only, pull-through, caching OCI Distribution Server
Version: 1.9.7, build date: 2025-10-06T23:35:46.56Z
Started: 2025-10-06 19:49:57.169081368 -0400 EDT (port 8080)
Running as (uid:gid) 1000:1000
Process id: 95070
Tls: none
Command line: bin/ociregistry --log-level info --image-path /tmp/images serve --port 8080
----------------------------------------------------------------------
INFO[0000] server is running                            
```

## Pull through the server

Assuming you have Docker (or Podman, or Crane, or your other favorite registry client), you can pull images through the _Ociregistry_ server. This uses the _in-path_ image url form that both Docker **and** _Ociregistry_ understand. Run this command in another console window:

```shell
docker pull localhost:8080/registry.k8s.io/kube-scheduler:v1.29.1
```

## Result

If you used `docker` you should see this output:

```shell
v1.29.1: Pulling from registry.k8s.io/kube-scheduler
aba5379b9c6d: Pull complete 
e5dbef90bae3: Pull complete 
fbe9343cb4af: Pull complete 
fcb6f6d2c998: Pull complete 
e8c73c638ae9: Pull complete 
1e3d9b7d1452: Pull complete 
4aa0ea1413d3: Pull complete 
65efb1cabba4: Pull complete 
13547472c521: Pull complete 
53f492e4d27a: Pull complete 
6523efc24f16: Pull complete 
Digest: sha256:a4afe5bf0eefa56aebe9b754cdcce26c88bebfa89cb12ca73808ba1d701189d7
Status: Downloaded newer image for localhost:8080/registry.k8s.io/kube-scheduler:v1.29.1
localhost:8080/registry.k8s.io/kube-scheduler:v1.29.1
```

Observe the _Ociregistry_ logs showing how the server handled the pull:

```shell
INFO[0199] get /v2/                                     
INFO[0199] echo server GET:/v2/ status=200 latency=73.392µs host=localhost:8080 ip=127.0.0.1 
INFO[0199] pulling manifest from upstream: "registry.k8s.io/kube-scheduler:v1.29.1" 
INFO[0199] echo server HEAD:/v2/registry.k8s.io/kube-scheduler/manifests/v1.29.1 status=200 latency=507.437429ms host=localhost:8080 ip=127.0.0.1 
INFO[0199] serving manifest from cache: "registry.k8s.io/kube-scheduler@sha256:a4afe5bf0eefa56aebe9b754cdcce26c88bebfa89cb12ca73808ba1d701189d7" 
INFO[0199] echo server GET:/v2/registry.k8s.io/kube-scheduler/manifests/sha256:a4afe5bf0e status=200 latency=450.065µs host=localhost:8080 ip=127.0.0.1 
INFO[0199] pulling manifest from upstream: "registry.k8s.io/kube-scheduler@sha256:019d7877d15b45951df939efcb941de9315e8381476814a6b6fdf34fc1bee24c" 
INFO[0201] echo server GET:/v2/registry.k8s.io/kube-scheduler/manifests/sha256:019d7877d1 status=200 latency=1.896723966s host=localhost:8080 ip=127.0.0.1 
INFO[0201] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:406945b511 status=200 latency=989.121µs host=localhost:8080 ip=127.0.0.1 
INFO[0201] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:aba5379b9c status=200 latency=1.206855ms host=localhost:8080 ip=127.0.0.1 
INFO[0201] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:e5dbef90ba status=200 latency=1.072352ms host=localhost:8080 ip=127.0.0.1 
INFO[0201] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:fbe9343cb4 status=200 latency=2.084094ms host=localhost:8080 ip=127.0.0.1 
INFO[0201] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:fcb6f6d2c9 status=200 latency=971.832µs host=localhost:8080 ip=127.0.0.1 
INFO[0201] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:e8c73c638a status=200 latency=843.471µs host=localhost:8080 ip=127.0.0.1 
INFO[0201] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:1e3d9b7d14 status=200 latency=895.636µs host=localhost:8080 ip=127.0.0.1 
INFO[0201] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:4aa0ea1413 status=200 latency=984.738µs host=localhost:8080 ip=127.0.0.1 
INFO[0201] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:65efb1cabb status=200 latency=974.436µs host=localhost:8080 ip=127.0.0.1 
INFO[0201] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:13547472c5 status=200 latency=1.461483ms host=localhost:8080 ip=127.0.0.1 
INFO[0201] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:53f492e4d2 status=200 latency=2.265104ms host=localhost:8080 ip=127.0.0.1 
INFO[0201] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:6523efc24f status=200 latency=70.020159ms host=localhost:8080 ip=127.0.0.1 
```

Note the occurrence of `pulling manifest from upstream` in the logs above. Stop the server with CTRL-C and simply re-run it with the same command. In your other console window, remove the image from Docker's cache so that on the next pull, Docker has to go back through the _Ociregistry_.

```shell
docker images --format "{{.ID}}" localhost:8080/registry.k8s.io/kube-scheduler:v1.29.1\
  | xargs docker rmi
```

Re-pull the same image with the same docker command:
```shell
docker pull localhost:8080/registry.k8s.io/kube-scheduler:v1.29.1
```

The more recent _Ociregistry_ log entries show:

```shell
INFO[0784] get /v2/                                     
INFO[0784] echo server GET:/v2/ status=200 latency=36.735µs host=localhost:8080 ip=127.0.0.1 
INFO[0784] serving manifest from cache: "registry.k8s.io/kube-scheduler:v1.29.1" 
INFO[0784] echo server HEAD:/v2/registry.k8s.io/kube-scheduler/manifests/v1.29.1 status=200 latency=127.739µs host=localhost:8080 ip=127.0.0.1 
INFO[0784] serving manifest from cache: "registry.k8s.io/kube-scheduler@sha256:a4afe5bf0eefa56aebe9b754cdcce26c88bebfa89cb12ca73808ba1d701189d7" 
INFO[0784] echo server GET:/v2/registry.k8s.io/kube-scheduler/manifests/sha256:a4afe5bf0e status=200 latency=169.374µs host=localhost:8080 ip=127.0.0.1 
INFO[0784] serving manifest from cache: "registry.k8s.io/kube-scheduler@sha256:019d7877d15b45951df939efcb941de9315e8381476814a6b6fdf34fc1bee24c" 
INFO[0784] echo server GET:/v2/registry.k8s.io/kube-scheduler/manifests/sha256:019d7877d1 status=200 latency=171.32µs host=localhost:8080 ip=127.0.0.1 
INFO[0784] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:406945b511 status=200 latency=288.877µs host=localhost:8080 ip=127.0.0.1 
INFO[0784] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:e5dbef90ba status=200 latency=381.53µs host=localhost:8080 ip=127.0.0.1 
etc...
```

You can see that `pulling manifest from upstream` from the first pull has been replaced with `serving manifest from cache`.

## The image store

To view the image store: `find /tmp/images`. There you will see the manifests and blobs comprising the cache. To clean up, simply stop the server (CTRL-C) and `rm -rf /tmp/images`. Everything the server persisted to the file system is under that one directory.

## Full Documentation

The full documentation in the [GitHub Pages](https://aceeric.github.io/ociregistry) site covers the quick start above and many other topics:

* Quick Start for the Helm chart
* Configuring The Server
* The Command Line
* Loading And Pre-Loading
* Pruning The Image Cache
* Design (including Concurrency Design)
* Airgap Considerations
* Administrative REST API
* Running _Ociregistry_ as a systemd Service
