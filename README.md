# Experiments with creating a light-weight "pull-only" OCI Registry

Approach:

## Review the Open Container Distribution Spec

https://github.com/opencontainers/distribution-spec/blob/main/spec.md#endpoints

## Run the swagger editor in Docker

docker run -d -p 8080:8080 swaggerapi/swagger-editor

## Code each Method in the API Spec in Swagger

```
docker run -p 80:8080 swaggerapi/swagger-editor
```

## Export to ociregistry.yaml (using the Swagger UI)

Results in `ociregistry.yaml`

## Install oapi-codegen
```
go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest
```

## Init the Go project

```
go mod init
```

## Frame out a dir structure

Mirroring: https://github.com/deepmap/oapi-codegen/tree/master/examples/petstore-expanded/echo:
```
├── api
│   ├── models
│   │   └──models.gen.go (generated)
│   ├── models.cfg.yaml
│   ├── ociregistry.gen.go (generated)
│   ├── ociregistry.go (implementation of the generated stubs)
│   └── server.cfg.yaml
├── bin
├── cmd
│   └── ociregistry.go (this is the server)
├── go.mod
├── go.sum
├── ociregistry.yaml (the openapi spec exported from the Swagger editor)
└── README.md
```

## Fill in api/models.cfg.yaml and api/server.cmg.yaml

Modeling the upstream Pet Store

## Generate server and model code

oapi-codegen -config api/models.cfg.yaml ociregistry.yaml
oapi-codegen -config api/server.cfg.yaml ociregistry.yaml

## Stub out `api/ociregistry.go`

Modeling after the upstream Pet Store. Support all the methods otherwise the project will not build.

## Build

```
go build -o bin/server-http cmd/ociregistry.go
go build -o bin/server-https cmd/ociregistry.go
```

## Implement the server

