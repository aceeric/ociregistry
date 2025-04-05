CHANGES

1. api/ociregistry.yaml from root
2. impl/ociregistry.go - should return struct or interface? If interface then impl/handlers.go
   SHOULD NOT take the struct in the receiver because its extendin the interface - need HandlerStructFromOciRegistry...

TODO
1. Populating the cache from file system has to populate blob cache
2. Resolve all TODO
3. if "always pull latest" then replace last latest if different - handle decrement old / increment new blobs
4. support configfor to cache tls creds to avoid the overhead?
5. --hello-world
6. Support TLS??
7. Replace yaml
8. imgpull blob concurrency init
9. Rename `--image-path` `--cache-path` and associated variables

1. prune by create
2. prune by last access
3. when loading the cache, compare digest to object?
4. Rework concurrent load?


imageref in imgpull - may need to move out of internal and use here


.
├── api                 no change
│   └── models          "
├── bin                 "
├── cmd                 ?
├── experiment          delete
├── hack                no change
├── impl
│   ├── extractor       delete replace by imgull
│   ├── globals         no change - mostly logging
│   ├── helpers         ?
│   ├── memcache        replace by parts of experimental
│   ├── preload         ?
│   ├── pullrequest     delete replace by imgull
│   ├── serialize       interface with new memcache
│   └── upstream        biggest impact: config / manifestholder / queue / types
│       ├── v1oci       delete replace by imgull
│       └── v2docker    "
├── mock                no change
│   └── testfiles       no change
├── resources           no change
└── systemd-service     no change


new structure

impl
  keep handlers and ociregistry as is
impl/memcache - new experimental functionality
impl/

