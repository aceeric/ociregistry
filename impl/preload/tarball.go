package preload

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/pullrequest"
	"github.com/aceeric/ociregistry/impl/serialize"

	"github.com/aceeric/imgpull/pkg/imgpull"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	log "github.com/sirupsen/logrus"
)

// RefResolver lets the caller supply a fully-qualified pull ref (e.g.
// "registry.k8s.io/foo:v1") for an image found in a tarball. When supplied,
// it is consulted for every image and has final say over the ref used -
// not merely a fallback for candidates that fail to parse. This matters
// because a tarball-supplied tag can be syntactically valid yet
// semantically wrong (e.g. crane's digest-pull placeholder tags - see
// resolveOrDefault below); a resolver needs the chance to override those
// too, not just outright-missing/unparseable ones. candidate is whatever
// ref the tarball itself offered, if any (may be empty). digest is the
// manifest digest if known at the point of the call, else empty (it isn't
// known yet for a docker-legacy tag until after the image is materialized).
// Returning "" means "skip this image". A resolver that wants ordinary tags
// left alone should return candidate unchanged for anything it doesn't
// specifically want to rewrite.
type RefResolver func(candidate string, digest string) string

// LoadTarball loads every image found in the OCI-layout or docker-save
// tarball at tarPath into the file system cache, the same way Load does for
// a text file of image URLs. All tarball format comprehension (docker-save
// vs OCI layout, tag matching, gzip vs plain layers, config blob handling,
// multi-image manifests/indexes) is delegated to go-containerregistry; this
// file's only responsibility is bridging its v1.Image/v1.ImageIndex output
// into this project's existing imgpull.ManifestHolder + serialize/
// pullrequest pipeline, at exactly one point: buildManifestHolder below.
// resolveRef is consulted for any image whose ref needs resolving or
// overriding; pass nil to only accept images that already carry a usable,
// fully-qualified ref of their own. resolveRef is only accepted for
// tarballs containing exactly one image - see guardResolver - since a
// single resolver has no reliable way to disambiguate across several.
func LoadTarball(tarPath string, resolveRef RefResolver) error {
	imagePath := config.GetImagePath()
	platformOs := config.GetOs()
	platformArch := config.GetArch()

	start := time.Now()
	log.Infof("loading images from tarball: %s", tarPath)

	plainTarPath, cleanup, err := ensurePlainTar(tarPath)
	if err != nil {
		return fmt.Errorf("error reading tarball %q: %w", tarPath, err)
	}
	defer cleanup()

	isOci, err := tarHasEntry(plainTarPath, "oci-layout")
	if err != nil {
		return fmt.Errorf("error reading tarball %q: %w", tarPath, err)
	}

	var itemcnt int
	if isOci {
		itemcnt, err = loadOciLayoutTarball(plainTarPath, resolveRef, imagePath, platformOs, platformArch)
	} else {
		itemcnt, err = loadDockerTarball(plainTarPath, resolveRef, imagePath)
	}
	if err != nil {
		return err
	}

	log.Infof("loaded %d images from tarball %q to the file system cache in %s", itemcnt, tarPath, time.Since(start))
	return nil
}

// --- docker save format ---

