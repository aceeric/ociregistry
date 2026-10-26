package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/aceeric/imgpull/pkg/imgpull/v1oci"
	"github.com/aceeric/ociregistry/cmd/subcmd"
	"github.com/aceeric/ociregistry/impl/globals"
	"github.com/aceeric/ociregistry/impl/serialize"
)

// Test the top-level ociregistry commands that function as CLIs (they perform
// an action and then immediately exit to the console.)
func TestTopLvlCLIs(t *testing.T) {
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(td)
	serialize.CreateDirs(td, true)
	imgFilename := filepath.Join(td, "images")
	os.WriteFile(imgFilename, []byte(""), 0777)

	imgFilenameBad := filepath.Join(td, "images-bad")
	os.WriteFile(imgFilenameBad, []byte("ZZZZ\n"), 0777)

	// create a manifest for list to find
	mh := imgpull.ManifestHolder{
		Type:     imgpull.V1ociManifest,
		Digest:   "1111111111111111111111111111111111111111111111111111111111111111",
		ImageUrl: "foo.io/foo:v1.2.3",
	}
	err = serialize.MhToFilesystem(mh, td, true)
	if err != nil {
		t.Fail()
	}

	testCases := []struct {
		name      string
		args      []string
		expResult int
	}{
		{name: "No command", args: []string{"bin/ociregistry"}, expResult: 0},
		{name: "Version", args: []string{"bin/ociregistry", "version"}, expResult: 0},
		{name: "Load", args: []string{"bin/ociregistry", "--image-path", td, "load", "--image-file", imgFilename}, expResult: 0},
		{name: "List", args: []string{"bin/ociregistry", "--image-path", td, "list"}, expResult: 0},
		{name: "Prune", args: []string{"bin/ociregistry", "--image-path", td, "prune", "--pattern", "frobozz"}, expResult: 0},
		{name: "Load - invalid image url", args: []string{"bin/ociregistry", "--image-path", td, "load", "--image-file", imgFilenameBad}, expResult: 1},
		{name: "List - regex does not compile", args: []string{"bin/ociregistry", "--image-path", td, "list", "--pattern", "["}, expResult: 1},
		{name: "Prune - invalid date", args: []string{"bin/ociregistry", "--image-path", td, "prune", "--date", "does-not-parse"}, expResult: 1},
	}
	for _, testCase := range testCases {
		setup()
		os.Args = testCase.args
		result := realMain()
		if result != testCase.expResult {
			t.Errorf("ociregistry top-level test case %s failed", testCase.name)
		}
	}
}

// Test the "serve" command in normal mode and --hello-world mode
func TestTopLvlServe(t *testing.T) {
	for _, helloWorldMode := range []bool{true, false} {
		subcmd.InitListener()
		if doTestServe(helloWorldMode) != nil {
			t.FailNow()
		}
	}
}

// Starts the server "serve" sub-command
func doTestServe(helloWorldMode bool) error {
	td, err := os.MkdirTemp("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(td)

	setup()
	os.Args = []string{"bin/ociregistry", "--image-path", td, "serve", "--port", "0"}
	if helloWorldMode {
		os.Args = append(os.Args, "--hello-world")
	}
	result := make(chan int, 1)
	go func() { result <- realMain() }()
	err = waitForEchoListener()
	if err != nil {
		return err
	}
	echoListener := subcmd.GetListener()
	if echoListener == nil {
		return errors.New("failed to get echo listener")
	}
	addr := echoListener.Addr()
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return errors.New("unexpected listener address type")
	}
	// shut the server down
	cmd := fmt.Sprintf("http://localhost:%d/cmd/stop", tcpAddr.Port)
	resp, err := http.Get(cmd)
	if err != nil {
		return err
	}
	resp.Body.Close()
	// wait for the server to stop, so it can't overlap the next test's use of the global
	// configuration and listener
	select {
	case code := <-result:
		if code != 0 {
			return fmt.Errorf("serve exited with %d", code)
		}
	case <-time.After(10 * time.Second):
		return errors.New("timed out waiting for the server to stop")
	}
	return nil
}

