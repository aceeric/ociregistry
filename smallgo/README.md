# Small Go Program and OCI Image

This directory creates a small Golang image with a three layers for testing the server. The first layer runs a simple main function that prints to the console forever - so it's useful for testing in Kubernetes. The other layers just provide small layers for testing.

## Build

```
cd smallgo && go build -o main main.go
```

## Test

```
./main
```

## Result

```
running main
running main
running main
running main
running main
(etc...)
```

## Create an OCI Image

```
docker build -t localhost:5000/appzygy/smallgo:v1.0.0 -f Dockerfile .
```

## Check

```
docker images
```

## Result

```
REPOSITORY                       TAG       IMAGE ID       CREATED          SIZE
localhost:5000/appzygy/smallgo   v1.0.0    c07281f652ba   23 minutes ago   1.72MB
```

## Files

```
ls -l
```

## Result

```
total 1700
-rw-rw-r-- 1 eace eace     101 Jan 12 15:27 Dockerfile
-rw-rw-r-- 1 eace eace       6 Jan 12 15:25 foobar
-rw-rw-r-- 1 eace eace       7 Jan 12 15:25 frobozz
-rwxrwxr-x 1 eace eace 1720169 Jan 13 19:52 main
-rw-rw-r-- 1 eace eace     123 Jan 13 19:52 main.go
-rw-rw-r-- 1 eace eace    4019 Jan 13 19:52 README.md
```

## Inspect

```
docker inspect c07281f652ba
```

## Result

```
[
    {
        "Id": "sha256:c07281f652bace2b746f8241a5013b6034d32d6b001967295c7e99b31a49b01e",
        "RepoTags": [
            "localhost:5000/appzygy/smallgo:v1.0.0"
        ],
        "RepoDigests": [],
        "Parent": "",
        "Comment": "buildkit.dockerfile.v0",
        "Created": "2024-01-13T19:53:35.627335648-05:00",
        "Container": "",
        "ContainerConfig": {
            "Hostname": "",
            "Domainname": "",
            "User": "",
            "AttachStdin": false,
            "AttachStdout": false,
            "AttachStderr": false,
            "Tty": false,
            "OpenStdin": false,
            "StdinOnce": false,
            "Env": null,
            "Cmd": null,
            "Image": "",
            "Volumes": null,
            "WorkingDir": "",
            "Entrypoint": null,
            "OnBuild": null,
            "Labels": null
        },
        "DockerVersion": "",
        "Author": "",
        "Config": {
            "Hostname": "",
            "Domainname": "",
            "User": "",
            "AttachStdin": false,
            "AttachStdout": false,
            "AttachStderr": false,
            "Tty": false,
            "OpenStdin": false,
            "StdinOnce": false,
            "Env": [
                "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
            ],
            "Cmd": null,
            "Image": "",
            "Volumes": null,
            "WorkingDir": "/",
            "Entrypoint": [
                "/main"
            ],
            "OnBuild": null,
            "Labels": null
        },
        "Architecture": "amd64",
        "Os": "linux",
        "Size": 1720182,
        "VirtualSize": 1720182,
        "GraphDriver": {
            "Data": {
                "LowerDir": "/var/lib/docker/overlay2/s3937i6pys1fpybjnzt6muijy/diff:/var/lib/docker/overlay2/ns47caoq14b40haf8yi9sxhw6/diff",
                "MergedDir": "/var/lib/docker/overlay2/olq93xnm1fb41xrynf1iolmph/merged",
                "UpperDir": "/var/lib/docker/overlay2/olq93xnm1fb41xrynf1iolmph/diff",
                "WorkDir": "/var/lib/docker/overlay2/olq93xnm1fb41xrynf1iolmph/work"
            },
            "Name": "overlay2"
        },
        "RootFS": {
            "Type": "layers",
            "Layers": [
                "sha256:a95a9c41f2c8e70fd9bbe64799e09fb24e8dc3beb37d121e6c8237e2d82c436a",
                "sha256:6e8b152e65c6f1bc9a0e575416a688b7714e15a53879100c97b4e248c28de09a",
                "sha256:7689c74f78297c5d751c7bfb7722819745df1b150c3e3c10234c00266ce03601"
            ]
        },
        "Metadata": {
            "LastTagTime": "2024-01-13T20:16:33.830618184-05:00"
        }
    }
]
```
