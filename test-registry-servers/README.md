# Pull-through testing

This directory tests the various ways that the project registry server can connect to upstream registries:

1. Anonymous
2. Basic Auth
3. One-way TLS (secure and insecure)
4. mTLS

These steps use the Go Container Registry *crane* utility: https://github.com/google/go-containerregistry/blob/main/cmd/crane/doc/crane.md as well as Docker and the Docker CLI.

## Approach

Create two registries running in Docker containers, and one Nginx instance also in Docker for TLS termination. Load a small image into each registry. Then curl the project registry server with various configurations the cause it to connect to one of the three upstream registries:

```
+--------------+     +----------------+     +------------+     +----------+
| test-script  | --> | ociregistry    | --> | nginx      |     | registry |
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

## Run the test

```
./test-script
```

## Expected results

```
PASS: http-anon.yaml
PASS: http-basic-auth.yaml
PASS: https-one-way-anon-secure-fails.yaml
PASS: https-one-way-anon-insecure.yaml
PASS: https-one-way-anon-secure.yaml
PASS: https-mtls-anon-secure.yaml
```
