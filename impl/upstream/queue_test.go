package upstream

import (
	"ociregistry/impl/pullrequest"
	"ociregistry/mock"
	"os"
	"testing"
	"time"
)

func TestGet(t *testing.T) {
	d, _ := os.MkdirTemp("", "")
	server, mi := mock.Server()
	defer server.Close()
	defer os.RemoveAll(d)
	pr := pullrequest.NewPullRequest("", "hello-world", "latest", mi.Url)
	_, err := Get(pr, d, 60000)
	if err != nil {
		t.Fail()
	}
	pr = pullrequest.NewPullRequest("", "hello-world", mi.ImageManifestDigest, mi.Url)
	_, err = Get(pr, d, 60000)
	if err != nil {
		t.Fail()
	}
}

func TestEnqueueing(t *testing.T) {
	ch := make(chan bool)
	var cnt = 0
	go func() {
		<-ch
		cnt++
		<-ch
		cnt++
	}()
	if enqueueGet("foo", ch) == alreadyEnqueued {
		t.Fail()
	}
	if enqueueGet("foo", ch) != alreadyEnqueued {
		t.Fail()
	}
	doneGet("foo")
	if len(ps.pullMap) != 0 {
		t.Fail()
	}
	time.Sleep(time.Second / 2)
	if cnt != 2 {
		t.Fail()
	}
}
