# Administrative REST API

The following REST endpoints are supported for administration of the image cache. Note - the output of the commands in some cases is columnar. Pipe through `column -t` to columnize.

## `/cmd/prune`

Prunes the in-memory cache and the file system while the server is running. Since pruning via the API is concurrent with other activity in the server this command can return 503 with _prune not started: upstream pulls in progress, try again later_ when upstream pulls run longer than 10 seconds. In other words while a pull from an upstream is in progress, pruning is held for a timeout period.

| Query param | Description |
|-|-|
| `type` | Valid values: `accessed`, `created`, `pattern`. |
| `dur` | A duration string. E.g.: `30d`. Valid time units are `d`=days, `m`=minutes, and `h`=hours. (Really every go duration is allowed but only the ones shown really make sense...) If `type` is `accessed`, then images that have not been accessed within the duration are pruned. If `type` is `created`, then images created earlier than the duration ago are pruned. (I.e.: created more than 30 days ago.) If `type` is `pattern`, then `dur` is ignored. |
| `expr` | If `type` is `pattern`, then a manifest URL pattern like `calico`, else ignored. Multiple patterns can be separated by commas: `foo,bar`|
| `count` | Max manifests to prune. Defaults to `50`. `-1` means no limit. |
| `dryRun` | Defaults to `true`, meaning nothing will actually prune. Set to `false` to actually prune. |

Example:

```shell
curl -X DELETE "http://hostname:8080/cmd/prune?type=created&dur=10d&count=50&dryRun=false"
```

Explanation: Prunes manifests created (initially downloaded) more than 10 days ago. Only prune a max of 50. Since _dry run_ is false, actually prunes.

## `/cmd/image/list`

Lists image manifests, and the blobs that are referenced by the selected manifests.

| Query param | Description |
|-|-|
| `pattern` | Comma-separated go regex expressions of manifest URL(s). |
| `digest` | Digest (or substring) of any blob referenced by the image. (Not the manifest digest!) |
| `count` | Max number of manifests to return. Defaults to `50`. `-1` means no limit. |

Example:
```shell
curl "http://hostname:8080/cmd/image/list?pattern=docker.io&count=10"
```

Explanation: List a max of 10 image manifests with `docker.io` in the URL.

## `/cmd/blob/list`

Lists blobs and ref counts.

| Query param | Description |
|-|-|
| `substr` | Digest (or substring) of a blob. |
| `count` | Max number of manifests to return. Defaults to `50`. `-1` means no limit. |

Example:
```shell
curl "http://hostname:8080/cmd/blob/list?substr=56aebe9b&count=10"
```

## `/cmd/manifest/list`

List manifests.

| Query param | Description |
|-|-|
| `pattern` | Comma-separated go regex expressions of manifest URLs. |
| `count` | Max number of manifests to return. Defaults to `50`. `-1` means no limit. |

Example:
```shell
curl "http://hostname:8080/cmd/manifest/list?pattern=calico,cilium&count=10"
```

## `/cmd/stop`

Stops the server.

## `/cmd/image/cache`

Dumps the in-mem manifest cache in its internal structure. Intended as a diagnostic tool.

| Query param | Description |
|-|-|
| `digestLen` | Shortens the digests from their default 64-byte (hex) lengths. |

Example:
```shell
curl "http://hostname:8080/cmd/image/cache?digestLen=20"
```

Example (abbreviated) output:

```text
ENTRIES:

ENTRY URL                       URL DIGEST               MANIFEST DIGEST          MANIFEST TYPE
docker.io/bitnami/nginx:1.26.2  2b023fc0f8cceea6950b...  84c926c7c58d73f7d77d...  image list

ALIASES:

ALIAS URL / TAGS
docker.io/bitnami/nginx@sha256:84c926c7c58d73f7d77d...
    docker.io/bitnami/nginx:1.26.2
```

The command lists the two portions of the in-mem manifest cache. The cache is structured in two parts:

- **Entries:** list each unique manifest URL, along with the URL digest, the manifest digest, and the manifest type. Manifests are stored on the filesystem by their _url digest_. So if it were ever necessary to find the manifest on the filesystem for `docker.io/bitnami/nginx:1.26.2` you would look in `/var/lib/ociregistry/manifests/2b023fc0f8cceea6950b...`
- **Aliases:** lists manifests by digest, and all the tag URLS that pull **that** manifest. This supports the ability to have `mycorp.io/foo:nightly`, `mycorp.io/foo:latest`, `mycorp.io/foo:v1.0.0` all referring to the same manifest by _manifest_ digest.