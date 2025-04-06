package cache

//// think about doing this in on transaction?
//func prune(key string) {
//	// get blobs associated with manifest
//	digests := simulateDigestList(key)
//	delManifestFromCache(key)
//	delBlobsFromCache(digests)
//}

//// todo from file system
//// this could delete a manifest right after it was pulled and waiters
//// are waiting which could cause the waiters to return
//func delManifestFromCache(key string) {
//	mc.Lock()
//	defer mc.Unlock()
//	delete(mc.manifests, key)
//}

//// todo lazy delete from file system??
//func delBlobsFromCache(digests []string) {
//	bc.Lock()
//	defer bc.Unlock()
//	for _, digest := range digests {
//		if cnt := bc.blobs[digest]; cnt > 0 {
//			bc.blobs[digest] = cnt - 1
//		}
//	}
//}
