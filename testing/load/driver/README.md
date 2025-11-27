# Test Driver

The test driver pulls _through_ an Ociregistry server under test _from_ an upstream registry and tallies the pull rate. It supports gradually scaling up and then scaling down the number of concurrent goroutines pulling through the Ociregistry server under test.

Running with no args will display help. The following options are supported. Details are provided below the summary table:

| Arg | Description | Other info |
|-|-|-|
| `--pullthrough-url VALUE`   | Pull through (ociregistry server) URL | required |
| `--registry-url VALUE`      | Upstream registry URL | required |
| `--patterns VALUE`          | Comma-separated batching patterns | Can specify multiple (at least one required) |
| `--iteration-seconds VALUE` | Seconds between iterations | default: 60 |
| `--tally-seconds VALUE`     | Interval for tallying pull rate | default: 15 |
| `--metrics-file VALUE`      | Path to metrics output file | stdout if omitted |
| `--log-file VALUE`          | Path to log file | stdout if omitted |
| `--filter VALUE`            | Repo filter | optional to create a smaller test set |
| `--dry-run`                 | Does everything except actually pull from the registry | boolean - default false |
| `--prune   `                | Enable pruning. | (boolean - default false) |
| `--shuffle`                 | If specified, then shuffles the image list between pull passes | boolean - default false |

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
| Pull through url  | This is the url of the Ociregistry server under test. Because the test driver currently only supports HTTP, you will have to configure the Ociregistry under test to pull from the Registry (below) on HTTP. |
| Registry url      | This is the registry the Ociregistry server will pull _from_. It **must** be listening on HTTP. |
| Patterns          | The patterns define the parallelism. For example, assume the upstream registry being pulled _from_ has 1000 images and each image name contains a component like `-0001`, `-0002`, etc. E.g. `testimage-<random number>-0001:<tag>`, `testimage-<random number>-0002:<tag>`, etc. These patterns are specified on the command line as a comma-separated string. The test driver will start one goroutine to pull the `-0001` images. Then the second goroutine would pull the `-0002` images, and so on. So 10 patterns will result in 10 concurrent pulls. |
| Iteration Seconds | The is the duration that the driver will run each puller goroutine before the next goroutine is scaled up or down. Therefore, a test with 5 patterns, and 60 iteration seconds (below) would take ten minutes: 5 minutes to scale up, and 5 minutes to scale down. |
| Tally Seconds     | This is the sample interval used to determine the pull rate across all puller goroutines. Its the same idea as a prometheus scrape interval. Every this many seconds the driver will emanate the total pull rate across all puller goroutines. |
| Metrics File      | The pull rate is written to this file if specified, otherwise written to stdout. |
| Log File          | Log messages are written to this file if specified, otherwise written to stdout. |
| Filter            | Allows to filter the images in the upstream registry (that Ociregistry is pulling _from_) **before** the test starts. E.g. say your upstream registry has 1000 images and you know that the tags allow you to get a subset of that. You specify that filter here to get a smaller starting set. Otherwise, all 1000 images will be used for the test. |
| Dry Run           | Does everything except pull through the Ociregistry server under test. (Instead, sleeps for a few milliseconds so as not to peg the CPU.) |
| Prune             | If `true` then each goroutine will prune the images it pulled through the Ociregistry server on each pass before starting the next pass. This essentially forces the Ociregistry server under test to always be going to the upstream registry. |
| Shuffle           | If `true` then each goroutine will shuffle the image list on each iteration. |
