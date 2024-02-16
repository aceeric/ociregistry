package upstream

import (
	"fmt"
	"ociregistry/impl/pullrequest"
	"os"
	"testing"
	"time"
)

func Test1(t *testing.T) {
	d, _ := os.MkdirTemp("", "")
	defer os.RemoveAll(d)
	pr := pullrequest.NewPullRequest("", "pause", "3.8", "registry.k8s.io")
	mh, err := Get(pr, d, 60000)
	if err != nil {
		t.Fail()
	}
	fmt.Printf("%+v", mh)
}

func Test2(t *testing.T) {
	d, _ := os.MkdirTemp("", "")
	defer os.RemoveAll(d)
	pr := pullrequest.NewPullRequest("", "pause", "sha256:f5944f2d1daf66463768a1503d0c8c5e8dde7c1674d3f85abc70cef9c7e32e95", "registry.k8s.io")
	mh, err := Get(pr, d, 60000)
	if err != nil {
		t.Fail()
	}
	fmt.Printf("%+v", mh)
}

func Test3(t *testing.T) {
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
