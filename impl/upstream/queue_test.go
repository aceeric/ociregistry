package upstream

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"ociregistry/impl/pullrequest"
	"ociregistry/mock"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// CertSetup is just a place to store and return the output of the certsetup
// function.
type CertSetup struct {
	// serverTLSConf has the server cert
	serverTLSConf *tls.Config
	// clientTLSConf has the client cert
	clientTLSConf *tls.Config
	// caPEM has the CA in PEM form
	caPEM *bytes.Buffer
	// cliCertPEM has the client cert in PEM form
	cliCertPEM *bytes.Buffer
	// cliCertPrivKeyPEM has the client key in PEM form
	cliCertPrivKeyPEM *bytes.Buffer
}

type tlsType string

const (
	ONEWAY tlsType = "1way"
	MTLS   tlsType = "mtls"
)

type tlsVerifyType string

const (
	INSECURE tlsVerifyType = "insecure"
	OS       tlsVerifyType = "os"
	OSFAKE   tlsVerifyType = "osfake"
	BYO      tlsVerifyType = "byo"
	BYOOTHER tlsVerifyType = "byo-other"
)

type expectType bool

const (
	PASS expectType = true
	FAIL expectType = false
)

type testParams struct {
	scheme mock.SchemeType
	auth   mock.AuthType
	tls    tlsType
	verify tlsVerifyType
}
type testConfiguration struct {
	server  testParams
	client  testParams
	expect  expectType
	comment string
}

var alltests = []testConfiguration{
	{server: testParams{scheme: mock.HTTP, auth: mock.NONE}, client: testParams{scheme: mock.HTTP, auth: mock.NONE}, expect: PASS},
	{server: testParams{scheme: mock.HTTP, auth: mock.BASIC}, client: testParams{scheme: mock.HTTP, auth: mock.BASIC}, expect: PASS},
	{server: testParams{scheme: mock.HTTP, auth: mock.BASIC}, client: testParams{scheme: mock.HTTP, auth: mock.NONE}, expect: FAIL, comment: "server wants auth, client does not provide"},
	{server: testParams{scheme: mock.HTTP, auth: mock.NONE}, client: testParams{scheme: mock.HTTPS, auth: mock.NONE, tls: ONEWAY, verify: INSECURE}, expect: PASS, comment: "client will always try HTTP even though configured for HTTPS"},
	{server: testParams{scheme: mock.HTTPS, auth: mock.NONE, tls: ONEWAY}, client: testParams{scheme: mock.HTTPS, auth: mock.NONE, tls: ONEWAY, verify: INSECURE}, expect: PASS},
	{server: testParams{scheme: mock.HTTPS, auth: mock.NONE, tls: ONEWAY}, client: testParams{scheme: mock.HTTPS, auth: mock.NONE, tls: ONEWAY, verify: BYO}, expect: PASS},
	{server: testParams{scheme: mock.HTTPS, auth: mock.NONE, tls: MTLS}, client: testParams{scheme: mock.HTTPS, auth: mock.NONE, tls: MTLS, verify: INSECURE}, expect: PASS},
	{server: testParams{scheme: mock.HTTPS, auth: mock.NONE, tls: MTLS}, client: testParams{scheme: mock.HTTPS, auth: mock.NONE, tls: MTLS, verify: BYO}, expect: PASS},
	{server: testParams{scheme: mock.HTTPS, auth: mock.BASIC, tls: ONEWAY}, client: testParams{scheme: mock.HTTPS, auth: mock.BASIC, tls: ONEWAY, verify: INSECURE}, expect: PASS},
	{server: testParams{scheme: mock.HTTPS, auth: mock.BASIC, tls: ONEWAY}, client: testParams{scheme: mock.HTTPS, auth: mock.BASIC, tls: ONEWAY, verify: BYO}, expect: PASS},
	{server: testParams{scheme: mock.HTTPS, auth: mock.BASIC, tls: MTLS}, client: testParams{scheme: mock.HTTPS, auth: mock.BASIC, tls: MTLS, verify: INSECURE}, expect: PASS},
	{server: testParams{scheme: mock.HTTPS, auth: mock.BASIC, tls: MTLS}, client: testParams{scheme: mock.HTTPS, auth: mock.BASIC, tls: MTLS, verify: BYO}, expect: PASS},
	{server: testParams{scheme: mock.HTTPS, auth: mock.BASIC, tls: MTLS}, client: testParams{scheme: mock.HTTPS, auth: mock.BASIC, tls: MTLS, verify: BYOOTHER}, expect: FAIL, comment: "client BYO CA didn't sign the server cert"},
	{server: testParams{scheme: mock.HTTPS, auth: mock.NONE, tls: ONEWAY}, client: testParams{scheme: mock.HTTPS, auth: mock.NONE, tls: ONEWAY, verify: OS}, expect: FAIL, comment: "ditto"},
	{server: testParams{scheme: mock.HTTPS, auth: mock.NONE, tls: MTLS}, client: testParams{scheme: mock.HTTPS, auth: mock.NONE, tls: MTLS, verify: OS}, expect: FAIL, comment: "ditto"},
	{server: testParams{scheme: mock.HTTPS, auth: mock.BASIC, tls: ONEWAY}, client: testParams{scheme: mock.HTTPS, auth: mock.BASIC, tls: ONEWAY, verify: OS}, expect: FAIL, comment: "ditto"},
	{server: testParams{scheme: mock.HTTPS, auth: mock.BASIC, tls: MTLS}, client: testParams{scheme: mock.HTTPS, auth: mock.BASIC, tls: MTLS, verify: OS}, expect: FAIL, comment: "ditto"},
}

