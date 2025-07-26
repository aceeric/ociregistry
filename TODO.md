# In progress

- Multi-platform ARM64
- HEAD the upstream on latest and don't pull if digest is unchanged.

# TODO

- Server base url (for ingress)
- Readthedocs
  - concurrency design
  - volumetrics
- Consider https://github.com/OpenAPITools/openapi-generator
- Instrumentation
- Enable swagger UI (https://github.com/go-swagger/go-swagger)?
- Logo
- Other (badges) https://github.com/prometheus/prometheus

## Multi-platform ARM64

- GitHub Actions runners using ubuntu-latest do not have QEMU pre-installed by default
- docker/setup-qemu-action
- https://docs.docker.com/engine/storage/containerd/
- https://github.com/LeslieLeung/go-multiplatform-docker
- https://docs.docker.com/engine/storage/containerd/
- https://docs.docker.com/build/building/multi-platform/#cross-compiling-a-go-application
- https://github.com/docker/setup-buildx-action


Per https://docs.docker.com/build/builders/drivers/ & https://github.com/docker/setup-buildx-action
- may prefer 
  - docker buildx create --name ociregistry --driver docker-container --driver-opt default-load=true
  - docker buildx build --load <image> --builder=ociregistry

name: Multi-Platform Build

on:
  push:
    branches:
      - main

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx <------------------------ Sets up the docker-container driver
        uses: docker/setup-buildx-action@v3

      - name: Build and push Docker image
        uses: docker/build-push-action@v5
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: your-dockerhub-username/your-image-name:latest