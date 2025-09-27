# Airgap Considerations

As stated in the Overview, one objective of the project is to support running Kubernetes in air-gapped and DDIL environments. To accomplish this, you will likely adopt one of two approaches:

1. Stand the cluster up in a connected environment, pre-load the distribution server with all required images, then disconnect and ship the cluster to its edge location.
2. Ship the cluster to its edge location and pre-load the distribution server there using stable comms. If comms are later degraded or lost then the required images remain cached.

There are two general ways to run the distribution server in support of this objective. These are now discussed.

## Kubernetes Workload

Using the helm chart described elsewhere in this guide you can install the server as a Kubernetes `Deployment`, and configure containerd on each Node to access the distribution server on a `NodePort`. In order to make this work, two things are needed:

1. The cluster needs persistent storage that is redundant and reliable. This means that when you down the cluster, ship it, and start it up at the edge, the persistent storage has to come up with the image cache intact.
2. You need a tarball of the distribution server container image available in the edge environment for the following reason: when you start the cluster for the first time, if containerd has been configured to pull from the distribution server, the distribution server image may not be in cache and so that Pod won't start. This is a classic deadlock scenario. Therefore your cluster startup procedure will be to load the distribution server image tarball into each cluster's containerd cache before starting the cluster. Now Kubernetes can start the distribution server workload which in turn can serve images from its cache. Ideally your Kubernetes distribution of choice will support the ability to pre-load containerd from an image tarball at a configurable file system location.

## `systemd` Service

You can run the distribution server as a `systemd` service on one of the cluster nodes. This avoids the deadlock scenario associated with running the distribution server as a Kubernetes workload because the service will come up when the cluster instances are started and will therefore immediately be available to serve cached images.

The risk is that the image cache only exists on one cluster instance. If this instance is lost, then the cluster is down until comms are re-established. This can be mitigated by replicating the cache across multiple cluster nodes. There are multiple tools available to support this. For example [Syncthing](https://syncthing.net/) could be run on each node to ensure that each node has a full copy of the image cache. If the node running the distribution server is lost then the nodes can have their containerd configuration modified to point to any one of the other nodes, and the distribution server systemd service can be started on that node.
