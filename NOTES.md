CHANGES

1. api/ociregistry.yaml from root

consider a new empty project - start from scratch... 

1. bring over handler
2. bring over puller - handle current use cases - namespace in path, in url, tag+digest, digest only
3. 


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

