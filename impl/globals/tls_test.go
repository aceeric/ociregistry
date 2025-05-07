package globals

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aceeric/ociregistry/impl/config"
	"github.com/aceeric/ociregistry/mock"
)

// client auth types: none, requireAndVerify
var serverTlsConfig = `
serverTlsConfig:
  cert: %[1]s/cert.pem
  key: %[1]s/key.pem
  ca: %[1]s/ca.pem
  clientAuth: verify
`

// Tests empty server TLS config
func TestNoTls(t *testing.T) {
	config.Set(config.Configuration{})
	cfg, err := ParseTls()
	if err != nil {
		t.FailNow()
	}
	if cfg != nil {
		t.FailNow()
	}
}

// Tests that a fully-populated server TLS config is properly loaded
// and parsed
func TestTls(t *testing.T) {
	config.Set(config.Configuration{})
	td, err := os.MkdirTemp("", "")
	if err != nil {
		t.Fail()
	}
	defer os.RemoveAll(td)
	cfgFile := filepath.Join(td, "testcfg.yaml")
	os.WriteFile(cfgFile, []byte(fmt.Sprintf(serverTlsConfig, td)), 0700)
	if config.Load(cfgFile) != nil {
		t.Fail()
	}
	if createCertFiles(td) != nil {
		t.FailNow()
	}
	cfg, err := ParseTls()
	if err != nil {
		t.FailNow()
	}
	if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.FailNow()
	}
}

// createCertFiles create cert, key, and ca for the TLS parse test
func createCertFiles(td string) error {
	certSetup, err := mock.NewCertSetup()
	if err != nil {
		return err
	}
	certSetup.ServerCertToFile(td, "cert.pem")
	certSetup.ServerCertPrivKeyToFile(td, "key.pem")
	certSetup.CaToFile(td, "ca.pem")
	return nil
}