var fakeOsTrustStoreTests = []testConfiguration{
	{server: testParams{scheme: mock.HTTPS, auth: mock.NONE, tls: ONEWAY}, client: testParams{scheme: mock.HTTPS, auth: mock.NONE, tls: ONEWAY, verify: OSFAKE}, expect: PASS, comment: "Validate the server cert with a fake truststore"},
	{server: testParams{scheme: mock.HTTPS, auth: mock.NONE, tls: MTLS}, client: testParams{scheme: mock.HTTPS, auth: mock.NONE, tls: MTLS, verify: OSFAKE}, expect: PASS, comment: "ditto"},
	{server: testParams{scheme: mock.HTTPS, auth: mock.BASIC, tls: ONEWAY}, client: testParams{scheme: mock.HTTPS, auth: mock.BASIC, tls: ONEWAY, verify: OSFAKE}, expect: PASS, comment: "ditto"},
	{server: testParams{scheme: mock.HTTPS, auth: mock.BASIC, tls: MTLS}, client: testParams{scheme: mock.HTTPS, auth: mock.BASIC, tls: MTLS, verify: OSFAKE}, expect: PASS, comment: "ditto"},
}

func TestAllGets(t *testing.T) {
	certSetup, err := createCerts()
	if err != nil {
		t.Fail()
	}
	d, _ := os.MkdirTemp("", "")
	defer os.RemoveAll(d)

	clientCertFile := filepath.Join(d, "client.crt")
	clientKeyFile := filepath.Join(d, "client.key")
	caFile := filepath.Join(d, "ca.crt")
	othercaFile := filepath.Join(d, "otherca.crt")
	os.WriteFile(clientCertFile, certSetup.cliCertPEM.Bytes(), 0777)
	os.WriteFile(clientKeyFile, certSetup.cliCertPrivKeyPEM.Bytes(), 0777)
	os.WriteFile(caFile, certSetup.caPEM.Bytes(), 0777)

	for idx, tc := range alltests {
		mp, cfg, expect := parseTestConfiguration(tc, clientCertFile, clientKeyFile, caFile, othercaFile, certSetup.serverTLSConf)
		if !doOneGet(mp, cfg, expect) {
			t.Logf("failed iteration %d\n", idx)
			t.FailNow()
		}
	}
}