// waitForEchoListener waits for the Echo server to initialize. This allows to
// get the port number that the server is listening on.
func waitForEchoListener() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if subcmd.GetListener() != nil {
				return nil
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// legacyCacheDir creates a cache in the legacy layout with one manifest and its blob.
func legacyCacheDir(t *testing.T) string {
	t.Helper()
	td := t.TempDir()
	blob := "2222222222222222222222222222222222222222222222222222222222222222"
	mh := imgpull.ManifestHolder{
		Type:     imgpull.V1ociManifest,
		Digest:   "1111111111111111111111111111111111111111111111111111111111111111",
		ImageUrl: "foo.io/foo:v1.2.3",
		V1ociManifest: v1oci.Manifest{
			Config: v1oci.Descriptor{Digest: "sha256:" + blob, Size: 4},
		},
	}
	b, err := json.Marshal(mh)
	if err != nil {
		t.Fatal(err)
	}
	for dir, files := range map[string]map[string][]byte{
		globals.ImgPath:  {mh.Digest: b},
		globals.BlobPath: {blob: []byte("blob")},
	} {
		if err := os.MkdirAll(filepath.Join(td, dir), 0755); err != nil {
			t.Fatal(err)
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(td, dir, name), content, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return td
}

// runMain runs realMain with the passed args and returns its exit code and what it wrote to
// stderr.
func runMain(t *testing.T, args ...string) (int, string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stderr
	os.Stderr = w
	setup()
	os.Args = append([]string{"bin/ociregistry"}, args...)
	result := realMain()
	os.Stderr = saved
	w.Close()
	out, _ := io.ReadAll(r)
	return result, string(out)
}

// Tests that load, list and prune refuse to run on a cache that hasn't been migrated, and
// leave it untouched.
func TestNotMigratedGate(t *testing.T) {
	td := legacyCacheDir(t)
	imgList := filepath.Join(t.TempDir(), "images")
	os.WriteFile(imgList, []byte(""), 0644)
	for _, args := range [][]string{
		{"--image-path", td, "list"},
		{"--image-path", td, "prune", "--pattern", "foo", "--dry-run"},
		{"--image-path", td, "load", "--image-file", imgList},
	} {
		result, stderr := runMain(t, args...)
		if result != 1 || !strings.Contains(stderr, subcmd.NotMigratedMsg) {
			t.Errorf("%v: expected exit 1 and the migration message, got %d %q", args, result, stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(td, globals.ManifestPath)); !os.IsNotExist(err) {
		t.Error("a refused command must not change the cache, not even create its directories")
	}
}

// Tests the migrate-cache command: a dry run writes nothing, a migration completes, and
// afterwards the CLI commands work and see the migrated manifest.
func TestMigrateCacheCmd(t *testing.T) {
	td := legacyCacheDir(t)
	if result, _ := runMain(t, "--image-path", td, "migrate-cache", "--dry-run"); result != 0 {
		t.Fatalf("dry run: expected exit 0, got %d", result)
	}
	if _, err := os.Stat(filepath.Join(td, globals.ManifestPath)); !os.IsNotExist(err) {
		t.Fatal("a dry run must not write anything")
	}
	if result, _ := runMain(t, "--image-path", td, "migrate-cache"); result != 0 {
		t.Fatalf("migrate: expected exit 0, got %d", result)
	}
	if _, err := os.Stat(filepath.Join(td, globals.ManifestPath, serialize.MigrationMarker)); err != nil {
		t.Fatal("expected the migration marker")
	}
	if _, found := serialize.MhFromFilesystem("foo.io/foo:v1.2.3", td); !found {
		t.Fatal("expected the migrated manifest")
	}
	if result, _ := runMain(t, "--image-path", td, "list"); result != 0 {
		t.Fatalf("list after migration: expected exit 0, got %d", result)
	}
	if result, _ := runMain(t, "--image-path", td, "migrate-cache"); result != 0 {
		t.Fatalf("migrating a migrated cache: expected exit 0, got %d", result)
	}
}

// Tests that migrate-cache exits 1 when a manifest can't be written.
func TestMigrateCacheCmdIncomplete(t *testing.T) {
	td := legacyCacheDir(t)
	// a regular file where the manifests directory should be
	os.WriteFile(filepath.Join(td, globals.ManifestPath), []byte("x"), 0644)
	if result, _ := runMain(t, "--image-path", td, "migrate-cache"); result != 1 {
		t.Fatalf("expected exit 1, got %d", result)
	}
}
