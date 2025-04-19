package cache

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

const srch = `.*sha256:([a-f0-9]{64}).*`

var re = regexp.MustCompile(srch)

// Type ManifestComparer applies a function to the passed manifest holder
// and returns true if the manifest matches the function selection logic
// else false.
type ManifestComparer func(imgpull.ManifestHolder) bool

// Type MHReader is a reader over a list of manifests
type MHReader struct {
	mh        []imgpull.ManifestHolder
	withBlobs bool
	idx       int
}

// Type BlobReader is a reader over a list of blobs
type BlobReader struct {
	blobs []blobResult
	idx   int
}

// type blobResult is a blob and its ref counts
type blobResult struct {
	digest string
	refCnt int
}

// NewMFReader returns a reader over a list of manifests. The reader will only
// show manifests
func NewMFReader(mh []imgpull.ManifestHolder) *MHReader {
	sort.Slice(mh, func(i, j int) bool {
		return mh[i].ImageUrl < mh[j].ImageUrl
	})
	return &MHReader{
		mh:  mh,
		idx: 0,
	}
}

// NewMFReaderWithBlobs returns a reader over a list of manifests. The reader
// shows manifests and the blobs in the manifest
func NewMFReaderWithBlobs(mh []imgpull.ManifestHolder) *MHReader {
	sort.Slice(mh, func(i, j int) bool {
		return mh[i].ImageUrl < mh[j].ImageUrl
	})
	return &MHReader{
		mh:        mh,
		withBlobs: true,
		idx:       0,
	}
}

// Read reads from the manifest reader in the receiver.
func (mhr *MHReader) Read(b []byte) (n int, err error) {
	if mhr.idx >= len(mhr.mh) {
		return 0, io.EOF
	}
	mh := mhr.mh[mhr.idx]
	var digests []string
	if mhr.withBlobs {
		bc.RLock()
		defer bc.RUnlock()
		for _, layer := range mh.Layers() {
			digests = append(digests, layer.Digest)
		}
	}

	hdr := ""
	if mhr.idx == 0 {
		hdr = "URL DIGEST\n"
	}
	url := mh.ImageUrl
	dgst := re.FindStringSubmatch(url)
	if len(dgst) == 2 {
		url = strings.Replace(url, dgst[1], dgst[1][:10], 1)
	}
	line := fmt.Sprintf("%s%s %s\n", hdr, url, mh.Digest)
	if len(digests) > 0 {
		for _, digest := range digests {
			line += fmt.Sprintf("- %s\n", digest)
		}
	}
	n = copy(b, line)
	mhr.idx++
	return n, nil
}

// GetManifestsCompare traverses the in-mem manifest cache and evaluates each manifest
// according to the passed comparer. Manifests selected by the comparer are returned to
// the caller in an array. The count arg is the max number of manifests to include. If
// noLimit (-1) then there is no limit. The purpose of the limit is to lock the manifest
// cache in small pieces. Querying more frequently with smaller chunks should result in
// better concurrency since this function locks the entire cache while it runs.
func GetManifestsCompare(comparer ManifestComparer, count int) []imgpull.ManifestHolder {
	mc.Lock()
	defer mc.Unlock()
	mhs := make([]imgpull.ManifestHolder, 0, len(mc.manifests))
	matches := 0
	for url, mh := range mc.manifests {
		if url != mh.ImageUrl {
			// when manifests are added to the in-mem cache, if the manifest has a tag
			// it is added by tag and again by digest so it is retrievable both ways. When
			// added by digest (the 2nd one), the map key will have the digest in the url but
			// the manifest produced by the map will still have the tag in its url. (It's a copy.)
			// The second copy is only for lookup and is not considered to be a separate manifest.
			// The prune will function handle the second copy so only add to the prune list if the
			// lookup key matches the manifest url - this *guarantees* that the manifest to prune
			// is not a 2nd copy.
			continue
		}
		if comparer(mh) {
			matches++
			if count != noLimit && matches > count {
				break
			}
			mhs = append(mhs, mh)
		}
	}
	return mhs
}

// Read reads from the blob reader in the receiver
func (br *BlobReader) Read(b []byte) (n int, err error) {
	if br.idx >= len(br.blobs) {
		return 0, io.EOF
	}
	blob := br.blobs[br.idx]

	hdr := ""
	if br.idx == 0 {
		hdr = "URL REFCNT\n"
	}
	line := fmt.Sprintf("%s%s %d\n", hdr, blob.digest, blob.refCnt)
	n = copy(b, line)
	br.idx++
	return n, nil
}

// NewBlobReader returns a reader over an array of blobs and ref counts
func NewBlobReader(blobs []blobResult) *BlobReader {
	sort.Slice(blobs, func(i, j int) bool {
		return blobs[i].digest < blobs[j].digest
	})
	return &BlobReader{
		blobs: blobs,
		idx:   0,
	}
}

// GetBlobsSubstr gets a list of blobs whose digests contain the passed
// substring like "4c9126d4"
func GetBlobsSubstr(substr string, count int) []blobResult {
	bc.RLock()
	defer bc.RUnlock()
	blobs := make([]blobResult, 0, len(bc.blobs))
	found := 0
	for digest, refCnt := range bc.blobs {
		if substr == "" || strings.Contains(digest, substr) {
			found++
			if found > count {
				break
			}
			blobs = append(blobs, blobResult{digest, refCnt})
		}
	}
	return blobs
}
