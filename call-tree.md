# Image-related Call Tree

NEXT:

GET RID OF:
func replaceInCache(pr pullrequest.PullRequest, mhNew imgpull.ManifestHolder, imagePath string) error {
	return addOrReplaceInCache(pr, mhNew, imagePath, true)
}

REVIEW/REWRITE ALL SOURCE DOCS IN WATCH.GO


| Operation                     | Package.Function   | cache live? | Source                   | Target                    |
|-------------------------------|--------------------|-------------|--------------------------|---------------------------|
| ociregistry load sub-command  | preload.Load       | No          | listfiles & tarballs     | filesystem cache          |
| ociregistry serve sub-command | cache.Load         | No          | image cache (filesystem) | in-mem cache              |
| ociregistry serve --preload   | preload.Preload    | No          | listfiles & tarballs     | filesystem cache          |
| ociregistry --watch-frequency | preload.RunWatcher | Yes         | listfiles & tarballs     | in-mem cache & filesystem |
| GET /v2/manifests/.../ref     | cache.GetManifest  | Yes         | REST API                 | in-mem cache & filesystem |


```text
subcmd.Load (load sub-command)
└── preload.Load
    └── preload.load - nil ledger

subcmd.Serve
├── cache.Load
│   └── serialize.WalkTheCache
│       ├── serialize.MhToFilesystem (date conversion)
│       └── cache.addToCache
│           └── cache.addOrReplaceInCache
│               └── cache.putInCache
│                   ├── cache.addBlobsToCache
│                   └── cache.set
│
├── preload.Preload (serve --preload)
│   └── preload.load - non-nil ledger
│       ├── preload.ResolveFiles
│       └── preload.loadFile
│           ├── preload.LoadTarball
│           │   ├── imgpull.OpenImageTarBall
│           │   ├── preload.chkResolveRef
│           │   ├── imgpull.TarManifestReader
│           │   ├── pullrequest.NewPullRequestFromUrl
│           │   ├── serialize.MhFromFilesystem
│           │   ├── serialize.MhToFilesystem
│           │   └── itb.SaveBlobs
│           └── preload.LoadFromListFile
│               └── preload.doPull
│                   ├── puller.HeadManifest
│                   └── preload.getFromCacheOrRemote
│                       ├── serialize.MhFromFilesystem
│                       ├── puller.GetManifestByDigest
│                       ├── puller.GetManifest
│                       ├── puller.PullBlobs
│                       └── serialize.MhToFilesystem
│    
└── preload.RunWatcher
    └── preload.oneWatchPass
        ├── preload.loadTarballW
        │   └── cache.LoadTarball
        │       ├── imgpull.OpenImageTarBall
        │       ├── itb.TarManifestReader
        │       ├── cache.loadFromTarball (watch.go:139)
        │       │   ├── cache.becomePuller
        │       │   ├── imgpull.SaveBlobs
        │       │   ├── cache.replaceInCache
        |       |   |   └── cache.addOrReplaceInCache
        │       │   |       └── cache.putInCache
        │       │   |           ├── cache.addBlobsToCache
        │       │   |           └── cache.set
        │       │   └── serialize.MhToFilesystem
        │       └── cache.trySweepBlobs
        └── preload.loadListFileW
            └── preload.loadImage
                ├── puller.HeadManifest
                ├── cache.CachedManifest
                └── cache.LoadFromUpstream (preload.loadListedImage is only caller)
                    ├── cache.becomePuller
                    ├── cache.pullFromUpstream
                    │   └── cache.replaceInCache
                    |       └── cache.addOrReplaceInCache
                    │           └── cache.putInCache
                    │               ├── cache.addBlobsToCache
                    │               └── cache.set
                    └── cache.trySweepBlobs

impl.handleV2ManifestsReference
└── cache.GetManifest
    ├── cache.getManifestOrEnqueue
    │   ├── cache.getManifestFromCache
    │   └── cache.enqueuePull
    ├── cache.pullToCache
    │   ├── cache.doPull
    │   │   ├── puller.GetManifest
    │   │   ├── puller.PullBlobs
    │   │   └── serialize.MhToFilesystem
    │   ├── cache.replaceInCache
    |   |   └── cache.addOrReplaceInCache
    │   │       └── cache.putInCache
    │   │           ├── cache.addBlobsToCache
    │   │           └── cache.set
    │   └── cache.addToCache
    |       └── cache.addOrReplaceInCache
    │           └── cache.putInCache
    │               ├── cache.addBlobsToCache
    │               └── cache.set
    ├── cache.getManifestFromCache
    └── cache.trySweepBlobs
```
