# Test Driver

The test driver CLI supports load testing. It tests:

1. Pulling _through_ an _Ociregistry_ server _from_ an upstream registry
2. Pulling only cached images from an _Ociregistry_ server 

The CLI supports gradually scaling up and then scaling down the number of concurrent goroutines pulling from the server, and tallies the pull rate over time logging either to stdout or to a file.

Running with no args will display help. The following options are supported. Details are provided below the summary table:

| Arg | Description | Other info |
|-|-|-|
| `--pullthrough-url VALUE`   | Pull through URL (_Ociregistry_ server under test) | Required |
| `--registry-url VALUE`      | Upstream registry URL (what _Ociregistry_ server is pulling **from**) | Required |
| `--patterns VALUE`          | Comma-separated batching go regex patterns | Can specify multiple. Aat least one is required. `*` is a valid pattern. |
| `--iteration-seconds VALUE` | Seconds between iterations | Default: 60 |
| `--tally-seconds VALUE`     | Interval for tallying pull rate | Default: 15 |
| `--metrics-file VALUE`      | Path to metrics output file | `stdout` if omitted |
| `--log-file VALUE`          | Path to log file | `stdout` if omitted |
| `--filter VALUE`            | Repo filter | Optional go regex expression to create a smaller test set from the upstream registry. |
| `--dry-run`                 | Does everything except actually pull from the registry | Boolean - default false |
| `--prune   `                | Enables pruning. | Boolean - default false |
| `--shuffle`                 | If specified, then shuffles the image list between pull passes | Boolean - default false |

> Arguments can be specified in two ways: `--arg=value` or `--arg value`.

## Example
```
go run .\
  --pullthrough-url=ubuntu.me:8080\
  --registry-url=ubuntu.me:5000\
  --patterns=-0001,-0002,-0003,-0004,-0005\
  --iteration-seconds=20\
  --tally-seconds=5\
  --filter=:v9\
  --dry-run
```

## Detailed arg documentation

| Arg | Description |
|-|-|
| Pull through url  | This is the url of the _Ociregistry_ server under test. Because the test driver currently only supports HTTP, _Ociregistry_ **must** be listening on HTTP - and - you will have to configure the _Ociregistry_ under test to pull from the upstream Registry (below) on HTTP. See the configuration exemplar below this table. Since _Ociregistry_ runs on 8080 by default, then: `--pullthrough-url=ubuntu.me:8080` |
| Registry url      | This is the "upstream" registry the _Ociregistry_ server will pull _from_. It **must** be listening on HTTP. For example, I run the `registry` image in a container on port 5000, so: `registry-url=ubuntu.me:5000`|
| Patterns          | The patterns define the parallelism. For example, assume the upstream registry being pulled _from_ has 1000 images and each image name contains a component like `-0001`, `-0002`, etc. E.g. `testimage-<random number>-0001:<tag>`, `testimage-<random number>-0002:<tag>`, etc. These patterns are specified on the command line as a comma-separated string. The test driver will start one goroutine to pull the `-0001` images. Then the second goroutine would pull the `-0002` images, and so on. So 10 patterns will result in 10 concurrently running goroutines each pulling their own filter. Note that `*,*,*,*,*` is a valid value for this arg. |
| Iteration Seconds | This is the duration that the test driver will run each puller goroutine before the next goroutine is scaled up or down. Therefore, a test with 5 patterns, and 60 iteration seconds would take ten minutes: 5 minutes to scale up, and 5 minutes to scale down. |
| Tally Seconds     | This is the sample interval used to determine the pull rate across all puller goroutines. Its the same idea as a prometheus scrape interval. Every _this value_ seconds the test driver will emanate the total pull rate across all puller goroutines. |
| Metrics File      | The pull rate metrics are written to this file if specified, otherwise written to `stdout`. |
| Log File          | Log messages are written to this file if specified, otherwise written to `stdout`. |
| Filter            | Allows to filter the images in the upstream registry (that _Ociregistry_ is pulling _from_) **before** the test starts. E.g. say your upstream registry has 1000 images and you know that the tags allow you to get a subset of that. You specify that filter here to get a smaller starting set. Otherwise, all 1000 images will be used for the test. |
| Dry Run           | Does everything except pull through the _Ociregistry_ server under test. (Instead, sleeps for a few milliseconds so as not to peg the CPU.) Also skips pruning. |
| Prune             | If `true` then each goroutine will prune the images it pulled through the _Ociregistry_ server on each pass before starting the next pass. This essentially forces the _Ociregistry_ server under test to always be going to the upstream registry. |
| Shuffle           | If `true` then each goroutine will shuffle the image list on each iteration. |

## Configuring _Ociregistry_

The test driver only supports HTTP at this time. The test driver gets its test set directly from the upstream registry that _Ociregistry_ is pulling from. After getting the test set, then the test driver pulls through _Ociregistry_ which pulls from the upstream. So since the upstream has to serve on HTTP, you have to configure _Ociregistry_ to pull from the upstream over HTTP. The configuration example below accomplishes this via the `scheme` key:
```
cat <<EOF > /tmp/test-config.yaml
registries:
  - name: ubuntu.me:5000
    description: The docker registry with the test images for Ociregistry to pull from
    scheme: http
EOF
```

Then: `ociregistry --config-file /tmp/test-config.yaml serve`

By default, _Ociregistry_ serves on HTTP so if run that way then it is automatically reachable by the test driver.
