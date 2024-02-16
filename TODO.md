# Features

1. How to handle "latest". On every pull would have to store the upstream digest, save it with the image. Then on all subsequent pulls, re-get the upstream digest (HEAD) and compare. If different, re-download else serve from cache.
2. Modularize
3. Use structs more to carry state
4. Unit tests
5. Each handler in its own file: handleV2Auth, handleV2Default, handleV2GetOrgImageBlobsDigest, handleV2OrgImageManifestsReference
6. Propagate errors better
7. Improve logging. timestam left, etc. Add file/line:
   - https://stackoverflow.com/questions/58198896/how-to-get-file-and-function-name-for-loggers
   - containerd/vendor/github.com/containerd/log/context.go
8. e2e tests with docker
9. Config reloader. Support program args in config to change log level without restart
10. Helm chart


(need to support get image by digest returning as list)

> HEAD manifest org/image/tag
  manifest list + digest <
> GET manifest digest
  manifest + digest      <
> GET blob
  blob                   <

object model

manifest list
  org / image / tag
  list of manifest (each is same tag)

image manifest
  tag (or i-am-a-digest if pulled by digest)
  blobs
  config

directory structure
manifest_lists dir
  org / image / tag dirs
    digest files

example
manifest_lists/calico/node/v3.27.0/manifest.json
image_manifests/calico/node/v3.27.0/manifest.json

struct imageRef {
   registry ?????
   org
   image
   tag
}

struct imageManifestEntry {
  "schemaVersion": 2,
   "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
   "config": {
      "mediaType": "application/vnd.docker.container.image.v1+json",
      "size": 2027,
      "digest": "sha256:1843802b91be8ff1c1d35ee08461ebe909e7a2199e59396f69886439a372312c"
   },
   "layers": []
}

struct imageManifest {
   imageRef
   imageManifestEntry
   digest
}

struct manifestListEntry{
     "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
      "size": 2214,
      "digest": "sha256:21498c24d6e850a70a6d68362ebb1b5354fb5894b7c09b0a7085ed63227a72f5",
      "platform": {
        "architecture": "arm",
        "os": "linux",
        "variant": "v7"
      }
}

struct manifestList {
   imageRef
   []manifestListEntry
   digest
}

logic

create pullRequest
if pullRequest.isCached
  pullRequest.fromCache
pullRequest.pull




// queue requests - make waiters wait for identical requets
struct pullRequest {
   type (by tag or by digest)
     if tag - return manifestlist
     if digest - return imageManifest
   org/image
   tag
   digest
   remote registry
}

maps:

manifestmap
digest = imageManifest