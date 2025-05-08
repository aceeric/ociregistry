package globals

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/aceeric/ociregistry/impl/config"
)

// ParseTls parses the TLS configuration for the server to use to establish TLS with
// downsteam clients like containerd. Supports:
//   - 1-way: we provide our certs to the client and do not request client certs
//   - mTls: we provide our certs to the client, and require and verify client certs
//
// Client cert verification can be via the OS trust store (no CA specified in config), or
// via the provided CA. If there is no TLS configuration, then a nil tls.Config is returned
// to the caller. This means the server should serve on HTTP. Otherwise the specifics
// of the TLS handshake requirements will be encapsulated in the returned struct.
func ParseTls() (*tls.Config, error) {
	tlsCfg := config.GetServerTlsCfg()
	cfg := &tls.Config{}
	hasCfg := false
	if tlsCfg.Cert != "" && tlsCfg.Key != "" {
		if cert, err := tls.LoadX509KeyPair(tlsCfg.Cert, tlsCfg.Key); err != nil {
			return nil, err
		} else {
			cfg.Certificates = []tls.Certificate{cert}
			hasCfg = true
		}
	}
	cliAuth := strings.ToLower(tlsCfg.ClientAuth)
	if !slices.Contains([]string{"", "none", "verify"}, cliAuth) {
		return nil, fmt.Errorf("unsupported client auth value: %s", tlsCfg.ClientAuth)
	}
	if cliAuth == "verify" {
		hasCfg = true
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
		if tlsCfg.CA != "" {
			if caCert, err := os.ReadFile(tlsCfg.CA); err != nil {
				return nil, err
			} else {
				cp := x509.NewCertPool()
				if !cp.AppendCertsFromPEM(caCert) {
					return nil, err
				}
				cfg.ClientCAs = cp
			}
		}
	}
	if hasCfg {
		return cfg, nil
	}
	return nil, nil
}
