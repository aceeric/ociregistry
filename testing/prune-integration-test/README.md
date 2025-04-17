# Prune integration test

This release adds continuous background pruning. This integration test was designed to validate the feature.

## Goal

Verify that concurrent create, read, update, and delete functions correctly.

## Approach

1. Configure pruning to run continuously as a stress-test.
2. Run multiple clients concurrently to pull an image that is being pruned (`kube-scheduler:v1.31.0`).
3. Run multiple clients concurrently to pull another image that is **not** being pruned (`kube-scheduler:v1.30.0`).
4. Choose these images because they share a number of blobs but also have disjoint blobs. This validates that the blob reference counting used to prune does not corrupt an image that is not being pruned, when the non-pruned image shares blobs with an image that is being pruned.

## Steps

1. Run Docker registry on `5000` in Docker daemon
2. Load the registry with two images
   - `registry.k8s.io/kube-scheduler:v1.31.0`
   - `registry.k8s.io/kube-scheduler:v1.30.0`
3. Configure the pull-through registry to prune continuously (zero second interval) as a stress test
   - Prune matching pattern `1.31`
4. Start the pull-through registry logging to a text file
5. In three consoles, perform image pull of `localhost:8888/registry.k8s.io/kube-scheduler:v1.31.0` (is being pruned)
   - continuous - no delay
   - log to file for analysis
5. In two other consoles, perform image pull of `localhost:8888/registry.k8s.io/kube-scheduler:v1.30.0` (shared blobs, not pruned)
   - continuous - no delay
   - log to file for analysis
6. Wait a couple minutes. Pull-through registry log size reaches 650Mb
8. Stop all processes

## Log inspections and conclusions

| Inspection | Conclusion |
|-|-|
| Client pull log of 1.30 had no errors | Continuous pruning of 1.31 did not corrupt 1.30 even though they share some blobs. |
| Client pull log of 1.31 had errors | As a pull was in progress, a prune took place within the same span of time. Not a system error: concurrent CRUD handled correctly. |
| Pull-through registry server log reported errors pulling 1.31 | Expected: this is the pair error of the client puller's error. |
| Pull-through registry server log reported **no** errors pulling 1.30 | Expected as described. |

## Prune / Pull contention

Like any concurrent C/R/U/D system, if one thread **C**reates, another **R**eads, and a third **D**eletes, then we expect the following:

1. Create is atomic and at the moment it completes - the entire created item is correct, i.e. an image manifest and its blobs
2. Reads of complex objects can be affected by concurrent deletes (illustration below)
3. Concurrent deletes should also be correct at the moment they finish
4. Concurrent creates and deletes should not be allowed to interfere with each other

Any pull that interleaves with a prune in the same interval can cause the pull client to have a successful initial result - getting the manifest - followed by an unsuccessful get of a blob. In the "diagram" below the dashed lines indicate elapsed time:

```
puller -> get manifest ------------------------------------------> get blob (not found)
pruner -----------------> lock manifest -> delete blobs -> sleep
```

The reason is that an image pull by a client is simply a series of REST calls over time and these calls interleave other concurrent activity.

## Conclusion

The test shows that the server is correctly handling the concurrent CRUD it has to support. If a pull client has auto-retry, and the pull-through server is not air-gapped then it should "heal", meaning pruned images will simply get re-pulled by clients if they are needed after pruning even if an error occurs during a first pull by the client.
