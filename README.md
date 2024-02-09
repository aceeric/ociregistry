# Pull-only Pull-through Caching OCI Distribution Server

This project is a **pull-only**, **pull-through**, **caching** OCI Distribution server. That means:

1. It exclusively provides _pull_ capability. You can't push images to it, it doesn't support the `/v2/_catalog` endpoint, etc.
2. It provides *caching pull-through* capability to any upstream registry: internal, air-gapped, or public - supporting the following types of access: anonymous, basic auth, HTTP/HTTPS, 1-way TLS, and mTLS.

This distribution server is intended to satisfy **one** use case: the need for an in-cluster Kubernetes caching pull-through registry that enables the cluster to run reliably in a network context with no-, intermittent-, or low latency connectivity to upstream registries - or - an environment where the upstream registries serving the k8s cluster have less than 5 nines availability.

As a secondary capability the server can be loaded from image tarballs. This supports a scenario where the registry is loaded in one location, disconnected, transported elsewhere, and then runs air-gapped: you load the registry from image tarballs before shipment.

The goals of the project are:

1. Implement one use case
2. Be simple

To achieve this - the server simply uses the file system as the sum total of it's knowledge about the image cache. The belief is - this approach should result in high reliability: the registry should be able to sustain normal k8s disruptions like pod evictions of the registry container as long as the underlying persistent storage for the image cache is reliable.



## Design

![design](resources/design.png)



The basic usage scenario is:

1. A client (containerd) pulls an image. The Echo server handles the API calls.
2. The Echo server delegates to the OCI Registry API handlers to implement the application logic.
3. If the image is cached on the file system it is provided to the caller.
4. Otherwise, the API handlers delegate to Google Crane code embedded in the server. To support this, containerd is configured to pass the upstream registry in the `X-Registry` header in step 1.
5. The Google Crane code builds the upstream registry URL using the `X-Registry` header value and pulls the image from the upstream as a tarball, returning it to the handler which unpacks it and saves it to the image store.
6. Concurrently, a tarball loader runs as a goroutine watching a staging directory on the file system. Whenever a tarball appears there it is unpacked and moved to the image store, and then the tarball is deleted.

## API Implementation

The OCI Distribution API is built by creating an Open API spec using Swagger: `ociregistry.yaml` in the project root. Then the `oapi-codegen` tool is used to generate the API and models using configuration in the `api` directory. This approach was modeled on the OAPI-Codegen _Petstore_ example: https://github.com/deepmap/oapi-codegen/tree/master/examples/petstore-expanded/echo.

The key components of the API scaffolding are shown below:

```shell
​```
├── api
│   ├── models
│   │   └──models.gen.go (generated)
│   ├── models.cfg.yaml
│   ├── ociregistry.gen.go (generated)
│   └── server.cfg.yaml
├── cmd
│   └── ociregistry.go (this is the server - which embeds the Echo server)
└── ociregistry.yaml (the openapi spec)
​```
```

## Configuring `containerd`

The following snippet shows how to configure `containerd` to pass the `X-Registry` header to support pull-through:

Add a `config_path` entry to `/etc/containerd/config.toml` to tell `containerd` to load all registry mirror configuration from that directory:

```shell
   ...
   [plugins."io.containerd.grpc.v1.cri".registry]
      config_path = "/etc/containerd/certs.d"
   ...
```

Then create a configuration directory and file for each upstream that will pull through the caching pull-through registry server. This is an example for `docker.io`. The file is `/etc/containerd/certs.d/docker.io/hosts.toml`. In this hypothetical example, the caching pull-through registry server is running on `192.168.0.49:8080`:

```shell
server = "http://192.168.0.49:8080"

[host."http://192.168.0.49:8080"]
  capabilities = ["pull"]
  skip_verify = true
  [host."http://192.168.0.49:8080".header]
    X-Registry = ["docker.io"]
```

## Configuring the Server

The server may need configuration information to connect to upstreams. By default, it will attempt anonymous plain HTTP access. Many servers will reject HTTP and fail over to HTTPS. Then you're in the realm of PKI. Some servers require auth. To address all of these concerns the server accepts a command line parameter `--config-path` which identifies a configuration file in the following format:

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
  description: Something that makes sense to you
  auth:
    user: theuser
    password: thepass
  tls:
    ca: /fully/qualified/path/to/ca.crt
    cert: ".../client.cert
    key: ".../client.key
    insecure_skip_verify: true/false
```

The `auth` section implements basic auth. The `tls` section can implement the following scenarios:

1. 1-way insecure TLS in which client certs are not provided to the remote, and the remote server cert is not validated:

   ```
   tls:
     insecure_skip_verify: true
   ```

2. 1-way **secure** TLS in which client certs are not provided to the remote, and the remote server cert **is** validated using the OS trust store:

   ```
   tls:
     insecure_skip_verify: false (or simply omit this - which defaults to false)
   ```

3. 1-way **secure** TLS in which client certs are not provided to the remote, and the remote server cert is validate using a **provided** CA cert

   ```
   tls:
     ca: /fully/qualified/path/to/ca.crt
   ```

4. 2-way TLS (client certs are provided to the remote):

   ```
   tls:
     cert: "/fully/qualified/path/to/client.cert
     key: ".../client.key
   ```
   2-way TLS can be implemented with and without remote server cert validation as described in the 1-way TLS scenarios above.

## Command line options

The following options are supported:

| Option        | Default              | Meaning                                                      |
| ------------- | -------------------- | ------------------------------------------------------------ |
| --log-level   | error                | Valid values: debug, info, warn, error                       |
| --image-path  | /var/lib/ociregistry | The root directory of the image store                        |
| --config-path | Empty                | Path a file providing remote registry auth and TLS config. If empty then every upstream will be tried with anonymous HTTP access failing over to HTTP using the OS Trust store to validate the remote registry. |
| --port        | 8080                 | Server port. E.g. `crane pull localhost:8080 foo.tar`        |

