


https://www.docker.com/blog/how-to-use-your-own-registry-2/

sudo systemctl start docker

docker run -d -p 5000:5000 --name registry registry:2.8.3

docker tag localhost:8080/hello-world:latest localhost:5000/hello-world:latest
docker push localhost:5000/hello-world:latest

start containerd on /run/containerdtest/containerd.sock (TEST)
configure crictl.yaml to match

sudo ./crictl images
(see logs in containerd)

sudo crictl pull localhost:5000/hello-world:latest

sudo ./crictl pull localhost:5000/hello-world:latest
Image is up to date for sha256:d2c94e258dcb3c5ac2798d32e1249e42ef01cba4841c2234249495f87264ac5a

sudo ./crictl images
IMAGE                        TAG                 IMAGE ID            SIZE
localhost:5000/hello-world   latest              d2c94e258dcb3       3.56kB
