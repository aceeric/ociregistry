# Administrative REST API

The following REST endpoints are supported for administration of the image cache. Note - the output of the commands in some cases is columnar. Pipe through `column -t` to columnize.

## `/cmd/prune`

Prunes the in-memory cache and the file system while the server is running.

| Query param | Description |
|-|-|
| `type` | Valid values: `accessed`, `created`, `pattern`. |
| `dur` | A duration string. E.g.: `30d`. Valid time units are `d`=days, `m`=minutes, and `h`=hours.  If `type` is `accessed`, then images that have not been accessed within the duration are pruned. If `type` is `created`, then images created earlier than the duration ago are pruned. (I.e.: created more than 30 days ago.) If `type` is `pattern`, then `dur` is ignored. |
| `expr` | If `type` is `pattern`, then a manifest URL pattern like `calico`, else ignored. Multiple patterns can be separated by commas: `foo,bar`|
| `count` | Max manifests to prune. Defaults to `50`. `-1` means no limit. |
| `dryRun` | If `true` then logs messages but does not prune. **Defaults to false, meaning: will prune by default.** |

Example: `curl -X DELETE "http://hostname:8080/cmd/prune?type=created&dur=10d&count=50&dryRun=true"`

Explanation: Prunes manifests created (initially downloaded) more than 10 days ago. Only prune a max of 50. Since _dry run_ is true, doesn't actually prune - only show what prune would do.

## `/cmd/image/list`

Lists image manifests, and the blobs that are referenced by the selected manifests.

| Query param | Description |
|-|-|
| `pattern` | Comma-separated go regex expressions of manifest URL(s). |
| `digest` | Digest (or substring) of any blob referenced by the image. (Not the manifest digest!) |
| `count` | Max number of manifests to return. Defaults to `50`. `-1` means no limit. |

Example: `curl "http://hostname:8080/cmd/image/list?pattern=docker.io&count=10"`

Explanation: List a max of 10 image manifests with `docker.io` in the URL.

## `/cmd/blob/list`

Lists blobs and ref counts.

| Query param | Description |
|-|-|
| `substr` | Digest (or substring) of a blob. |
| `count` | Max number of manifests to return. Defaults to `50`. `-1` means no limit. |

Example: `curl "http://hostname:8080/cmd/blob/list?substr=56aebe9b&count=10"`

## `/cmd/manifest/list`

List manifests.

| Query param | Description |
|-|-|
| `pattern` | Comma-separated go regex expressions of manifest URLs. |
| `count` | Max number of manifests to return. Defaults to `50`. `-1` means no limit. |

Example: `curl "http://hostname:8080/cmd/manifest/list?pattern=calico,cilium&count=10"`

## `/cmd/stop`

Stops the server.
