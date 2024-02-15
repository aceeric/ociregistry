package importer

import (
	"testing"
)

func TestExtractNullRepotags(t *testing.T) {
	Extract("/tmp/frobozz.tar", "/tmp")
}
