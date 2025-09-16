# Quick Start (Desktop)

The easiest way to understand how the server works is to git clone the repo, then build and run the server locally on your Linux desktop.

After git-cloning the project:

## Build the server
```shell
make server
```

This command compiles the server and creates a binary called `ociregistry` in the `bin` directory relative to the project root.

## Run the server

You provide a file system location for the image cache with the `--image-path` arg. If the directory doesn't exist the server will create it. The default is `/var/lib/ociregistry` but to kick the tires it makes more sense to use the system temp directory. By default the server listens on `8080`. If you have something already running and bound to that port, specify `--port`. We'll specify it explicitly here with the default value:

```shell
bin/ociregistry --image-path /tmp/images serve --port 8080
```

 Result:
```shell
----------------------------------------------------------------------
OCI Registry: pull-only, pull-through, caching OCI Distribution Server
Version: 1.9.4, build date: 2025-09-08T00:05:54.37Z
Started: 2025-09-10 19:30:24.133946199 -0400 EDT (port 8080)
Running as (uid:gid) 1000:1000
Process id: 27010
Tls: none
Command line: bin/ociregistry --image-path /tmp/images serve --port 8080
----------------------------------------------------------------------
```

## Curl an image manifest list

Curl a manifest list using the OCI Distribution Server REST API. Note the `ns` query parameter in the URL below which tells the server to go to that upstream if the image isn't already locally cached. This is exactly how containerd does it when you configure containerd to mirror:

```shell
curl localhost:8080/v2/kube-scheduler/manifests/v1.29.1?ns=registry.k8s.io | jq
```

_(The server also supports in-path namespaces like `localhost:8080/v2/registry.k8s.io/kube-scheduler/manifests/v1.29.1`)_

 Result:
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
    etc...
