package importer

import (
	"testing"
)

func TestExtractSha(t *testing.T) {
	Extract("/tmp/pulls/test.tar", "/tmp", "docker.io/calico/node@sha256:c505b92c0b63dffe1f09ce64ae9d99cddefb01aafbb2a51d8531f44b0998f248")
}

func TestExtractNullRepotags(t *testing.T) {
	Extract("/tmp/frobozz.tar", "/tmp", "")
}
