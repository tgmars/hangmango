package main

import (
	"bufio"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"log"
	"os"
)

func initialiseSigning() (x509.Certificate, []byte, *rsa.PublicKey) {
	cert, certBytes := parseCert()
	return cert, certBytes, cert.PublicKey.(*rsa.PublicKey)
}

// generateCert ... Provided a root certificate and its provided key, sign the struct of the x509 certificate
// provided in the first argument. Function also writes out the certificate and private key object (containing priv and pub key)
// from the to certPath and keyPath. The private key is to be used to sign messages from the server, and the certificate is
// to be distributed with clients.
func parseCert() (x509.Certificate, []byte) {
	certPath := "./app/client/hangmango.crt"

	if !fileExists(certPath) {
		log.Printf("- ERROR - No existing certificate at %s - Ensure the file is present and try again.", certPath)
		os.Exit(1)
	} else {
		// Certificate used for verification is already on disk, we'll use that to verify our messages from the server
		certFile, err := os.Open(certPath)
		if err != nil {
			log.Printf("- CRYPTO - Expected to read from %s but failed to open handle - %s", certPath, err)
			os.Exit(1)
		}
		pemfileinfo, _ := certFile.Stat()
		var size int64 = pemfileinfo.Size()
		pembytes := make([]byte, size)
		buffer := bufio.NewReader(certFile)
		_, err = buffer.Read(pembytes)
		data, _ := pem.Decode([]byte(pembytes))
		certFile.Close()

		serverCert, err := x509.ParseCertificate(data.Bytes)
		if err != nil {
			log.Printf("- CRYPTO - Failed to unmarshal certificate object from %s bytes - %s", certPath, err)
			os.Exit(1)
		}
		err = serverCert.VerifyHostname("127.0.0.1")
		if err != nil {
			log.Printf("- CRYPTO - certificate failed to validate for hostname - %s", err)
		}
		return *serverCert, data.Bytes
	}
	return x509.Certificate{}, nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// Sign signs data with rsa-sha256
func (enc *encryptedMessage) Sign(r *rsa.PrivateKey) {
	hash := sha256.New()
	hash.Write(enc.A)
	hashed := hash.Sum(nil)
	signature, err := rsa.SignPSS(rand.Reader, r, crypto.SHA256, hashed, nil)
	if err != nil {
		os.Exit(1)
	}
	enc.B = signature
}
