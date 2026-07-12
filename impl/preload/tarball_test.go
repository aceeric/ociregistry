package preload

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/impl/serialize"

	"github.com/aceeric/imgpull/pkg/imgpull"
)

// This file deliberately does NOT use imgpull's ToTar/TestTarNew fixture.
// ToTar writes a config "blob" that is literally the raw digest string (not
// JSON) and a "layer.tar.gz" whose content isn't actually gzip-compressed -
// fine for imgpull's own round-trip test of its own writer, but it would
// make go-containerregistry's parsing fail for reasons that have nothing to
// do with whether LoadTarball itself is correct. Building real, minimally
// valid tarballs here instead means a failure actually tells us something
// about tarball.go.

// digestHex returns the sha256 digest of b as a bare hex string (no
// "sha256:" prefix - that's the form used for tar entry paths in both
// formats tested here).
func digestHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// gzipLayer builds a tiny single-file tar (the "layer" contents) and returns
// both its raw (uncompressed) bytes and its gzip-compressed bytes. Real
// image layers are gzip-compressed tars of a filesystem diff; content
// doesn't matter for these tests, only that it's a structurally real gzip
// stream, since go-containerregistry will actually gunzip it if asked.
func gzipLayer(t *testing.T, fileContent string) (raw []byte, gz []byte) {
	t.Helper()
	var rawBuf bytes.Buffer
	tw := tar.NewWriter(&rawBuf)
	hdr := &tar.Header{
		Name: "hello.txt",
		Mode: 0644,
		Size: int64(len(fileContent)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("writing tar header: %s", err)
	}
	if _, err := tw.Write([]byte(fileContent)); err != nil {
		t.Fatalf("writing tar content: %s", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("closing tar writer: %s", err)
	}
	raw = rawBuf.Bytes()

	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	if _, err := gw.Write(raw); err != nil {
		t.Fatalf("writing gzip content: %s", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("closing gzip writer: %s", err)
	}
	return raw, gzBuf.Bytes()
}

// minimalImageConfig builds a just-barely-valid OCI/docker image config JSON.
// diffIDHex is the uncompressed layer digest - go-containerregistry cross-
// checks rootfs.diff_ids against the layer count, so this has to be right
// for a real image to result rather than an error.
func minimalImageConfig(diffIDHex string) []byte {
	cfg := map[string]any{
		"architecture": "amd64",
		"os":           "linux",
		"config":       map[string]any{},
		"rootfs": map[string]any{
			"type":     "layers",
			"diff_ids": []string{"sha256:" + diffIDHex},
		},
		"history": []map[string]any{
			{"created": time.Now().UTC().Format(time.RFC3339), "created_by": "tarball_test"},
		},
	}
	b, _ := json.Marshal(cfg)
	return b
}

// writeTarFile writes a set of {path: content} entries as a plain
// (uncompressed) tar file at tarPath.
func writeTarFile(t *testing.T, tarPath string, entries map[string][]byte) {
	t.Helper()
	f, err := os.Create(tarPath)
	if err != nil {
		t.Fatalf("creating tar file: %s", err)
	}
	defer f.Close()
	tw := tar.NewWriter(f)
	defer tw.Close()
	for name, content := range entries {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("writing header for %q: %s", name, err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatalf("writing content for %q: %s", name, err)
		}
	}
}

// setupImagePath points the package-level config at a fresh temp dir and
// creates the lts/img/blobs subdirectories preload/serialize expect, same
// as what happens at real server startup.
func setupImagePath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	config.SetImagePath(dir)
	if err := serialize.CreateDirs(dir, true); err != nil {
		t.Fatalf("CreateDirs: %s", err)
	}
	return dir
}

// --- docker save format ---

// buildDockerSaveTar hand-builds a tarball shaped like a real (single-image)
// `docker save` output: manifest.json at the root, referencing a config
// file and a gzip layer file by their real content digests.
func buildDockerSaveTar(t *testing.T, dir string, ref string) string {
	t.Helper()

	rawLayer, gzLayer := gzipLayer(t, "hello world")
	diffIDHex := digestHex(rawLayer)
	layerDigestHex := digestHex(gzLayer) // layer descriptors reference the *compressed* digest

	configBytes := minimalImageConfig(diffIDHex)
	configDigestHex := digestHex(configBytes)

	manifestJSON, err := json.Marshal([]map[string]any{
		{
			"Config":   configDigestHex + ".json",
			"RepoTags": []string{ref},
			"Layers":   []string{layerDigestHex + ".tar.gz"},
		},
	})
	if err != nil {
		t.Fatalf("marshalling manifest.json: %s", err)
	}

	tarPath := filepath.Join(dir, "docker-save.tar")
	writeTarFile(t, tarPath, map[string][]byte{
		configDigestHex + ".json":  configBytes,
		layerDigestHex + ".tar.gz": gzLayer,
		"manifest.json":            manifestJSON,
	})
	return tarPath
}

// buildDockerSaveTarMultiImage hand-builds a docker-save-shaped tarball
// containing more than one image (a manifest.json array with more than one
// element), each with distinct content so they get distinct digests. Used
// specifically to exercise guardResolver.
func buildDockerSaveTarMultiImage(t *testing.T, dir string, refs []string) string {
	t.Helper()

	var manifestEntries []map[string]any
	entries := map[string][]byte{}

	for i, ref := range refs {
		rawLayer, gzLayer := gzipLayer(t, fmt.Sprintf("hello world %d", i))
		diffIDHex := digestHex(rawLayer)
		layerDigestHex := digestHex(gzLayer)

		configBytes := minimalImageConfig(diffIDHex)
		configDigestHex := digestHex(configBytes)

		entries[configDigestHex+".json"] = configBytes
		entries[layerDigestHex+".tar.gz"] = gzLayer
		manifestEntries = append(manifestEntries, map[string]any{
			"Config":   configDigestHex + ".json",
			"RepoTags": []string{ref},
			"Layers":   []string{layerDigestHex + ".tar.gz"},
		})
	}

	manifestJSON, err := json.Marshal(manifestEntries)
	if err != nil {
		t.Fatalf("marshalling manifest.json: %s", err)
	}
	entries["manifest.json"] = manifestJSON

	tarPath := filepath.Join(dir, "docker-save-multi.tar")
	writeTarFile(t, tarPath, entries)
	return tarPath
}

func TestLoadTarball_DockerSave(t *testing.T) {
	imagePath := setupImagePath(t)
	workDir := t.TempDir()
	ref := "test.example.io/hello:v1"

	tarPath := buildDockerSaveTar(t, workDir, ref)

	if err := LoadTarball(tarPath, nil); err != nil {
		t.Fatalf("LoadTarball: %s", err)
	}

	// We don't know the manifest digest a priori here since go-containerregistry
	// synthesizes the actual distribution-shaped manifest bytes (this project
	// doesn't - and shouldn't - reimplement that). Walk the cache directory
	// instead of predicting the digest.
	found := false
	err := serialize.WalkTheCache(imagePath, func(mh imgpull.ManifestHolder, _ os.FileInfo) error {
		if mh.ImageUrl == ref {
			found = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkTheCache: %s", err)
	}
	if !found {
		t.Fatalf("expected a cached manifest for %q, found none", ref)
	}
}

// gzipFile reads plainTarPath and writes a gzip-compressed copy at
// gzPath, exercising the same magic-byte detection ensurePlainTar uses
// (real .tar.gz/.tgz content, not a renamed plain tar).
func gzipFile(t *testing.T, plainTarPath string, gzPath string) {
	t.Helper()
	raw, err := os.ReadFile(plainTarPath)
	if err != nil {
		t.Fatalf("reading %q: %s", plainTarPath, err)
	}
	out, err := os.Create(gzPath)
	if err != nil {
		t.Fatalf("creating %q: %s", gzPath, err)
	}
	defer out.Close()
	gw := gzip.NewWriter(out)
	if _, err := gw.Write(raw); err != nil {
		t.Fatalf("gzipping %q: %s", gzPath, err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("closing gzip writer for %q: %s", gzPath, err)
	}
}

// TestLoadTarball_DockerSave_Gzip exercises ensurePlainTar: LoadTarball
// should transparently handle a gzip-compressed docker-save tarball
// (whether named .tar.gz or .tgz - detection is by magic bytes, not
// filename), not just an already-plain .tar.
func TestLoadTarball_DockerSave_Gzip(t *testing.T) {
	imagePath := setupImagePath(t)
	workDir := t.TempDir()
	ref := "test.example.io/hello:v1"

	plainTarPath := buildDockerSaveTar(t, workDir, ref)

	for _, ext := range []string{".tar.gz", ".tgz"} {
		t.Run(ext, func(t *testing.T) {
			gzPath := filepath.Join(workDir, "docker-save"+ext)
			gzipFile(t, plainTarPath, gzPath)

			if err := LoadTarball(gzPath, nil); err != nil {
				t.Fatalf("LoadTarball(%q): %s", gzPath, err)
			}

			found := false
			err := serialize.WalkTheCache(imagePath, func(mh imgpull.ManifestHolder, _ os.FileInfo) error {
				if mh.ImageUrl == ref {
					found = true
				}
				return nil
			})
			if err != nil {
				t.Fatalf("WalkTheCache: %s", err)
			}
			if !found {
				t.Fatalf("expected a cached manifest for %q after loading %q, found none", ref, gzPath)
			}
		})
	}
}

// TestLoadTarball_DockerSave_CraneDigestPlaceholderTag exercises RefResolver
// against a real-world quirk: `crane pull --format tarball <ref>@sha256:...`
// (i.e. pulling by digest rather than tag) writes a RepoTags entry like
// "index.docker.io/grafana/grafana:i-was-a-digest" - a syntactically valid
// ref (it parses fine via pullrequest.NewPullRequestFromUrl: dotted host,
// real repo, some string after the colon), but a useless placeholder rather
// than a real tag. Without a resolver able to override an already-valid-
// looking candidate, this tag would sail through untouched. This test
// supplies a resolver that recognizes the placeholder and substitutes a
// real tag, and asserts the image ends up cached under that real tag - not
// the placeholder.
func TestLoadTarball_DockerSave_CraneDigestPlaceholderTag(t *testing.T) {
	imagePath := setupImagePath(t)
	workDir := t.TempDir()

	placeholderRef := "index.docker.io/grafana/grafana:i-was-a-digest"
	realRef := "index.docker.io/grafana/grafana:12.4.1"

	tarPath := buildDockerSaveTar(t, workDir, placeholderRef)

	resolver := func(candidate string, digest string) string {
		if candidate == placeholderRef {
			return realRef
		}
		return candidate
	}

	if err := LoadTarball(tarPath, resolver); err != nil {
		t.Fatalf("LoadTarball: %s", err)
	}

	var foundReal, foundPlaceholder bool
	err := serialize.WalkTheCache(imagePath, func(mh imgpull.ManifestHolder, _ os.FileInfo) error {
		switch mh.ImageUrl {
		case realRef:
			foundReal = true
		case placeholderRef:
			foundPlaceholder = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkTheCache: %s", err)
	}
	if !foundReal {
		t.Fatalf("expected a cached manifest for the resolved ref %q, found none", realRef)
	}
	if foundPlaceholder {
		t.Fatalf("found a manifest cached under the crane placeholder tag %q - resolver override was not honored", placeholderRef)
	}
}

// TestLoadTarball_DockerSave_RefResolverRejectedForMultiImageTarball
// exercises guardResolver: a RefResolver is only meaningful for a tarball
// known to contain exactly one image, since it's called once per image with
// no way to tell which image it's currently being asked about beyond that
// image's own candidate/digest. Supplying one against a multi-image
// tarball should be rejected outright rather than silently doing something
// resolver-writer-didn't-expect to the other images.
func TestLoadTarball_DockerSave_RefResolverRejectedForMultiImageTarball(t *testing.T) {
	workDir := t.TempDir()
	refs := []string{
		"test.example.io/hello:v1",
		"test.example.io/other:v1",
	}
	tarPath := buildDockerSaveTarMultiImage(t, workDir, refs)

	t.Run("resolver supplied - rejected", func(t *testing.T) {
		setupImagePath(t)
		resolver := func(candidate string, digest string) string { return candidate }
		err := LoadTarball(tarPath, resolver)
		if err == nil {
			t.Fatalf("expected LoadTarball to reject a RefResolver against a %d-image tarball, got no error", len(refs))
		}
	})

	t.Run("no resolver - succeeds", func(t *testing.T) {
		imagePath := setupImagePath(t)
		if err := LoadTarball(tarPath, nil); err != nil {
			t.Fatalf("LoadTarball without a resolver should succeed for a multi-image tarball whose tags are already valid: %s", err)
		}
		var foundCount int
		err := serialize.WalkTheCache(imagePath, func(mh imgpull.ManifestHolder, _ os.FileInfo) error {
			for _, ref := range refs {
				if mh.ImageUrl == ref {
					foundCount++
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("WalkTheCache: %s", err)
		}
		if foundCount != len(refs) {
			t.Fatalf("expected %d cached manifests, found %d", len(refs), foundCount)
		}
	})
}

// --- OCI layout format ---

// buildOciLayoutTar hand-builds a tarball shaped like `ctr image export`
// output: the oci-layout marker, index.json, and content-addressed blobs
// under blobs/sha256/.
func buildOciLayoutTar(t *testing.T, dir string, ref string) string {
	t.Helper()

	_, gzLayer := gzipLayer(t, "hello world")
	layerDigestHex := digestHex(gzLayer)

	// OCI layout doesn't require diff_ids to be cross-checked the way
	// go-containerregistry's docker-save reader does, but a realistic config
	// is still used for fidelity.
	configBytes := minimalImageConfig(layerDigestHex)
	configDigestHex := digestHex(configBytes)

	manifest := map[string]any{
		"schemaVersion": 2,
		"mediaType":     "application/vnd.oci.image.manifest.v1+json",
		"config": map[string]any{
			"mediaType": "application/vnd.oci.image.config.v1+json",
			"digest":    "sha256:" + configDigestHex,
			"size":      len(configBytes),
		},
		"layers": []map[string]any{
			{
				"mediaType": "application/vnd.oci.image.layer.v1.tar+gzip",
				"digest":    "sha256:" + layerDigestHex,
				"size":      len(gzLayer),
			},
		},
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshalling manifest: %s", err)
	}
	manifestDigestHex := digestHex(manifestBytes)

	index := map[string]any{
		"schemaVersion": 2,
		"manifests": []map[string]any{
			{
				"mediaType": "application/vnd.oci.image.manifest.v1+json",
				"digest":    "sha256:" + manifestDigestHex,
				"size":      len(manifestBytes),
				"annotations": map[string]string{
					"org.opencontainers.image.ref.name": ref,
				},
			},
		},
	}
	indexBytes, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("marshalling index.json: %s", err)
	}

	ociLayoutBytes := []byte(`{"imageLayoutVersion":"1.0.0"}`)

	tarPath := filepath.Join(dir, "oci-layout.tar")
	writeTarFile(t, tarPath, map[string][]byte{
		"oci-layout":                       ociLayoutBytes,
		"index.json":                       indexBytes,
		"blobs/sha256/" + configDigestHex:   configBytes,
		"blobs/sha256/" + layerDigestHex:    gzLayer,
		"blobs/sha256/" + manifestDigestHex: manifestBytes,
	})
	return tarPath
}

func TestLoadTarball_OciLayout(t *testing.T) {
	imagePath := setupImagePath(t)
	workDir := t.TempDir()
	ref := "test.example.io/hello:v1"

	tarPath := buildOciLayoutTar(t, workDir, ref)

	if err := LoadTarball(tarPath, nil); err != nil {
		t.Fatalf("LoadTarball: %s", err)
	}

	found := false
	err := serialize.WalkTheCache(imagePath, func(mh imgpull.ManifestHolder, _ os.FileInfo) error {
		if mh.ImageUrl == ref {
			found = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkTheCache: %s", err)
	}
	if !found {
		t.Fatalf("expected a cached manifest for %q, found none", ref)
	}
}
