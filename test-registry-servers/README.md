# Pull through testing

This directory supports running the "unit" tests in `impl/upstream/pull_test.go`. I say "unit" tests because they're not true unit tests at this time. I need to implement mocks for them to be unit tests. For now the tests require setup outside of the standard Go testi scaffolding.

These steps use the Go Container Registry *crane* utility: https://github.com/google/go-containerregistry/blob/main/cmd/crane/doc/crane.md

## Theory

The goal is to test the various basic auth and TLS configurations for pulling from upstreams. To test the pull through ability of this project, I create two registries running in Docker containers, and one Nginx instance also in Docker for TLS termination. This enables me to test HTTP and HTTPS to the upstreams:

```
+--------------+     +----------------+     +------------+     +----------+
| pull_test.go | --> | ociregistry    | --> | nginx      |     | registry |
+--------------+     | (this project) |     | 1-way 8443 | --> | no auth  |
                     |                |     | 2-way 8444 |     | 5000     |
                     |                |     +------------+     |          |
                     |                | ---------------------> |          |
                     +----------------+     +------------+     +----------+
                              |             | registry   |
                              +-----------> | basic auth |
                                            | 5001       |
                                            +------------+
```

## Steps

Run three docker containers as described below. These instructions assume your current working directory is the project root.

### Container 1 - HTTP no auth registry

In console 1:
```
docker run\
  --rm\
  --name registry\
  -p 5000:5000\
  registry:2.8.3
```

### Container 2 - HTTP basic auth registry

In console 2:
```
docker run\
 --rm\
 --name registry-auth\
 -p 5001:5000\
 -v $(realpath test-registry-servers/basic-auth-no-tls/auth):/auth\
 -e REGISTRY_AUTH=htpasswd\
 -e REGISTRY_AUTH_HTPASSWD_REALM="Registry Realm"\
 -e REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd\
 registry:2.8.3
```

### Container 3 - Nginx for TLS termination

Remember that Nginx is configured to serve on both 8443 and 8444.

In console 3:
```
docker run\
  --rm\
  --name nginx\
  --network host\
  -v $(realpath test-registry-servers/nginx/conf/):/etc/nginx\
  -v $(realpath test-registry-servers/nginx/certs):/certs\
  -P\
  nginx:latest
```

### Populate the two Docker registries

```
crane pull docker.io/hello-world:latest hello-world.latest.tar
crane push hello-world.latest.tar localhost:5000/hello-world:latest
crane auth login -u ericace -p ericace localhost:5001
crane push hello-world.latest.tar localhost:5001/hello-world:latest
crane auth logout localhost:5001
```

Now both the `5000` and `5001` registries have an image to pull through into the ociregistry to support the tests.

### Run the tests

At this point the tests in `impl/upstream/pull_test.go` can be run.

