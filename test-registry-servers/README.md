# Pull through testing

This directory supports running the "unit" tests in `pullsync/cranepull_test.go`. I say "unit" tests because they're not true unit tests at this time. I need to implement mocks for them to be unit tests. For now the tests require setup outside of the testing scaffolding.

## Theory

To test the pull through ability of this project, I test it like so:

```
+-------------------+     +----------------+     +------------+     +----------+
| cranepull_test.go | --> | ociregistry    | --> | nginx      |     | registry |
+-------------------+     | (this project) |     | 1-way 8443 | --> | no auth  |
                          |                |     | 2-way 8444 |     | 5000     |
                          |                |     +------------+     |          |
                          |                | ---------------------> +----------+
                          +----------------+         +------------+
                                   |                 | registry   |
                                   +---------------> | basic auth |
                                                     | 5001       |
                                                     +------------+
```

## Steps

1. Create a CA, and cert and key. Nginx uses the cert and key to send to the client and the CA to validate the certs received from the client
1. The clients configured by `cranepull_test.go` use the same certs so - each side can validate the other with the same CA and certs
1. Run a plain HTTP no-auth registry in a docker container on port 5000.
1. Run a plain HTTP basic auth registry in a docker container on port 5001
1. Run one nginx instance in a docker container with two `server` directives in `nginx.conf`: `listen: 8443` for 1-way TLS, and `listen: 8444` for mTLS.

With these containers running the `cranepull_test.go` tests can test the following:

1. Anonymous HTTP pull from 5000
1. Basic auth HTTP pull from 5001
1. Anonymous HTTPS 1-way insecure from 8443
1. Anonymous HTTPS 1-way secure (verify server cert) from 8443
1. Anonymous HTTPS mTLS (verify server cert / server verify client) from 8444

(I should at some point also test HTTPS + Basic Auth.)

With all three docker containers running:

```
crane pull docker.io/hello-world:latest hello-world.latest.tar
crane push hello-world.latest.tar localhost:5000/hello-world:latest
crane auth login -u ericace -p ericace localhost:5001
crane push hello-world.latest.tar localhost:5001/hello-world:latest
crane auth logout localhost:5001
```

Now both the `5000` and `5001` registries have an image to pull through into the ociregistry to support the tests.

## Running the three docker containers

These instructions assume your current working directory is the project root.

### HTTP no auth

In console 1:
```
docker run\
  --rm\
  --name registry\
  -p 5000:5000\
  registry:2.8.3
```

### HTTP basic auth

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

### Nginx for TLS termination

Remember that Nginx is configured to serve on both 8443 and 8444. In console 3:
```
docker run\
  --rm\
  --name nginx\
  --network host\
  -v $(realpath test-registry-servers/no-auth-one-way-tls/conf):/etc/nginx\
  -v $(realpath test-registry-servers/no-auth-one-way-tls/certs):/certs\
  -P\
  nginx:latest
```

At this point the tests in `cranepull_test.go` can be run.
