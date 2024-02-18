package preload

import (
	"bufio"
	"fmt"
	"ociregistry/impl/pullrequest"
	"ociregistry/impl/serialize"
	"ociregistry/impl/upstream"
	"os"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

var dlmCln = regexp.MustCompile("[/:]+")
var dlmAt = regexp.MustCompile("[/@]+")

func Preload(imageListFile, imagePath, platformArch, platformOs string) error {
	start := time.Now()
	log.Infof("loading images from file: %s", imageListFile)
	itemcnt := 0
	f, err := os.Open(imageListFile)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := string(scanner.Bytes())
		if strings.HasPrefix(line, "#") {
			continue
		}
		var parts []string
		if strings.Contains(line, "@") {
			parts = dlmAt.Split(line, -1)
		} else {
			parts = dlmCln.Split(line, -1)
		}
		var org, image, ref, remote string
		if len(parts) == 4 {
			org = parts[1]
			image = parts[2]
			ref = parts[3]
		} else if len(parts) == 3 {
			image = parts[1]
			ref = parts[2]
		} else {
			return fmt.Errorf("unable to parse image ref: %s", line)
		}
		remote = parts[0]

		// manifest list
		pr := pullrequest.NewPullRequest(org, image, ref, remote)
		log.Infof("get from remote: %s", pr.Url())
		mh, err := upstream.Get(pr, imagePath, 60000)
		if err != nil {
			return err
		}

		if serialize.IsOnFilesystem(mh.Digest, false, imagePath) {
			log.Infof("image manifest already cached for: %s", pr.Url())
		} else {
			serialize.ToFilesystem(mh, imagePath)
			itemcnt++
		}
		digest, err := getImageManifestDigest(mh, platformArch, platformOs)
		if err != nil {
			return err
		}

		// image manifest
		pr = pullrequest.NewPullRequest(org, image, digest, remote)
		if serialize.IsOnFilesystem(digest, true, imagePath) {
			log.Infof("image manifest already cached for: %s", pr.Url())
			continue
		}
		log.Infof("get from remote: %s", pr.Url())
		_, err = upstream.Get(pr, imagePath, 60000)
		if err != nil {
			return err
		}
		serialize.ToFilesystem(mh, imagePath)
		itemcnt++
	}
	log.Infof("loaded %d images to the file system cache in %s", itemcnt, time.Since(start))
	return nil
}

func getImageManifestDigest(mh upstream.ManifestHolder, platformArch, platformOs string) (string, error) {
	if mh.Type == upstream.V2dockerManifestList {
		for _, m := range mh.V2dockerManifestList.Manifests {
			if m.Platform.Architecture == platformArch && m.Platform.OS == platformOs {
				return m.Digest, nil
			}
		}
	} else if mh.Type == upstream.V1ociIndex {
		for _, m := range mh.V1ociIndex.Manifests {
			if m.Platform.Architecture == platformArch && m.Platform.Os == platformOs {
				return m.Digest, nil
			}
		}
	} else {
		return "", fmt.Errorf("unexpected manifest type for url %s", mh.ImageUrl)
	}
	return "", fmt.Errorf("no manifest found for url %s matching arch=%s and os=%s", mh.ImageUrl, platformArch, platformOs)
}