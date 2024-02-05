# Configure and run docker registry with basic auth

Based on: https://learning-ocean.com/tutorials/docker/docker-docker-registry-basic-authentication

## Prepare

Install `htpasswd` utility:
```
sudo apt install apache2-utils
```

## Create credentials file
```
htpasswd -bnB ericace ericace > auth/htpasswd
```

## Verify
```
cat auth/htpasswd 
```

## Result
```
ericace:$2y$05$4B7xWnrLxZiCJkG/kBIYkufcT9yPg3C3leUQT9MMxqoOP6geYhmd2
```

## Run the registry
```
docker run --rm -p 5001:5000 --name registry\
 -v $(pwd)/auth:/auth\
 -e REGISTRY_AUTH=htpasswd\
 -e REGISTRY_AUTH_HTPASSWD_REALM="Registry Realm"\
 -e REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd\
 registry:2.8.3
```

## In another terminal
```
crane push docker.io+infoblox+dnstools+latest.tar localhost:5001/infoblox/dnstools:latest
```

## Result
```
Error: HEAD http://localhost:5001/v2/infoblox/dnstools/manifests/latest: unexpected status code 401 Unauthorized (HEAD responses have no body, use GET for details)
```

This confirms basic auth is reqruied.

## Log in
```
crane auth login localhost:5001 -u ericace -p ericace
```

## Result
```
2024/02/04 17:50:29 logged in via /home/eace/.docker/config.json
```

## Push again
```
crane push docker.io+infoblox+dnstools+latest.tar localhost:5001/infoblox/dnstools:latest
```

SUCCESS

## Confirm
```
crane auth logout localhost:5001
```

## Result
```
2024/02/04 17:51:22 logged out via /home/eace/.docker/config.json
```

## Possible to pull while logged out?
```
crane pull localhost:5001/infoblox/dnstools:latest deleteme.tar
```

## No
```
Error: GET http://localhost:5001/v2/infoblox/dnstools/manifests/latest: UNAUTHORIZED: authentication required; [map[Action:pull Class: Name:infoblox/dnstools Type:repository]]
```
