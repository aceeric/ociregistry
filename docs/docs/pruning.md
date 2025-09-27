# Pruning The Image Cache

The compiled server binary can be used as a CLI to prune the cache on the file system.

> The server can also be configured to prune as a continual background process. For information on that, see [Prune Configuration](configuring-the-server.md/#prune-configuration) in the _Configuring the Server_ page. There is also a REST API for ad-hoc pruning the running server's cache. See the [Administrative REST API](rest-api.md) document for information on that.

**This** page is about ad-hoc manual pruning using the server binary as a CLI.

!!! Important
    The server must be stopped while pruning using the binary as a CLI because the CLI only manipulates the file system, not the in-memory representation of the cache. Pruning removes manifest lists, manifests, and possibly blobs (more on blobs below.)

## List Images

Generally, it is expected that you will use the server binary as a CLI to list the images before deciding which images to prune. E.g.:

```
bin/ociregistry --image-path /my/image/cache list --pattern dashboard
```

Result (for example) truncated in the doc for readability:
```
docker.io/kubernetesui/dashboard-web@sha256:05ad8120...
docker.io/kubernetesui/dashboard-metrics-scraper@sha256:0cdefa04...
docker.io/kubernetesui/dashboard:v2.7.0
docker.io/kubernetesui/dashboard-metrics-scraper:1.2.2
docker.io/kubernetesui/dashboard-api:1.12.0
docker.io/kubernetesui/dashboard-web:1.6.2
docker.io/kubernetesui/dashboard-auth:1.2.4
docker.io/kubernetesui/dashboard@sha256:ca93706e...
docker.io/kubernetesui/dashboard-auth@sha256:d6dd67b7...
docker.io/kubernetesui/dashboard-api@sha256:dcc897f8...
```

## Dry Run

It is strongly recommended to use the `--dry-run` arg to develop your pruning expression. Then remove `--dry-run` to actually prune the cache. When `--dry-run` is specified, the CLI shows you exactly what will be pruned but does not actually modify the file system.

## By Pattern

Specify `--pattern` with single parameter consisting of one or more manifest URL patterns separated by commas. The patterns are Golang regular expressions as documented in the [regexp/syntax](https://pkg.go.dev/regexp/syntax) package documentation. The expressions on the command line are passed _directly_ to the Golang `regex` parser _as received from the shell_, and are matched to manifest URLs. As such, shell expansion and escaping must be taken into consideration. Simplicity wins the day here. Examples:

```shell
bin/ociregistry prune --pattern kubernetesui/dashboard:v2.7.0 --dry-run
bin/ociregistry prune --pattern docker.io --dry-run
bin/ociregistry prune --pattern curl,cilium --dry-run
```

## By Create Date/time

The `--prune-before` option accepts a single parameter consisting of a local date/time in the form `YYYY-MM-DDTHH:MM:SS`. All manifests created **before** that time stamp will be selected. Example:

```shell
bin/ociregistry prune --date 2024-03-01T22:17:31 --dry-run
```

The intended workflow is to use the CLI with `list` sub-command to determine desired a cutoff date and then to use that date as an input to the `prune` sub-command.

## Important to know about pruning

Generally, but not always, image list manifests have tags, and image manifests have digests. This is because in most cases, upstream images are multi-architecture. For example, this command specifies a tag:

```shell
bin/ociregistry prune --pattern calico/typha:v3.27.0 --dry-run
```

In this case, on a Linux/amd64 machine running the server the CLI will find **two** manifests:

```shell
docker.io/calico/typha:v3.27.0
docker.io/calico/typha@sha256:eca01eab...
```

The first is a multi-arch image list manifest, and the second is the image manifest matching the OS and Architecture that was selected for download. In all cases, only image manifests have blob references. If your search finds only an image list manifest, the CLI logic will **also** look for cached image manifests (and associated blobs) for the specified image list manifest since that's probably the desired behavior. (The blobs consume the storage.)

## Blob removal when pruning

Internally, the CLI begins by building a blob list with ref counts. As each image manifest is removed its referenced blobs have their count decremented. After all manifests are removed, any blob with zero refs is also removed. Removing an image manifest therefore won't remove blobs that are still referenced by un-pruned manifests.
