1. cache  migration (serve and migrate-cache)
2. Strict config parsing
3. Prune dryrun -> dryRun
   - dryRun: no also means false.
   - No config file: serve can only enable the pruner from the config file, so dry run doesn’t matter there.
   - The offline prune command: it still uses its --dry-run flag, which defaults to false (opposite)
   - The REST endpoint: DELETE /cmd/prune already defaults to a dry run.
   - Prune dry run is now the default. Scheduled prunes only log what they would remove unless the configuration
     has pruneConfig.dryRun: false. Installations that prune today must add dryRun: false after upgrading, or
     pruning will stop
4. --no-cache


TODO
 - load test and metrics
 - no cache testing - latest / nightly
 - Review 3-way consistency between cmd rest api, server config  and sub-commands like prune
 - Review documentation


## Cache design

If

imgpull localhost:8888/docker.io/hello-world:latest deleteme.tar --scheme http

Then

var mc (is the manifest cache) =
  entries map[cache.imageUrl]imgpull.ManifestHolder = [
    "docker.io/library/hello-world:latest"            : imgpull.ManifestHolder { (inside here digest is c567b231 ) }, <- manifest list
    "docker.io/library/hello-world@sha256:d1a8d0a4...": imgpull.ManifestHolder { (inside here digest is d1a8d0a4 ) }  <- image manifest
  ]
  aliases map[url]cache.tagSet = [
    "docker.io/library/hello-world@sha256:c567b231...": ["docker.io/library/hello-world:latest": {},
  ], 

So

IMAGE_MANIFEST_DIGEST=d1a8d0a4eeb63aff09f5f34d4d80505e0ba81905f36158cc3970d8e07179e59e
imgpull localhost:8888/docker.io/hello-world@sha256:$IMAGE_MANIFEST_DIGEST deleteme.${IMAGE_MANIFEST_DIGEST:0:8}.tar --scheme http

$ ls -l delete*.tar
-rw-rw-r-- 1 eace eace 6656 Oct 28 18:41 deleteme.d1a8d0a4.tar
-rw-rw-r-- 1 eace eace 6656 Oct 28 18:31 deleteme.tar

Same files

$ tar -xf deleteme.tar manifest.json --to-stdout | jq .[0].repoTags[0]
"docker.io/hello-world:latest"

$ ls -lrt /var/lib/ociregistry/manifests/
total 32

-rw-r--r-- 1 eace eace 26190 Oct 28 18:39 05c465a76e627711836aef6167f264ee2114c8ef9eda772245231ecc412cc788
-rw-r--r-- 1 eace eace  2753 Oct 28 18:41 a1354e9c3499cba2bfb90aacd433a5d83ade06f0bae20041db72fc58078af4b4


echo -n docker.io/library/hello-world:latest | sha256sum | awk '{print $1}' && \
echo -n docker.io/library/hello-world@sha256:d1a8d0a4eeb63aff09f5f34d4d80505e0ba81905f36158cc3970d8e07179e59e  | sha256sum | awk '{print $1}'

05c465a76e627711836aef6167f264ee2114c8ef9eda772245231ecc412cc788
a1354e9c3499cba2bfb90aacd433a5d83ade06f0bae20041db72fc58078af4b4

IMAGE_LIST_MANIFEST_DIGEST=c567b231f65614688711a75f31518a064258a51d442117a3e771a3a8fdba78ef
imgpull localhost:8888/docker.io/hello-world@sha256:$IMAGE_LIST_MANIFEST_DIGEST deleteme.${IMAGE_LIST_MANIFEST_DIGEST:0:8}.tar --scheme http

$ ls -l delete*.tar
-rw-rw-r-- 1 eace eace 6656 Oct 28 19:11 deleteme.c567b231.tar
-rw-rw-r-- 1 eace eace 6656 Oct 28 18:41 deleteme.d1a8d0a4.tar
-rw-rw-r-- 1 eace eace 6656 Oct 28 19:08 deleteme.tar

pulling the image manifest by digest did the same as pulling by tag - it found the cached manifest list, then returned it to the caller, the caller picked the digest of the image manifest and requested it and it was returned.


$ curl localhost:8888/cmd/image/cache?digestLen=20
ENTRIES:

ENTRY URL                                                     URL DIGEST               MANIFEST DIGEST          MANIFEST TYPE
docker.io/library/hello-world:latest                          05c465a76e627711836a...  c567b231f65614688711...  image list
docker.io/library/hello-world@sha256:d1a8d0a4eeb63aff09f5...  a1354e9c3499cba2bfb9...  d1a8d0a4eeb63aff09f5...  image

ALIASES:

ALIAS URL / TAGS
docker.io/library/hello-world@sha256:c567b231f65614688711...
    docker.io/library/hello-world:latest
