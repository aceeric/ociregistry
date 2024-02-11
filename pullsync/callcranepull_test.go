package pullsync

import (
	"testing"
)

func TestPullImage(t *testing.T) {
	callCranePull("calico/node@sha256:01547e127b496c622e7d7235f4d8852ba6e9159cdc6de7ffa4ae9c7ffa937b82", "/tmp")
}
