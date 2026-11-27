package subcmd

import (
	"fmt"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/preload"
)

// Load uses the preload.Load function to load the cache on disk from image tarballs and image list
// files. See that function for more details. Return values:
//
//	0, nil - no error - all images loaded to disk
//	1, err - fatal error, e.g.: unreachable path, bad glob
//	3, err - partial success: some files loaded, some didn't
func Load() (int, error) {
	failedCount, err := preload.Load(config.GetImageFile(), config.GetResolveRef())
	if err != nil {
		return 1, err
	} else if failedCount > 0 {
		return 3, fmt.Errorf("%d file(s) failed to load (see the log)", failedCount)
	}
	return 0, nil
}
