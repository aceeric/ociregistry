# Configure and run docker registry with no auth

This is the same as `basic-auth-no-tls` except the registry server is started with no auth.

## Start the registry with no auth

```
docker run --rm -p 5001:5000 --name registry registry:2.8.3
```

## In another terminal

```
crane push docker.io+infoblox+dnstools+latest.tar localhost:5001/infoblox/dnstools:latest
```

SUCCESS

## Test

```
crane pull localhost:5001/infoblox/dnstools:latest deleteme.tar
```

SUCCESS
