/*
Package importer runs a filesystem notifier on the "images" directory. (See the
'--image-path' arg 'cmd/ociregistry.go'). Whenever a tarball is placed in the
images directory, the manifest in the tarball is parsed to get the manifest tag,
and then the tarball contents are inflated into a directory structure derived from
the tag. Here is the canonical use case:

 1. Export an image from the containerd cache:
    image=docker.io/calico/apiserver:v3.27.0
    tarfile=tarfile.tar
    ctr -n k8s.io -a /var/run/containerd/containerd.sock image export $tarfile $image
 2. Place the tar file into the "images" directory and wait a second
 3. ls -l images/docker.io/calico/apiserver/v3.27.0/
 4. This image can now be pulled or run:
    docker run localhost:8080/calico/apiserver:v3.27.0
*/
package importer
