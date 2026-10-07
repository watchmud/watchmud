package telnet

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeCert puts a self-signed certificate for localhost, numbered serial,
// into dir as cert.pem and key.pem, dated mtime.
func writeCert(t *testing.T, dir string, serial int64, mtime time.Time) (certFile, keyFile string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	keyDer, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	certFile, keyFile = filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	require.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer}), 0o600))
	require.NoError(t, os.Chtimes(certFile, mtime, mtime))
	require.NoError(t, os.Chtimes(keyFile, mtime, mtime))
	return certFile, keyFile
}

func servedSerial(t *testing.T, c *certificate) int64 {
	t.Helper()
	got, err := c.get(nil)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(got.Certificate[0])
	require.NoError(t, err)
	return leaf.SerialNumber.Int64()
}

// A renewed certificate is picked up on the next handshake: restarting to
// load it would disconnect every player.
func TestCertificate_reloadsWhenRenewed(t *testing.T) {
	dir := t.TempDir()
	start := time.Now().Add(-time.Hour)
	certFile, keyFile := writeCert(t, dir, 1, start)

	c, err := loadCertificate(certFile, keyFile)
	require.NoError(t, err)
	assert.EqualValues(t, 1, servedSerial(t, c))

	writeCert(t, dir, 2, start.Add(time.Minute))
	assert.EqualValues(t, 2, servedSerial(t, c), "the renewal")
}

// A renewal caught half-copied -- the new cert beside the old key, or a
// truncated file -- keeps serving the old one, and tries again next time.
func TestCertificate_keepsTheOldOneThroughABadRenewal(t *testing.T) {
	dir := t.TempDir()
	start := time.Now().Add(-time.Hour)
	certFile, keyFile := writeCert(t, dir, 1, start)
	c, err := loadCertificate(certFile, keyFile)
	require.NoError(t, err)

	later := start.Add(time.Minute)
	require.NoError(t, os.WriteFile(certFile, []byte("-----BEGIN CERTIF"), 0o600))
	require.NoError(t, os.Chtimes(certFile, later, later))
	assert.EqualValues(t, 1, servedSerial(t, c), "still the old one")

	writeCert(t, dir, 3, later.Add(time.Minute)) // the copy finishes
	assert.EqualValues(t, 3, servedSerial(t, c))
}

// A certificate that can't be read at startup stops the server: TLS was asked
// for, and a banner advertising a port that doesn't work is worse.
func TestCertificate_unreadableAtStartup(t *testing.T) {
	_, err := loadCertificate(filepath.Join(t.TempDir(), "nope.pem"), "nope.key")
	assert.Error(t, err)
}

// tlsListener serves TLS on loopback with a fresh certificate, sharing limit.
func tlsListener(t *testing.T, limit *addressLimit, handshake time.Duration) string {
	t.Helper()
	certFile, keyFile := writeCert(t, t.TempDir(), 1, time.Now())
	cert, err := loadCertificate(certFile, keyFile)
	require.NoError(t, err)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	l := listener{ln: ln, tls: tlsConfig(cert), handshakeTimeout: handshake, banner: "Welcome to WatchMUD.\r\n"}
	go func() { _ = serve(ctx, l, &fakeServer{passwords: map[string]string{}}, nil, limit) }()
	return ln.Addr().String()
}

func newLimit(max int) *addressLimit {
	return &addressLimit{max: max, open: make(map[string]int)}
}

func dialTLS(addr string) (*tls.Conn, error) {
	d := &net.Dialer{Timeout: 2 * time.Second}
	return tls.DialWithDialer(d, "tcp", addr, &tls.Config{InsecureSkipVerify: true}) // self-signed
}

func TestTLS_theGameOverTLS(t *testing.T) {
	addr := tlsListener(t, newLimit(5), 2*time.Second)

	tc, err := dialTLS(addr)
	require.NoError(t, err)
	defer tc.Close()
	require.NoError(t, tc.SetReadDeadline(time.Now().Add(2*time.Second)))
	r := bufio.NewReader(tc)
	line, err := r.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "Welcome to WatchMUD.", strings.TrimSpace(line))
	assert.GreaterOrEqual(t, tc.ConnectionState().Version, uint16(tls.VersionTLS12))
}

// A plain telnet client on the TLS port, or anything that connects and says
// nothing, is dropped when the handshake times out -- and gives its slot back.
func TestTLS_noHandshakeIsDropped(t *testing.T) {
	limit := newLimit(1)
	addr := tlsListener(t, limit, 100*time.Millisecond)

	nc, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer nc.Close()
	require.NoError(t, nc.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, err = bufio.NewReader(nc).ReadString('\n')
	assert.Error(t, err, "hung up on")

	// the one slot is free again
	require.Eventually(t, func() bool {
		tc, err := dialTLS(addr)
		if err != nil {
			return false
		}
		tc.Close()
		return true
	}, 2*time.Second, 20*time.Millisecond)
}

// The cap is per address across both ports: TLS isn't five more.
func TestTLS_sharesTheCapWithTelnet(t *testing.T) {
	limit := newLimit(1)
	addr := tlsListener(t, limit, 2*time.Second)
	ok, _ := limit.acquire("127.0.0.1")
	require.True(t, ok, "a telnet connection holds the only slot")

	tc, err := dialTLS(addr)
	if err == nil {
		defer tc.Close()
		require.NoError(t, tc.SetReadDeadline(time.Now().Add(2*time.Second)))
		_, err = bufio.NewReader(tc).ReadString('\n')
	}
	assert.Error(t, err, "refused")
}
