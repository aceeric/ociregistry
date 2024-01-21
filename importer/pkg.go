/*
Package importer runs a filesystem notifier on the "images" directory. (The
"images" directory defaults to <project root>/images and can be explicitly
specified with the --images arg.) Whenever a tarball is placed there, the
filename is parsed and the tarball is inflated into a directory structure
that the registry server supports. The file name parsing uses the plus sign
('+') as the component separator. Here is the canonical use case:

 1. Export an image from the containerd cache:
    image=docker.io/calico/apiserver:v3.27.0
    tarfile=docker.io+calico+apiserver+v3.27.0.tar
    ctr -n k8s.io -a /var/run/containerd/containerd.sock image export $tarfile $image
    (or 'crane pull' or 'docker save' from a registry)
 2. Place the tar file into the "images" directory
 3. ls -l images/calico/apiserver/v3.27.0/
 4. This image can now be pulled/run:
    docker run localhost:8080/calico/apiserver:v3.27.0

It is not necessary to include a registry (docker.io) in the filename and in fact common
registries are ignored. (See the 'ignore' var in extractor.go.)
*/
package importer