Go back into `api/ociregistry.go` and fill in the methods that implement the server. Based on running [Fiddler](https://www.telerik.com/fiddler) - the free version - and capturing all of the traffic associated with running `crane pull docker.io/hello-world:latest`. (Was unable to capture any traffic associated with `docker pull`.)

## Run

```
bin/server-http (or https)
```

## Testing

In the `smallgo` directory build the Go program and the Docker image. If successful then:
```
docker images
```

Should produce
```
REPOSITORY                       TAG       IMAGE ID       CREATED          SIZE
localhost:5000/appzygy/smallgo   v1.0.0    c07281f652ba   23 minutes ago   1.72MB
...
```

This docker image will be used to test the mock Registry. First run a local registry to push the image to:
```
docker run -d -p 5000:5000 --name registry registry:2.8.3
```

Push the image to the local registry
```
docker push localhost:5000/appzygy/smallgo:v1.0.0
```

Result:
```
The push refers to repository [localhost:5000/appzygy/smallgo]
7689c74f7829: Pushed 
6e8b152e65c6: Pushed 
a95a9c41f2c8: Pushed 
v1.0.0: digest: sha256:532e9bccbe948659fb417f0e9403ac63d8581f9e3b7414ed460ec83154b77f9b size: 942
```

Use the Google `crane` utility to export the image to a tarball:

```
crane pull localhost:5000/appzygy/smallgo:v1.0.0 smallgo.tar --cache_path /tmp
```

Result:
```
2024/01/13 20:19:51 Layer sha256:e9a5742d071431258605f03e2eee366586042dcf71cf59a5a88d07e07b49a2cd not found (compressed) in cache, getting
2024/01/13 20:19:51 Layer sha256:a28711b5343eec30b7799fc0482b85560aa02717445bf415c081bef15323e6b6 not found (compressed) in cache, getting
2024/01/13 20:19:51 Layer sha256:033967b6126e74d6e31fc90c016476c328b4eea755b4b6750a2d758df0070aed not found (compressed) in cache, getting
```

Inspect:
```
tar -tvf smallgo.tar 
```

Result:
```
-rw-r--r-- 0/0            1020 1969-12-31 19:00 sha256:c07281f652bace2b746f8241a5013b6034d32d6b001967295c7e99b31a49b01e
-rw-r--r-- 0/0         1076173 1969-12-31 19:00 e9a5742d071431258605f03e2eee366586042dcf71cf59a5a88d07e07b49a2cd.tar.gz
-rw-r--r-- 0/0             100 1969-12-31 19:00 a28711b5343eec30b7799fc0482b85560aa02717445bf415c081bef15323e6b6.tar.gz
-rw-r--r-- 0/0             103 1969-12-31 19:00 033967b6126e74d6e31fc90c016476c328b4eea755b4b6750a2d758df0070aed.tar.gz
-rw-r--r-- 0/0             372 1969-12-31 19:00 manifest.json
```

Uncompress the tarball into the `images`` directory:
```
mkdir -p images/appzygy/smallgo/v1.0.0 &&
tar -xf smallgo.tar -C images/appzygy/smallgo/v1.0.0
```

Verify:
```
find images
```

Result:
```
images
images/appzygy
images/appzygy/smallgo
images/appzygy/smallgo/v1.0.0
images/appzygy/smallgo/v1.0.0/a28711b5343eec30b7799fc0482b85560aa02717445bf415c081bef15323e6b6.tar.gz
images/appzygy/smallgo/v1.0.0/manifest.json
images/appzygy/smallgo/v1.0.0/sha256:c07281f652bace2b746f8241a5013b6034d32d6b001967295c7e99b31a49b01e
images/appzygy/smallgo/v1.0.0/033967b6126e74d6e31fc90c016476c328b4eea755b4b6750a2d758df0070aed.tar.gz
images/appzygy/smallgo/v1.0.0/e9a5742d071431258605f03e2eee366586042dcf71cf59a5a88d07e07b49a2cd.tar.gz
```

In another console use `crane` to pull the image from the mock registry:
```
crane pull localhost:8080/appzygy/smallgo:v1.0.0 smallgo-mock.tar --cache_path /tmp
```

Result:
```
2024/01/13 20:24:27 Layer sha256:e9a5742d071431258605f03e2eee366586042dcf71cf59a5a88d07e07b49a2cd found (compressed) in cache
2024/01/13 20:24:27 Layer sha256:a28711b5343eec30b7799fc0482b85560aa02717445bf415c081bef15323e6b6 found (compressed) in cache
2024/01/13 20:24:27 Layer sha256:033967b6126e74d6e31fc90c016476c328b4eea755b4b6750a2d758df0070aed found (compressed) in cache
```

Verify:
```
ls -l *.tar
```

Result:
```
-rw-rw-r-- 1 eace eace 1082368 Jan 13 20:24 smallgo-mock.tar
-rw-rw-r-- 1 eace eace 1082368 Jan 13 20:19 smallgo.tar
```

Test the Kubernetes can run a workload from the image:
```
scp -i ./generated/kickstart/id_ed25519 ~/projects/ociregistry/smallgo-mock.tar root@192.169.56.200:.
```

Result:
```
smallgo-mock.tar
```

SSH into the Kubernetes control plane host and load the image into the containerd cache:
```
ctr -n k8s.io -a /var/run/containerd/containerd.sock image import smallgo-mock.tar
```

Result:
```
unpacking localhost:8080/appzygy/smallgo:v1.0.0 (sha256:c79ddd7f1e6ebba54b0eeb294b7a09081857e09350eb5d4022cbdb2738fe2629)...done
```

Confirm:
```
crictl images
```

Result:
```
IMAGE                          TAG     IMAGE ID       SIZE
...
localhost:8080/appzygy/smallgo v1.0.0  c07281f652bac  1.08MB
...
```

Log out of the Host, and create a deployment:
```
kubectl apply -f ~/projects/ociregistry/k8s-manifests/deployment.yaml
```

Verify:
```
kubectl logs -f deploy/foo
```

Result:
```
running main
running main
running main
running main
running main
running main
running main
running main
running main
running main
running main
(etc.)
```

# TODO

1. Can't `docker run` or have containerd pull from the mock registry !! Each attempts to GET `/v2/appzygy/smallgo/manifests/sha256:????` and I can't figure out why these requests differ from `crane save`
2. TLS
3. Real Auth
4. Host the OpenAPI swagger UI from the serverr

## Misc notes

Was able to get containerd in the cluster to pull from the mock registry running on the desktop:
```
[root@vm1 containerd]# find certs.d/ && cat certs.d/192.168.0.49:8080/hosts.toml
certs.d/
certs.d/192.168.0.49:8080
certs.d/192.168.0.49:8080/hosts.toml
server = "http://192.168.0.49:8080"

[host."http://192.168.0.49:8080"]
  capabilities = ["pull"]
  skip_verify = true

curl http://localhost:8080/v2/library/hello-world/manifests/latest | jq
rane > GET index.docker.io/v2/library/hello-world/manifests/sha256:e2fc4e5012d16e7fe466f5291c476431beaa1f9b90a5c2125b493ed28e2aba57
crane > GET index.docker.io/v2/library/hello-world/blobs/sha256:d2c94e258dcb3c5ac2798d32e1249e42ef01cba4841c2234249495f87264ac5a
```



## Nginx Docker

WORKS:

docker run -it --rm --name curl --network host curlimages/curl:latest sh

curl -H "X-Registry: docker.io"  http://localhost:8080/v2/kubernetesui/dashboard/manifests/v2.7.0

{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"application/vnd.oci.image.config.v1+json","digest":"sha256:07655ddf2eebe5d250f7a72c25f638b27126805d61779741b4e62e69ba080558","size":1555},"layers":[{"mediaType":"application/vnd.oci.image.layer.v1.tar+gzip","digest":"sha256:ee3247c7e545df975ba3826979c7a8d73f1373cbb3ac47def3b734631cef2965","size":75784467},{"mediaType":"application/vnd.oci.image.layer.v1.tar+gzip","digest":"sha256:8e052fd7e2d0aec4ef51e4505d006158414775ad5f0ea3e479ac0ba92f90dfff","size":508}]}

goal - validate that registry can connect to upstream:
- one-way tls insecure
- one-way tls secure
- two-way tls insecure
- two-way tls secure

curl > ociregistry (desktop) > nginx revproxy (docker) > registry (docker)