// loadDockerTarball uses go-containerregistry's tarball package, which
// understands manifest.json's array-of-images shape, tag matching, and the
// translation from docker's proprietary config/layer file references into a
// real distribution-shaped manifest - all without us touching tar internals
// or synthesizing anything by hand.
func loadDockerTarball(tarPath string, resolveRef RefResolver, imagePath string) (int, error) {
	manifests, err := tarball.LoadManifest(pathOpener(tarPath))
	if err != nil {
		return 0, fmt.Errorf("error reading docker save manifest: %w", err)
	}
	if err := guardResolver(len(manifests), resolveRef); err != nil {
		return 0, err
	}

	itemcnt := 0
	for _, m := range manifests {
		if len(m.RepoTags) == 0 {
			log.Infof("skipping untagged image in %s (config %s): no RepoTag to select it by", tarPath, m.Config)
			continue
		}
		for _, tagStr := range m.RepoTags {
			ref := resolveOrDefault(tagStr, "", resolveRef)
			if ref == "" {
				log.Infof("skipping image %q: no usable ref", tagStr)
				continue
			}
			// ASSUMPTION TO VERIFY: tarball.ImageFromPath takes a *name.Tag and
			// selects the manifest.json entry whose RepoTags contains it.
			// WeakValidation is used since docker-produced tags don't always
			// satisfy go-containerregistry's stricter default parsing.
			tag, err := name.NewTag(tagStr, name.WeakValidation)
			if err != nil {
				log.Errorf("error parsing tag %q from %s: %s", tagStr, tarPath, err)
				continue
			}
			img, err := tarball.ImageFromPath(tarPath, &tag)
			if err != nil {
				log.Errorf("error reading image %q from %s: %s", tagStr, tarPath, err)
				continue
			}
			cnt, err := writeImageManifest(img, ref, imagePath)
			if err != nil {
				log.Errorf("error loading image %q from tarball: %s", ref, err)
				return itemcnt, err
			}
			itemcnt += cnt
		}
	}
	return itemcnt, nil
}

// pathOpener adapts a plain file path to tarball.Opener.
func pathOpener(tarPath string) tarball.Opener {
	return func() (io.ReadCloser, error) {
		return os.Open(tarPath)
	}
}

// ensurePlainTar returns a path to an uncompressed tar file for tarPath,
// transparently decompressing to a temp file first if tarPath is gzip-
// compressed (.tar.gz/.tgz - detected by magic bytes, not by file
// extension, so this doesn't depend on the caller naming the file
// correctly). Returns the path to use downstream (identical to tarPath if
// no decompression was needed), a cleanup func the caller should defer, and
// an error.
//
// This exists as a single up-front normalization step rather than gzip-
// detection scattered across each consumer, because go-containerregistry's
// own APIs aren't consistent about how they accept input: tarball.LoadManifest
// takes an Opener (which could be made gzip-transparent on its own), but
// tarball.ImageFromPath takes a bare file path with no hook to intercept -
// so an Opener-level fix wouldn't cover it. Guaranteeing every downstream
// consumer (tarHasEntry, loadDockerTarball, loadOciLayoutTarball's own
// extractTar) sees a real, uncompressed tar file on disk sidesteps that
// inconsistency entirely instead of working around it three separate times.
func ensurePlainTar(tarPath string) (string, func(), error) {
	noop := func() {}

	f, err := os.Open(tarPath)
	if err != nil {
		return "", noop, err
	}
	defer f.Close()

	magic := make([]byte, 2)
	n, _ := io.ReadFull(f, magic)
	if n != 2 || magic[0] != 0x1f || magic[1] != 0x8b {
		return tarPath, noop, nil // already a plain tar
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", noop, err
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", noop, fmt.Errorf("error opening %q as gzip: %w", tarPath, err)
	}
	defer gz.Close()

	out, err := os.CreateTemp("", "ociregistry-preload-*.tar")
	if err != nil {
		return "", noop, err
	}
	if _, err := io.Copy(out, gz); err != nil {
		out.Close()
		os.Remove(out.Name())
		return "", noop, err
	}
	if err := out.Close(); err != nil {
		os.Remove(out.Name())
		return "", noop, err
	}
	return out.Name(), func() { os.Remove(out.Name()) }, nil
}

// --- OCI layout format (ctr image export, buildkit --output type=oci, nerdctl save) ---

// loadOciLayoutTarball extracts the tar (layout.ImageIndexFromPath needs a
// real directory, not an archive) and hands it to go-containerregistry's
// layout package, which understands index.json and content-addressed
// blobs/sha256/* directly.
// refFromAnnotations extracts the best available ref from an OCI-layout
// manifest descriptor's annotations.
//
// Real producers disagree about what goes in the OCI spec's own
// "org.opencontainers.image.ref.name" annotation. Per spec it's just a
// "reference name" - historically meant for local lookup via an OCI
// layout's refs/ directory, not necessarily a fully-qualified pull ref -
// and Docker's containerd-backed `docker save` populates it with exactly
// that: a bare tag like "3.10.2", which pullrequest.NewPullRequestFromUrl
// will reject outright (no registry host). Docker adds its own
// "io.containerd.image.name" annotation alongside it, carrying the real
// fully-qualified ref (e.g. "registry.k8s.io/pause:3.10.2") - so that's
// preferred when present. containerd's own `ctr image export`, by
// contrast, conventionally does put the full ref directly in the spec
// annotation (no io.containerd.image.name at all), so that remains the
// fallback rather than being dropped.
func refFromAnnotations(annotations map[string]string) string {
	if v := annotations["io.containerd.image.name"]; v != "" {
		return v
	}
	return annotations["org.opencontainers.image.ref.name"]
}