```

## Curl an image manifest

Pick the first manifest from the list above - the `amd64/linux` manifest - and curl the manifest by SHA, again using the OCI Distribution Server REST API:

```shell
DIGEST=019d7877d15b45951df939efcb941de9315e8381476814a6b6fdf34fc1bee24c
curl localhost:8080/v2/kube-scheduler/manifests/sha256:$DIGEST?ns=registry.k8s.io | jq
```

 Result:
```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
  "config": {
    "mediaType": "application/vnd.docker.container.image.v1+json",
    "size": 2425,
    "digest": "sha256:406945b5115423a8c1d1e5cd53222ef2ff0ce9d279ed85badbc4793beebebc6c"
  },
  "layers": [
    {
      "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
      "size": 84572,
      "digest": "sha256:aba5379b9c6dc7c095628fe6598183d680b134c7f99748649dddf07ff1422846"
    },
    etc...
```

## Inspect the image cache

```shell
find /tmp/images
```

Result:

```shell
/tmp/images
/tmp/images/blobs
/tmp/images/blobs/fcb6f6d2c9986d9cd6a2ea3cc2936e5fc613e09f1af9042329011e43057f3265
/tmp/images/blobs/e5dbef90bae3c9df1dfd4ae7048c56226f6209d538c91f987aff4f54e888f566
/tmp/images/blobs/e8c73c638ae9ec5ad70c49df7e484040d889cca6b4a9af056579c3d058ea93f0
etc..
/tmp/images/img
/tmp/images/img/019d7877d15b45951df939efcb941de9315e8381476814a6b6fdf34fc1bee24c
/tmp/images/img/a4afe5bf0eefa56aebe9b754cdcce26c88bebfa89cb12ca73808ba1d701189d7
```

The manifest list was saved in: `images/img/a4afe5bf0ee...` and the image manifest was saved in: `images/img/019d7877d1...`.

> When you curled the image manifest the server pulled and cached the blobs at the same time and stored them in `images/blobs`

## Restart the Server and Repeat

Stop the server with CTRL-C. Re-run, this time run with `info` logging for more visibility into what the server is doing. (The default logging level is `error`.)

```shell
bin/ociregistry --image-path /tmp/images --log-level info serve --port 8080
```

Run the same two curl commands.

## Observe the logs

You will notice that the manifest list and the image manifest are now being returned from cache. You can see this in the logs:

```shell
INFO[0000] server is running                            
INFO[0007] serving manifest from cache: "registry.k8s.io/kube-scheduler:v1.29.1" 
INFO[0007] echo server GET:/v2/kube-scheduler/manifests/v1.29.1?ns=registry.k8s.io status=200 latency=663.938µs host=localhost:8888 ip=::1 
INFO[0010] serving manifest from cache: "registry.k8s.io/kube-scheduler@sha256:019d7877d15b45951df939efcb941de9315e8381476814a6b6fdf34fc1bee24c" 
INFO[0010] echo server GET:/v2/kube-scheduler/manifests/sha256:019d7877d1?ns=registry.k8s.io status=200 latency=949.646µs host=localhost:8888 ip=::1 
```

## Docker pull (through the server)

If you have Docker (or Podman, or Crane, or your other favorite registry client), you can pull the image through the _Ociregistry_ server. This uses the _in-path_ image url form that both Docker **and** _Ociregistry_ understand:

```shell
docker pull localhost:8080/registry.k8s.io/kube-scheduler:v1.29.1
```

 Result:

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
Status: Downloaded newer image for localhost:8888/registry.k8s.io/kube-scheduler:v1.29.1
localhost:8888/registry.k8s.io/kube-scheduler:v1.29.1
```

## Observe the new log entries

The _Ociregistry_ server displays new log entries that show the image is being served from cache:

```shell
...
INFO[0294] get /v2/                                     
INFO[0294] echo server GET:/v2/ status=200 latency=117.389µs host=localhost:8888 ip=127.0.0.1 
INFO[0294] serving manifest from cache: "registry.k8s.io/kube-scheduler:v1.29.1" 
INFO[0294] echo server HEAD:/v2/registry.k8s.io/kube-scheduler/manifests/v1.29.1 status=200 latency=353.63µs host=localhost:8888 ip=127.0.0.1 
INFO[0294] serving manifest from cache: "registry.k8s.io/kube-scheduler@sha256:a4afe5bf0eefa56aebe9b754cdcce26c88bebfa89cb12ca73808ba1d701189d7" 
INFO[0294] echo server GET:/v2/registry.k8s.io/kube-scheduler/manifests/sha256:a4afe5bf0e status=200 latency=341.107µs host=localhost:8888 ip=127.0.0.1 
INFO[0294] serving manifest from cache: "registry.k8s.io/kube-scheduler@sha256:019d7877d15b45951df939efcb941de9315e8381476814a6b6fdf34fc1bee24c" 
INFO[0294] echo server GET:/v2/registry.k8s.io/kube-scheduler/manifests/sha256:019d7877d1 status=200 latency=471.765µs host=localhost:8888 ip=127.0.0.1 
INFO[0294] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:406945b511 status=200 latency=458.581µs host=localhost:8888 ip=127.0.0.1 
INFO[0294] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:aba5379b9c status=200 latency=1.187488ms host=localhost:8888 ip=127.0.0.1 
INFO[0294] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:fbe9343cb4 status=200 latency=1.730496ms host=localhost:8888 ip=127.0.0.1 
INFO[0294] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:e5dbef90ba status=200 latency=1.452105ms host=localhost:8888 ip=127.0.0.1 
INFO[0294] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:fcb6f6d2c9 status=200 latency=916.319µs host=localhost:8888 ip=127.0.0.1 
INFO[0294] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:e8c73c638a status=200 latency=855.956µs host=localhost:8888 ip=127.0.0.1 
INFO[0294] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:1e3d9b7d14 status=200 latency=836.118µs host=localhost:8888 ip=127.0.0.1 
INFO[0294] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:4aa0ea1413 status=200 latency=654.022µs host=localhost:8888 ip=127.0.0.1 
INFO[0294] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:65efb1cabb status=200 latency=581.97µs host=localhost:8888 ip=127.0.0.1 
INFO[0294] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:13547472c5 status=200 latency=1.036117ms host=localhost:8888 ip=127.0.0.1 
INFO[0294] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:53f492e4d2 status=200 latency=1.654236ms host=localhost:8888 ip=127.0.0.1 
INFO[0294] echo server GET:/v2/registry.k8s.io/kube-scheduler/blobs/sha256:6523efc24f status=200 latency=47.179364ms host=localhost:8888 ip=127.0.0.1 
```

## Observe the image is the Docker cache

Running `docker image ls` should show the newly pulled image:

```shell
REPOSITORY                                      TAG       IMAGE ID       CREATED         SIZE
localhost:8888/registry.k8s.io/kube-scheduler   v1.29.1   406945b51154   20 months ago   59.5MB
```

## Summary

In this quick start you built the server on your desktop, ran it, and pulled an image through it. You verified that the first pull pulled through the _Ociregistry_ server to the upstream, but a subsequent pull served the manifests and blobs from the _Ociregistry_ server's cache.