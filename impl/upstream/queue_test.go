package upstream

import (
	"fmt"
	"testing"
)

func Test1(t *testing.T) {
	d, err := cranePull("registry.k8s.io/pause:3.8")
	fmt.Printf("%+v, %s\n", d, err)
}

func Test2(t *testing.T) {
	result, err := Get("registry.k8s.io/pause:3.8", "/tmp", 6000)
	fmt.Printf("%+v, %s\n", result, err)
}