func loadOciLayoutTarball(tarPath string, resolveRef RefResolver, imagePath string, platformOs string, platformArch string) (int, error) {
	dir, err := extractTar(tarPath)
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(dir)

	idx, err := layout.ImageIndexFromPath(dir)
	if err != nil {
		return 0, fmt.Errorf("error reading OCI layout: %w", err)
	}
	im, err := idx.IndexManifest()
	if err != nil {
		return 0, err
	}
	if err := guardResolver(len(im.Manifests), resolveRef); err != nil {
		return 0, err
	}

	itemcnt := 0
	for _, desc := range im.Manifests {
		ref := resolveOrDefault(refFromAnnotations(desc.Annotations), desc.Digest.String(), resolveRef)
		if ref == "" {
			log.Infof("skipping image (digest %s): no usable ref", desc.Digest)
			continue
		}
		if desc.MediaType.IsIndex() {
			// A nested index is containerd's multi-platform fan-out for one
			// logical image (e.g. `ctr image export --all-platforms`). Cache the
			// index itself (mirrors doPull's existing manifest-list handling in
			// preload.go), then select and cache the single platform this server
			// is configured for.
			childIdx, err := idx.ImageIndex(desc.Digest)
			if err != nil {
				log.Errorf("error reading nested index %s: %s", desc.Digest, err)
				continue
			}
			cnt, err := writeIndexAndSelectedImage(childIdx, ref, imagePath, platformOs, platformArch)
			if err != nil {
				log.Errorf("error loading multi-platform image %q from tarball: %s", ref, err)
				return itemcnt, err
			}
			itemcnt += cnt
			continue
		}
		img, err := idx.Image(desc.Digest)
		if err != nil {
			log.Errorf("error reading image %s: %s", desc.Digest, err)
			continue
		}
		cnt, err := writeImageManifest(img, ref, imagePath)
		if err != nil {
			log.Errorf("error loading image %q from tarball: %s", ref, err)
			return itemcnt, err
		}
		itemcnt += cnt
	}
	return itemcnt, nil
}

// writeIndexAndSelectedImage caches the multi-platform index itself, then
// the one child image matching platformOs/platformArch - the same two-step
// "cache the list, then cache the selected image" doPull already does for
// a manifest-list pull from a live upstream.
func writeIndexAndSelectedImage(idx v1.ImageIndex, ref string, imagePath string, platformOs string, platformArch string) (int, error) {
	itemcnt, err := writeIndexManifest(idx, ref, imagePath)
	if err != nil {
		return itemcnt, err
	}
	im, err := idx.IndexManifest()
	if err != nil {
		return itemcnt, err
	}
	for _, d := range im.Manifests {
		if d.Platform != nil && d.Platform.OS == platformOs && d.Platform.Architecture == platformArch {
			img, err := idx.Image(d.Digest)
			if err != nil {
				return itemcnt, err
			}
			cnt, err := writeImageManifest(img, ref, imagePath)
			if err != nil {
				return itemcnt, err
			}
			return itemcnt + cnt, nil
		}
	}
	return itemcnt, fmt.Errorf("no manifest for os=%s arch=%s in multi-platform image %q", platformOs, platformArch, ref)
}

