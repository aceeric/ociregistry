package pullrequest

import (
	"fmt"
	"testing"
)

func Test1(t *testing.T) {
	pr := NewPullRequest("", "hello-world", "latest", "docker.io")
	fmt.Printf("%+v\n", pr)
}
