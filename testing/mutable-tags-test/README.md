# Test mutable tags

## Begin

Expect a locally tagged image that can be pushed into `registry` running as a container. E.g.:

`docker images`:

```text
REPOSITORY   TAG       IMAGE ID       CREATED         SIZE
registry     3.0.0     e4e570676819   19 months ago   57.7MB
```

## Host

This example expects the current hostname is `ubuntu.me`:

```sh
nslookup ubuntu.me
Server:		127.0.0.53
Address:	127.0.0.53#53

Name:	ubuntu.me
Address: 192.168.0.49
```

## Re-tag the same image

E.g.:

```text
docker tag registry:3.0.0  ubuntu.me:5000/registry:3.0.0
docker tag registry:3.0.0  ubuntu.me:5000/registry:latest
docker tag registry:3.0.0  ubuntu.me:5000/registry:nightly
```

Now, `docker images` shows:

```
REPOSITORY                TAG       IMAGE ID       CREATED         SIZE
registry                  3.0.0     e4e570676819   19 months ago   57.7MB
ubuntu.me:5000/registry   3.0.0     e4e570676819   19 months ago   57.7MB
ubuntu.me:5000/registry   latest    e4e570676819   19 months ago   57.7MB
ubuntu.me:5000/registry   nightly   e4e570676819   19 months ago   57.7MB
```

## Run docker registry as a container

On **this** host:

```sh
docker run\
  --name registry\
  --detach\
  --publish 5000:5000\
  registry:3.0.0
```

## Push to registry

```sh
docker push ubuntu.me:5000/registry:3.0.0
docker push ubuntu.me:5000/registry:latest
docker push ubuntu.me:5000/registry:nightly
```

## Confirm tags

`curl -s http://ubuntu.me:5000/v2/registry/tags/list | jq` shows:

```json
{
  "name": "registry",
  "tags": [
    "3.0.0",
    "latest",
    "nightly"
  ]
}
```

## Run ociregistry

> Note: the `noCache` key  is commented out

```sh
../../bin/ociregistry --config-file $PWD/config.yaml serve
```

## Pull all three images

To the local file system:

```sh
$ imgpull localhost:8888/ubuntu.me:5000/registry:3.0.0   registry.3.0.0.tar   --scheme http
image "localhost:8888/registry:3.0.0" saved to "registry.3.0.0.tar" in 225.34739ms
```

```sh
$ imgpull localhost:8888/ubuntu.me:5000/registry:latest  registry.latest.tar  --scheme http
image "localhost:8888/registry:latest" saved to "registry.latest.tar" in 113.677563ms
```

```sh
$ imgpull localhost:8888/ubuntu.me:5000/registry:nightly registry.nightly.tar --scheme http
image "localhost:8888/registry:nightly" saved to "registry.nightly.tar" in 115.844006ms
```

## Inspect pulled images

```sh
ls -l *.tar
```

Output:

```sh
-rw-rw-r-- 1 eace eace 18558464 Oct 28 20:05 registry.3.0.0.tar
-rw-rw-r-- 1 eace eace 18558464 Oct 28 20:05 registry.latest.tar
-rw-rw-r-- 1 eace eace 18558464 Oct 28 20:05 registry.nightly.tar
```

##   Inspect ociregistry image cache

```sh
curl localhost:8888/cmd/image/cache?digestLen=10
```

Output:

```text
ENTRIES:

ENTRY URL                        URL DIGEST     MANIFEST DIGEST  MANIFEST TYPE
ubuntu.me:5000/registry:3.0.0    2d0b6e447b...  025666893b...    image
ubuntu.me:5000/registry:latest   8998391b0e...  025666893b...    image
ubuntu.me:5000/registry:nightly  5370e2f78c...  025666893b...    image

ALIASES:

ALIAS URL / TAGS
ubuntu.me:5000/registry@sha256:025666893b...
    ubuntu.me:5000/registry:3.0.0
    ubuntu.me:5000/registry:latest
    ubuntu.me:5000/registry:nightly
```

## Cache on the file system

```sh
ls -l /tmp/mutable-test/manifests/
```

Output:

```
total 12
-rw-r--r-- 1 eace eace 3349 Oct 28 20:05 2d0b6e447b00dd8523ef037cd467f5fc4c3e2c3cf5dfea51a2595d9be14a188f
-rw-r--r-- 1 eace eace 3351 Oct 28 20:05 5370e2f78c27aa52b472ccef90e8805d4cd6b0514c0acbeeb930f272ef7931f7
-rw-r--r-- 1 eace eace 3350 Oct 28 20:05 8998391b0ed2b098b5316d4d7af4fd4240be85ee6d299646683b36ccc4df859d
```

## Validate filenames

File names are hashes of **image URLS**:

```sh
for url in  ubuntu.me:5000/registry:3.0.0 ubuntu.me:5000/registry:nightly ubuntu.me:5000/registry:latest; do\
  echo -n $url | sha256sum | awk '{print $1}'; done
```

Output:

```text
2d0b6e447b00dd8523ef037cd467f5fc4c3e2c3cf5dfea51a2595d9be14a188f
5370e2f78c27aa52b472ccef90e8805d4cd6b0514c0acbeeb930f272ef7931f7
8998391b0ed2b098b5316d4d7af4fd4240be85ee6d299646683b36ccc4df859d
```

## Get image URLS

These are in the `ManifestHolder` resources cached to the file system.

```sh
find /tmp/mutable-test/manifests/ -maxdepth 1 -type f | xargs jq -r '[input_filename, .imageUrl] | @tsv' | sort -k1
```

Output:

```text
/tmp/mutable-test/manifests/2d0b6e447b00dd8523ef037cd467f5fc4c3e2c3cf5dfea51a2595d9be14a188f   ubuntu.me:5000/registry:3.0.0
/tmp/mutable-test/manifests/5370e2f78c27aa52b472ccef90e8805d4cd6b0514c0acbeeb930f272ef7931f7   ubuntu.me:5000/registry:nightly
/tmp/mutable-test/manifests/8998391b0ed2b098b5316d4d7af4fd4240be85ee6d299646683b36ccc4df859d   ubuntu.me:5000/registry:latest
```

## Stop Server

CTRL-C

## Enable mutable tags

Uncomment `noCache` in `config.yaml`:

```yaml
logLevel: debug
imagePath: /tmp/mutable-test
#noCache:                    <--- HERE
#  - nightly                 <--- HERE
port: 8888
registries:
  - name: ubuntu.me:5000
    description: docker registry running as a container in docker daemon
    scheme: http
```

## Start server

As before:

```sh
../../bin/ociregistry --config-file $PWD/config.yaml serve
```

## Pull

Pull the three images. Watch ociregistry logs. Observe the `nightly` is always pulled from upstream, but the others are pulled from cache.