// extractTar unpacks tarPath into a fresh temp directory, which the caller
// is responsible for removing. Includes a zip-slip guard since tar entry
// names are attacker-controllable input if the tarball's provenance isn't
// trusted.
func extractTar(tarPath string) (string, error) {
	dir, err := os.MkdirTemp("", "ociregistry-preload-*")
	if err != nil {
		return "", err
	}
	f, err := os.Open(tarPath)
	if err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	defer f.Close()

	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			os.RemoveAll(dir)
			return "", err
		}
		target := filepath.Join(dir, filepath.Clean(hdr.Name))
		if !strings.HasPrefix(target, filepath.Clean(dir)+string(os.PathSeparator)) {
			os.RemoveAll(dir)
			return "", fmt.Errorf("tar entry %q escapes extraction directory", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				os.RemoveAll(dir)
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				os.RemoveAll(dir)
				return "", err
			}
			out, err := os.Create(target)
			if err != nil {
				os.RemoveAll(dir)
				return "", err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				os.RemoveAll(dir)
				return "", err
			}
			out.Close()
		}
	}
	return dir, nil
}

// tarHasEntry does a cheap header-only scan for a top-level file named
// name, used just to decide docker-save vs OCI-layout - not a full parse.
func tarHasEntry(tarPath string, name string) (bool, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return false, err
	}
	defer f.Close()
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if filepath.Clean(hdr.Name) == name {
			return true, nil
		}
	}
}

// --- the one integration point with imgpull's data structures ---

// manifestLike is satisfied structurally by both v1.Image and v1.ImageIndex
// (both expose exactly these three methods), letting writeImageManifest and
// writeIndexManifest share the same conversion-to-ManifestHolder logic
// below.
type manifestLike interface {
	RawManifest() ([]byte, error)
	MediaType() (types.MediaType, error)
	Digest() (v1.Hash, error)
}

// buildManifestHolder is the single place this file constructs an
// imgpull.ManifestHolder. It always goes through imgpull's own public
// NewManifestHolder constructor (manifest_holder.go) rather than touching
// any of its unexported fields, so this stays correct even if imgpull's
// internal representation changes later. Returns (holder, alreadyCached, err).
//
// digest.Hex (bare hex, no "sha256:" prefix) is used rather than
// digest.String() - imgpull's own network-pull path always strips the
// prefix before setting ManifestHolder.Digest (see util.DigestFrom in
// internal/methods/methods.go), and serialize.MhToFilesystem writes the
// manifest file at exactly filepath.Join(imagePath, subDir, mh.Digest) with
// no stripping of its own - it trusts the caller to already be bare hex.
// Passing the prefixed form here would silently write files named
// "sha256:<hex>" instead of "<hex>", inconsistent with every manifest
// written by a real network pull.
func buildManifestHolder(ml manifestLike, ref string, imagePath string) (imgpull.ManifestHolder, bool, error) {
	pr, err := pullrequest.NewPullRequestFromUrl(ref)
	if err != nil {
		return imgpull.ManifestHolder{}, false, fmt.Errorf("unable to parse image ref %q: %w", ref, err)
	}
	digest, err := ml.Digest()
	if err != nil {
		return imgpull.ManifestHolder{}, false, err
	}
	if _, found := serialize.MhFromFilesystem(digest.Hex, pr.IsLatest(), imagePath); found {
		return imgpull.ManifestHolder{}, true, nil
	}
	raw, err := ml.RawManifest()
	if err != nil {
		return imgpull.ManifestHolder{}, false, err
	}
	mediaType, err := ml.MediaType()
	if err != nil {
		return imgpull.ManifestHolder{}, false, err
	}
	mh, err := imgpull.NewManifestHolder(string(mediaType), raw, digest.Hex, ref)
	return mh, false, err
}

// writeIndexManifest caches a manifest-list/index's own bytes. No blobs to
// write - an index isn't an image manifest, mh.IsImageManifest() will be
// false for it, matching mh.Layers() returning nothing for this Type.
func writeIndexManifest(idx v1.ImageIndex, ref string, imagePath string) (int, error) {
	mh, cached, err := buildManifestHolder(idx, ref, imagePath)
	if err != nil || cached {
		return 0, err
	}
	if err := serialize.MhToFilesystem(mh, imagePath, false); err != nil {
		return 0, err
	}
	return 1, nil
}