// It appears that once the Golang TLS subsystem is initialized on first use, it is blind
// to subsequent changes to the TLS environment (e.g. by virtue of setting environment variables
// to point to different trust stores). So it is not possible (or I can't figure out how) to
// test with different OS trust stores in the same test run. So what I did was comment out
// the t.SkipNow() statement and manually run this test function to verify that the client
// was able to verify a server cert from the OS trust store using a "fake" root CA generated
// by the test.
func TestFakeOsTrustStoreGets(t *testing.T) {
	t.SkipNow()
	certSetup, err := createCerts()
	if err != nil {
		t.Fail()
	}
	_, _, otherCAPEM, err := createCA("did not sign any certs")
	if err != nil {
		t.Fail()
	}
	d, _ := os.MkdirTemp("", "")
	defer os.RemoveAll(d)

	clientCertFile := filepath.Join(d, "client.crt")
	clientKeyFile := filepath.Join(d, "client.key")
	caFile := filepath.Join(d, "ca.crt")
	othercaFile := filepath.Join(d, "otherca.crt")
	os.WriteFile(clientCertFile, certSetup.cliCertPEM.Bytes(), 0777)
	os.WriteFile(clientKeyFile, certSetup.cliCertPrivKeyPEM.Bytes(), 0777)
	os.WriteFile(caFile, certSetup.caPEM.Bytes(), 0777)
	os.WriteFile(othercaFile, otherCAPEM.Bytes(), 0777)
	t.Setenv("SSL_CERT_FILE", caFile)
	for idx, tc := range fakeOsTrustStoreTests {
		mp, cfg, expect := parseTestConfiguration(tc, clientCertFile, clientKeyFile, caFile, othercaFile, certSetup.serverTLSConf)
		if !doOneGet(mp, cfg, expect) {
			t.Logf("failed iteration %d\n", idx)
			t.FailNow()
		}
	}
}

