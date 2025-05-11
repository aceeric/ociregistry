package globals

const (
	// LtsPath is the subdirectory under the image cache root where "latest" manifests
	// are stored. Latest manifests exist side-by-side with non-latest.
	LtsPath = "lts"
	// ImgPath is the subdirectory under the image cache root where non-"latest" manifests
	// are stored
	ImgPath = "img"
	// BlobPath is the subdirectory under the image cache root where blobs are stored
	BlobPath = "blobs"
)