// writeImageManifest caches a single image's manifest and its blobs.
func writeImageManifest(img v1.Image, ref string, imagePath string) (int, error) {
	mh, cached, err := buildManifestHolder(img, ref, imagePath)
	if err != nil {
		return 0, err
	}
	if cached {
		log.Infof("already cached: %s", ref)
		return 0, nil
	}
	if err := writeBlobs(img, imagePath); err != nil {
		return 0, err
	}
	if err := serialize.MhToFilesystem(mh, imagePath, false); err != nil {
		return 0, err
	}
	log.Infof("loaded %s", ref)
	return 1, nil
}

// writeBlobs copies every layer plus the config blob into imagePath/blobs,
// skipping any already present. Compressed() is used for layers so the
// bytes on disk match whatever digest the manifest's layer descriptor
// actually declares (go-containerregistry's Layer.Digest() always
// corresponds to the compressed representation for the media types both
// docker-save and OCI-layout tarballs use).
func writeBlobs(img v1.Image, imagePath string) error {
	blobDir := filepath.Join(imagePath, globals.BlobPath)

	layers, err := img.Layers()
	if err != nil {
		return err
	}
	for _, layer := range layers {
		digest, err := layer.Digest()
		if err != nil {
			return err
		}
		if exists, _ := serialize.BlobExists(imagePath, digest.Hex); exists {
			continue
		}
		rc, err := layer.Compressed()
		if err != nil {
			return err
		}
		if err := writeBlob(blobDir, digest.Hex, rc); err != nil {
			return err
		}
	}

	configDigest, err := img.ConfigName()
	if err != nil {
		return err
	}
	if exists, _ := serialize.BlobExists(imagePath, configDigest.Hex); !exists {
		raw, err := img.RawConfigFile()
		if err != nil {
			return err
		}
		if err := writeBlob(blobDir, configDigest.Hex, io.NopCloser(bytes.NewReader(raw))); err != nil {
			return err
		}
	}
	return nil
}

func writeBlob(blobDir string, digestHex string, rc io.ReadCloser) error {
	defer rc.Close()
	f, err := os.Create(filepath.Join(blobDir, digestHex))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, rc)
	return err
}

// resolveOrDefault accepts candidate as-is if it already parses as a
// fully-qualified pull ref, otherwise defers to resolveRef (if provided).
// resolveOrDefault gives resolveRef the final word over the ref used for an
// image, whenever one is supplied - it is not merely a fallback for
// candidates that fail to parse. This matters because a tag can be
// syntactically valid (parses fine via pullrequest.NewPullRequestFromUrl)
// while still being semantically wrong. For example, `crane pull --format
// tarball <ref>@sha256:...` (i.e. pulling by digest) writes a RepoTags
// entry like "index.docker.io/grafana/grafana:i-was-a-digest" - a real,
// parseable, but useless placeholder tag standing in for "no tag". If this
// only consulted resolveRef for candidates that fail to parse, a caller
// would have no way to catch and rewrite that case; the placeholder would
// sail through untouched and get cached under a meaningless tag. So: when
// resolveRef is provided, it is always consulted and decides the outcome,
// including declining an otherwise-valid-looking candidate by returning "".
// Only when no resolver is supplied does this fall back to accepting an
// already-valid candidate as-is.
func resolveOrDefault(candidate string, digest string, resolveRef RefResolver) string {
	if resolveRef != nil {
		return resolveRef(candidate, digest)
	}
	if candidate != "" {
		if _, err := pullrequest.NewPullRequestFromUrl(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// guardResolver rejects the combination of a caller-supplied RefResolver
// with a tarball containing more than one image. RefResolver is called
// once per image (per RepoTag, in the docker-save case) with just that
// image's candidate ref and digest - it has no visibility into how many
// other images are in the same tarball or what they are. A resolver
// written with one specific image in mind (e.g. "rewrite the crane
// digest-pull placeholder tag to the real version") could silently
// mis-resolve, over-accept, or produce collisions across images it was
// never designed to see. There's no clean way to fix that in general, so
// it's enforced as a hard precondition instead of left to caller
// discipline: pass a RefResolver only for tarballs known to hold exactly
// one image.
func guardResolver(imageCount int, resolveRef RefResolver) error {
	if resolveRef != nil && imageCount != 1 {
		return fmt.Errorf("a RefResolver was supplied but the tarball contains %d images; RefResolver is only supported for single-image tarballs", imageCount)
	}
	return nil
}