func TestCreateCerts(t *testing.T) {
	_, err := createCerts()
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

// parseTestConfiguration converts the passed 'testConfiguration' into structs needed to
// configure the mock OCI Distribution Server and the Crane client. It returns those two
// structs as well as the expected result (PASS/FAIL) of the test config. PASS means
// Crane is espected to be able to get the manifest from the mock OCI Distribution Server
// and FAIL means Crane is expected to *not* be able to get the manifest from the upstream
// registry.
func parseTestConfiguration(testConfig testConfiguration, clientCertFile, clientKeyFile, caFile string, othercaFile string, serverTLSConf *tls.Config) (mock.MockParams, cfgEntry, expectType) {
	cliCfg := cfgEntry{}
	cliCfg.Tls = tlsCfg{}
	if testConfig.client.scheme == mock.HTTPS {
		if testConfig.client.tls == MTLS {
			cliCfg.Tls.Cert = clientCertFile
			cliCfg.Tls.Key = clientKeyFile
		}
		switch testConfig.client.verify {
		case OS:
			cliCfg.Tls.Insecure = false
		case OSFAKE:
			cliCfg.Tls.Insecure = false
		case BYO:
			cliCfg.Tls.CA = caFile
		case BYOOTHER:
			cliCfg.Tls.CA = othercaFile
		case INSECURE:
			cliCfg.Tls.Insecure = true
		}
	}
	cliCfg.Auth = authCfg{}
	if testConfig.client.auth == mock.BASIC {
		cliCfg.Auth = authCfg{
			User:     "frodo",
			Password: "baggins",
		}
	}
	mockServerParams := mock.MockParams{
		Scheme: testConfig.server.scheme,
		Auth:   testConfig.server.auth,
	}
	if testConfig.server.scheme == mock.HTTPS {
		mockServerParams.TlsConfig = serverTLSConf
		switch testConfig.server.tls {
		case MTLS:
			// Don't attempt to deal with server valiation of the client cert. We
			// just want to force the client to send it. In the real world the server
			// might validate the client cert but we're not testing the upstream
			// server's mTLS capability - we're testing the client's adherence to its
			// configuration directives.
			mockServerParams.CliAuth = tls.RequireAnyClientCert
		default:
			mockServerParams.CliAuth = tls.NoClientCert
		}
	}
	return mockServerParams, cliCfg, testConfig.expect
}

// doOneGet starts up the mock OCI Distribution server with a configuration, and
// sets the package 'config' map with the passed config entry then performs a
// Crane Get. (Crane will use the config.) If the Crane Get result matches the
// 'expectType', then the function returns PASS, else it returns FAIL.
func doOneGet(mp mock.MockParams, ce cfgEntry, expectPass expectType) expectType {
	d, _ := os.MkdirTemp("", "")
	defer os.RemoveAll(d)
	server, url := mock.Server(mp)
	defer server.Close()
	ce.Name = url
	config = make(map[string]cfgEntry)
	config[url] = ce
	pr := pullrequest.NewPullRequest("", "hello-world", "latest", url)
	_, err := Get(pr, d, 60000)
	if err != nil && expectPass {
		return FAIL
	} else if err == nil && !expectPass {
		return FAIL
	}
	return PASS
}

// createCerts was adapted from https://gist.github.com/shaneutt/5e1995295cff6721c89a71d13a71c251
// It generates:
//
//	a self-signed CA
//	a server cert and key signed by the CA
//	a client cert and key signed by the CA
//
// Also returns the CA, client cert, & clientkey all three as PEMs as a convenience
// because the caller will use then to configure files that in turn configure
// the client.
func createCerts() (CertSetup, error) {
	ca, caPrivKey, caPEM, err := createCA("root")
	if err != nil {
		return CertSetup{}, err
	}

	// server certificate
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			CommonName: "server",
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	certPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return CertSetup{}, err
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &certPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return CertSetup{}, err
	}

	certPEM := new(bytes.Buffer)
	pem.Encode(certPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	certPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(certPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey),
	})

	serverCert, err := tls.X509KeyPair(certPEM.Bytes(), certPrivKeyPEM.Bytes())
	if err != nil {
		return CertSetup{}, err
	}

	// client certificate
	cliCert := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			CommonName: "client",
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	cliCertPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return CertSetup{}, err
	}

	cliCertBytes, err := x509.CreateCertificate(rand.Reader, cliCert, ca, &cliCertPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return CertSetup{}, err
	}

	cliCertPEM := new(bytes.Buffer)
	pem.Encode(cliCertPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cliCertBytes,
	})

	cliCertPrivKeyPEM := new(bytes.Buffer)
	pem.Encode(cliCertPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(cliCertPrivKey),
	})

	clientCert, err := tls.X509KeyPair(cliCertPEM.Bytes(), cliCertPrivKeyPEM.Bytes())
	if err != nil {
		return CertSetup{}, err
	}

	certpool := x509.NewCertPool()
	certpool.AppendCertsFromPEM(caPEM.Bytes())

	serverTLSConf := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		RootCAs:      certpool,
	}

	clientTLSConf := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certpool,
	}

	return CertSetup{
		serverTLSConf:     serverTLSConf,
		clientTLSConf:     clientTLSConf,
		caPEM:             caPEM,
		cliCertPEM:        cliCertPEM,
		cliCertPrivKeyPEM: cliCertPrivKeyPEM,
	}, nil
}

// createCA creates a CA with the passed Common Name. The cert, private key, and
// PEM CA are returned
func createCA(cn string) (*x509.Certificate, *rsa.PrivateKey, *bytes.Buffer, error) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// CA private and public key
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, err
	}

	// create the CA
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, nil, nil, err
	}

	// pem encode CA
	caPEM := new(bytes.Buffer)
	pem.Encode(caPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	return ca, caPrivKey, caPEM, nil
}
