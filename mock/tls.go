package mock

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// TlsType specifies the supported TLS types. This is used by the puller to
// configure its TLS and by the mock server to configure its TLS. So these values are
// interpreted differently depending on perspective (mock server vs. puller). None
// of the tests involve the server verifying client certs at this time. Also at this
// time, testing the puller validating the server certs using the OS trust store is
// not tested.
type TlsType int

const (
	// HTTP, so, no TLS considerations
	NOTLS TlsType = iota
	// Mock server will present certs to puller. Puller will not present certs. Mock
	// server will not request certs. Server certs will not be validated by puller.
	ONEWAY_INSECURE
	// Mock server will present certs to puller. Puller will not present certs. Mock
	// server will not request certs. Server certs will be validated by puller. Therefore
	// puller will need the test CA Cert.
	ONEWAY_SECURE
	// Mock server will present certs to puller. Puller will present certs to server. Mock
	// server will request (any) cert. Server certs will not be validated by puller. Therefore
	// puller will not need the test CA Cert. Server will not validate client certs.
	MTLS_INSECURE
	// Mock server will present certs to puller. Puller will present certs to server. Mock
	// server will request (any) cert. Server certs will be validated by puller. Therefore
	// puller will need the test CA Cert. Server will not validate client certs.
	MTLS_SECURE
)

// CertSetup stores the output of the certSetup function
type CertSetup struct {
	// CaPEM is the CA certificate in PEM form
	CaPEM *bytes.Buffer
	// ServerCert is the server certificate
	ServerCert tls.Certificate
	// ServerCertPEM is the server certificate in PEM form
	ServerCertPEM *bytes.Buffer
	// ServerCertPrivKeyPEM is the server key in PEM form
	ServerCertPrivKeyPEM *bytes.Buffer
	// ClientCert is the client certificate
	ClientCert tls.Certificate
	// ClientCertPEM is the client certificate in PEM form
	ClientCertPEM *bytes.Buffer
	// ClientCertPrivKeyPEM is the client key in PEM form
	ClientCertPrivKeyPEM *bytes.Buffer
}

// CaToFile serializes the CA Certificate in the receiver to a file named 'fileName'
// at the passed 'path' if it does not already exist. In all cases the full path and
// filename are returned to the caller.
func (cs CertSetup) CaToFile(path, fileName string) string {
	p := filepath.Join(path, fileName)
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(p, cs.CaPEM.Bytes(), 0644); err != nil {
			panic(err)
		}
	}
	return p
}

// ServerCertToFile serializes the server cert in the receiver to a file named 'fileName'
// at the passed 'path' if it does not already exist. In all cases the full path and
// filename are returned to the caller.
func (cs CertSetup) ServerCertToFile(path, fileName string) string {
	p := filepath.Join(path, fileName)
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(p, cs.ServerCertPEM.Bytes(), 0644); err != nil {
			panic(err)
		}
	}
	return p
}

// ClientCertToFile serializes the client cert in the receiver to a file named 'fileName'
// at the passed 'path' if it does not already exist. In all cases the full path and
// filename are returned to the caller.
func (cs CertSetup) ClientCertToFile(path, fileName string) string {
	p := filepath.Join(path, fileName)
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(p, cs.ClientCertPEM.Bytes(), 0644); err != nil {
			panic(err)
		}
	}
	return p
}

// ServerCertPrivKeyToFile serializes the server cert's key in the receiver to a file named 'fileName'
// at the passed 'path' if it does not already exist. In all cases the full path and
// filename are returned to the caller.
func (cs CertSetup) ServerCertPrivKeyToFile(path, fileName string) string {
	p := filepath.Join(path, fileName)
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(p, cs.ServerCertPrivKeyPEM.Bytes(), 0644); err != nil {
			panic(err)
		}
	}
	return p
}

// ClientCertPrivKeyToFile serializes the client cert's key in the receiver to a file named 'fileName'
// at the passed 'path' if it does not already exist. In all cases the full path and
// filename are returned to the caller.
func (cs CertSetup) ClientCertPrivKeyToFile(path, fileName string) string {
	p := filepath.Join(path, fileName)
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(p, cs.ClientCertPrivKeyPEM.Bytes(), 0644); err != nil {
			panic(err)
		}
	}
	return p
}

// NewCertSetup was adapted from https://gist.github.com/shaneutt/5e1995295cff6721c89a71d13a71c251
// It returns a fully-populated 'CertSetup' struct, or an error.
func NewCertSetup() (CertSetup, error) {
	cs := CertSetup{}
	caCert, caPrivKey, caPEM, err := createCACert()
	if err != nil {
		return CertSetup{}, err
	}
	cs.CaPEM = caPEM

	serverCert := newX509("server", false)
	cs.ServerCert, cs.ServerCertPEM, cs.ServerCertPrivKeyPEM, err = createCertItems(serverCert, caCert, caPrivKey)
	if err != nil {
		return CertSetup{}, err
	}

	clientCert := newX509("client", false)
	cs.ClientCert, cs.ClientCertPEM, cs.ClientCertPrivKeyPEM, err = createCertItems(clientCert, caCert, caPrivKey)
	if err != nil {
		return CertSetup{}, err
	}
	return cs, nil
}

// createCertItems returns 1) a tls.Certificate created from the passed 'cert' arg, 2) the same
// certificate PEM-encoded, and 3) the PEM-encoded private key for the tls.Certificate in #1
func createCertItems(cert x509.Certificate, caCert x509.Certificate, caPrivKey *rsa.PrivateKey) (tls.Certificate, *bytes.Buffer, *bytes.Buffer, error) {
	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, nil, nil, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &cert, &caCert, &pk.PublicKey, caPrivKey)
	if err != nil {
		return tls.Certificate{}, nil, nil, err
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	privKeyPEM := new(bytes.Buffer)
	pem.Encode(privKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(pk),
	})
	certificate, err := tls.X509KeyPair(certPEM.Bytes(), privKeyPEM.Bytes())
	if err != nil {
		return tls.Certificate{}, nil, nil, err
	}
	return certificate, certPEM, privKeyPEM, nil
}

// createCACert creates a CA certificate with Common Name "root". The certificate,
// private key, and PEM-encoded certificate are returned.
func createCACert() (x509.Certificate, *rsa.PrivateKey, *bytes.Buffer, error) {
	ca := newX509("root", true)
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return x509.Certificate{}, nil, nil, err
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, &ca, &ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return x509.Certificate{}, nil, nil, err
	}
	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	return ca, caPrivKey, caPEM, nil
}

// newX509 returns a new x509 cert with the passed common name. If isCA is true then a CA
// cert is generated, otherwise a non-CA cert.
func newX509(cn string, isCA bool) x509.Certificate {
	keyUsage := x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	if isCA {
		keyUsage |= x509.KeyUsageCertSign
	}
	return x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			CommonName: cn,
		},
		IsCA:                  isCA,
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		SubjectKeyId:          []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              keyUsage,
	}
}
