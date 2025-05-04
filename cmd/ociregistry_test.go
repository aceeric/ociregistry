package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aceeric/imgpull/pkg/imgpull"
	"github.com/aceeric/ociregistry/cmd/subcmd"
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
	go realMain()
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
	_, err = http.Get(cmd)
	if err != nil {
		return err
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
