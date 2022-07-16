package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"io"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"sync"
	"time"
)

var crtpath = "example.crt"
var keypath = "example.key"

type CertificateKeyPair struct {
	certMutex sync.RWMutex
	cert      *tls.Certificate
	certPath  string
	keyPath   string
}

var result = CertificateKeyPair{keyPath: keypath, certPath: crtpath}

func NewCertificateKeyPair(certPath, keyPath string) (*CertificateKeyPair, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	result.cert = &cert
	return &result, nil
}

func (kpr *CertificateKeyPair) reloadCert() error {
	newCert, err := tls.LoadX509KeyPair(kpr.certPath, kpr.keyPath)
	if err != nil {
		return err
	}
	kpr.certMutex.Lock()
	defer kpr.certMutex.Unlock()
	kpr.cert = &newCert
	return nil
}

func (kpr *CertificateKeyPair) GetCertificateFunc() func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		kpr.certMutex.RLock()
		defer kpr.certMutex.RUnlock()
		return kpr.cert, nil
	}
}

func writeCertToFile() error {
	randOrg := make([]byte, 32)
	_, err := rand.Read(randOrg)
	template := &x509.Certificate{
		IsCA:                  true,
		BasicConstraintsValid: true,
		SubjectKeyId:          []byte{1},
		SerialNumber:          big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{base64.URLEncoding.EncodeToString(randOrg)},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(30 * time.Minute),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
	}

	privatekey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	publickey := &privatekey.PublicKey
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, publickey, privatekey)

	cert := pem.EncodeToMemory(&pem.Block{
		Type: "CERTIFICATE", Bytes: certDER})
	key := pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privatekey)})

	ioutil.WriteFile(crtpath, cert, 0644)
	ioutil.WriteFile(keypath, key, 0644)
	return nil
}

func reloadHandler(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "reloading certificates\n")
}

func main() {
	http.HandleFunc("/reload-cert", reloadHandler)

	// Generate and write new certificates to a file
	err := writeCertToFile()
	if err != nil {
		log.Fatal("config failed: %s", err)
	}

	done := make(chan struct{})
	go func() {
		// Reload certificate every 30 seconds (or use watcher that checks when the secrets change)
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				log.Printf("Generating new certificates.\n")
				err := writeCertToFile()
				if err != nil {
					log.Printf("error when generating new cert: %v", err)
					continue
				}

				err = result.reloadCert()
				if err != nil {
					log.Printf("error when reloading new cert: %v", err)
					continue
				}
			case <-done:
				return
			}
		}
	}()
	defer close(done)

	kpr, err := NewCertificateKeyPair(crtpath, keypath)
	if err != nil {
		log.Fatal(err)
	}

	config := &tls.Config{
		PreferServerCipherSuites: true,
		MinVersion:               tls.VersionTLS12,
	}
	server := &http.Server{
		Addr:      ":8443",
		TLSConfig: config,
	}
	server.TLSConfig.GetCertificate = kpr.GetCertificateFunc()

	log.Fatal(server.ListenAndServeTLS(crtpath, keypath))
}
