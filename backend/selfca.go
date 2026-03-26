package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// selfTLSSetup loads (or generates) a local CA and server certificate stored
// in cacheDir, and returns a *tls.Config ready for ListenAndServeTLS.
//
// The raw CA certificate PEM is also returned so the server can expose it at
// GET /ca.crt for one-time device trust installation.
//
// Files written to cacheDir:
//
//	ca.key / ca.crt   — local root CA  (generated once, valid 10 years)
//	self.key / self.crt — server cert signed by the CA (valid 1 year, auto-renewed)
func selfTLSSetup(cacheDir string) (*tls.Config, []byte, error) {
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return nil, nil, err
	}

	caKey, caCert, caCertPEM, err := loadOrCreateCA(cacheDir)
	if err != nil {
		return nil, nil, err
	}

	srvCert, err := loadOrCreateServerCert(cacheDir, caKey, caCert)
	if err != nil {
		return nil, nil, err
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{srvCert},
		MinVersion:   tls.VersionTLS13,
	}
	return tlsCfg, caCertPEM, nil
}

// loadOrCreateCA reads ca.key and ca.crt from dir; if either is missing or
// unreadable a new P-256 CA is generated and saved.
func loadOrCreateCA(dir string) (*ecdsa.PrivateKey, *x509.Certificate, []byte, error) {
	keyPath := filepath.Join(dir, "ca.key")
	certPath := filepath.Join(dir, "ca.crt")

	keyPEM, keyErr := os.ReadFile(keyPath)
	certPEM, certErr := os.ReadFile(certPath)
	if keyErr == nil && certErr == nil {
		key, err := parseECKey(keyPEM)
		if err != nil {
			return nil, nil, nil, err
		}
		cert, err := parseCert(certPEM)
		if err != nil {
			return nil, nil, nil, err
		}
		return key, cert, certPEM, nil
	}

	// Generate a new local CA.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	serial, err := randSerial()
	if err != nil {
		return nil, nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "YinMoNote Local CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, nil, err
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, nil, err
	}

	keyPEM, err = marshalECKey(key)
	if err != nil {
		return nil, nil, nil, err
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	if err := atomicWriteFile(keyPath, keyPEM, 0600); err != nil {
		return nil, nil, nil, err
	}
	if err := atomicWriteFile(certPath, certPEM, 0644); err != nil {
		return nil, nil, nil, err
	}
	return key, cert, certPEM, nil
}

// loadOrCreateServerCert returns a valid server certificate signed by the given
// CA.  If self.crt is missing or expires within 30 days a new cert is issued,
// including all current host IPs in the SAN so it works on any interface.
func loadOrCreateServerCert(dir string, caKey *ecdsa.PrivateKey, caCert *x509.Certificate) (tls.Certificate, error) {
	keyPath := filepath.Join(dir, "self.key")
	certPath := filepath.Join(dir, "self.crt")

	if _, err := os.Stat(certPath); err == nil {
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err == nil {
			if x509c, err := x509.ParseCertificate(cert.Certificate[0]); err == nil {
				if time.Until(x509c.NotAfter) > 30*24*time.Hour {
					return cert, nil
				}
			}
		}
	}

	// Issue a new server certificate.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := randSerial()
	if err != nil {
		return tls.Certificate{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "YinMoNote"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  hostIPs(),
		DNSNames:     []string{"localhost"},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, err
	}

	keyPEM, err := marshalECKey(key)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	if err := atomicWriteFile(keyPath, keyPEM, 0600); err != nil {
		return tls.Certificate{}, err
	}
	if err := atomicWriteFile(certPath, certPEM, 0644); err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}

// hostIPs returns all IP addresses to include in the server certificate SAN.
//
// It combines:
//  1. Loopback addresses (127.0.0.1, ::1) — always included for local access.
//  2. All unicast IPs on active network interfaces — covers LAN, VPN, etc.
//  3. Any IPs listed in TLS_EXTRA_IPS (comma-separated) — for NAT/cloud
//     deployments where the public IP is on the gateway and not visible to
//     the local machine (e.g. most cloud VPS providers).
func hostIPs() []net.IP {
	seen := map[string]bool{}
	var ips []net.IP

	add := func(ip net.IP) {
		if ip == nil {
			return
		}
		key := ip.String()
		if !seen[key] {
			seen[key] = true
			ips = append(ips, ip)
		}
	}

	add(net.ParseIP("127.0.0.1"))
	add(net.ParseIP("::1"))

	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip == nil || ip.IsLoopback() {
					continue
				}
				add(ip)
			}
		}
	}

	// TLS_EXTRA_IPS: comma-separated list of additional IPs to include in the
	// certificate SAN. Useful for NAT/cloud servers where the public IP is on
	// the upstream gateway and not assigned to any local interface.
	for _, s := range strings.Split(os.Getenv("TLS_EXTRA_IPS"), ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if ip := net.ParseIP(s); ip != nil {
			add(ip)
		}
	}

	return ips
}

func randSerial() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
}

func parseECKey(pemBytes []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("selfca: no PEM block found in EC key data")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

func parseCert(pemBytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("selfca: no PEM block found in certificate data")
	}
	return x509.ParseCertificate(block.Bytes)
}

func marshalECKey(key *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), nil
}
