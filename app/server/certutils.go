package main

import (
	"bufio"
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"time"
)

func initialiseSigning() rsa.PrivateKey {
	// caStruct represents the root certificate of the authority chain.
	caStruct := generateCAStruct()
	caPrivKey := generateCASigningPair(caStruct)
	// certStruct represents the certificate to be provided to clients.
	certStruct := generateCertStruct()
	return generateCert(certStruct, caStruct, caPrivKey)
}

// generateCA ... returns an x509.Certificate struct
func generateCAStruct() x509.Certificate {
	ca := x509.Certificate{
		SerialNumber: big.NewInt(2020),
		Subject: pkix.Name{
			Organization:  []string{"UNECOSC540"},
			Country:       []string{"AUS"},
			Province:      []string{"ACT"},
			Locality:      []string{"Canberra"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	return ca
	// generateCASigningPair(ca)
}

func generateCASigningPair(ca x509.Certificate) *rsa.PrivateKey {
	certPath := "./app/server/hangmango.crt"
	keyPath := "./app/server/hangmango-signing.pem"

	if !fileExists(keyPath) && !fileExists(certPath) {
		log.Printf("- CRYPTO - Generating keypair for CA...")
		size := 2048
		caKey, err := rsa.GenerateKey(rand.Reader, size)
		if err != nil {
			log.Printf("- CRYPTO - CA keypair generation error - %s", err)
		}
		err = caKey.Validate()
		if err != nil {
			log.Printf("- CRYPTO - CA keypair generation error -  key failed to validate - %s", err)
		}

		// Signs the certificate struct ca with the specified keys, also returns byte slice representation
		// of the ca.
		caBytes, err := x509.CreateCertificate(rand.Reader, &ca, &ca, &caKey.PublicKey, caKey)
		if err != nil {
			log.Printf("- CRYPTO - failed to create CA certificate - %s", err)
		}

		// Unused
		caPEM := new(bytes.Buffer)
		pem.Encode(caPEM, &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: caBytes,
		})

		// Unused
		caPrivKeyPEM := new(bytes.Buffer)
		pem.Encode(caPrivKeyPEM, &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(caKey),
		})
		return caKey
	}
	return nil
}

// generateCertStruct ... returns a struct containing the metadata
// of the certificates to be distributed with hangmango clients
// With this certificate installed, only servers running on localhost with
// reference to the client can be communicated with.
func generateCertStruct() x509.Certificate {
	cert := x509.Certificate{
		SerialNumber: big.NewInt(2001),
		Subject: pkix.Name{
			Organization:  []string{"UNECOSC540"},
			Country:       []string{"AUS"},
			Province:      []string{"ACT"},
			Locality:      []string{"Canberra"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3},
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	return cert
}

// generateCert ... Provided a root certificate and its provided key, sign the struct of the x509 certificate
// provided in the first argument. Function also writes out the certificate and private key object (containing priv and pub key)
// from the to certPath and keyPath. The private key is to be used to sign messages from the server, and the certificate is
// to be distributed with clients.
func generateCert(cert x509.Certificate, ca x509.Certificate, caKey *rsa.PrivateKey) rsa.PrivateKey {
	certPath := "./app/server/hangmango.crt"
	keyPath := "./app/server/hangmango-signing.pem"

	if !fileExists(keyPath) && !fileExists(certPath) {
		log.Printf("- CRYPTO - No existing file at %s and %s - Generating keypair for signatures...", certPath, keyPath)
		size := 2048
		certKey, err := rsa.GenerateKey(rand.Reader, size)
		if err != nil {
			log.Printf("- CRYPTO - signing keypair generation error - %s", err)
		}
		err = certKey.Validate()
		if err != nil {
			log.Printf("- CRYPTO - signing keypair generation error -  key failed to validate - %s", err)
		}

		certBytes, err := x509.CreateCertificate(rand.Reader, &cert, &ca, &certKey.PublicKey, caKey)
		if err != nil {
			log.Printf("- CRYPTO - failed to create server certificate - %s", err)
			os.Exit(1)
		}

		log.Printf("- CRYPTO - Keys generated, writing private key to file.")

		certPrivKeyFile, err := os.Create(keyPath)
		if err != nil {
			log.Printf(" - CRYPTO - failed to open handle to %s - %s", keyPath, err)
			os.Exit(1)
		}

		err = pem.Encode(certPrivKeyFile, &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(certKey),
		})
		if err != nil {
			log.Printf(" - CRYPTO - failed to encode cert private key %s - %s", keyPath, err)
			os.Exit(1)
		}
		certPrivKeyFile.Close()

		certFile, err := os.Create(certPath)
		if err != nil {
			log.Printf(" - CRYPTO - failed to open handle to %s - %s", certPath, err)
			os.Exit(1)
		}

		err = pem.Encode(certFile, &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certBytes,
		})
		if err != nil {
			log.Printf("- CRYPTO - failed to encode signing certificate %s - %s", certPath, err)
			os.Exit(1)
		}
		certFile.Close()

		// Super hacky but provides the flow that we're after.
		log.Printf("- CRYPTO - Keypairs and certificate generated. Hangmango is closing, please restart to bundle the certificate with clients.")
		os.Exit(1)

		return *certKey
	} else {
		log.Printf("- CRYPTO - cert and key already exist. Loading certificate private key to sign messages outbound from the server.")
		// Private key used for signing is already on disk, we'll use that to sign our messages from the server
		certPrivKeyFile, err := os.Open(keyPath)
		if err != nil {
			log.Printf("- CRYPTO - Expected to read from %s but failed to open handle - %s", keyPath, err)
			os.Exit(1)
		}
		pemfileinfo, _ := certPrivKeyFile.Stat()
		var size int64 = pemfileinfo.Size()
		pembytes := make([]byte, size)
		buffer := bufio.NewReader(certPrivKeyFile)
		_, err = buffer.Read(pembytes)
		data, _ := pem.Decode([]byte(pembytes))
		certPrivKeyFile.Close()

		key, err := x509.ParsePKCS1PrivateKey(data.Bytes)
		if err != nil {
			log.Printf("- CRYPTO - Failed to unmarshal private key object from %s bytes - %s", keyPath, err)
			os.Exit(1)
		}
		err = key.Validate()
		if err != nil {
			log.Printf("- CRYPTO - key failed to validate - %s", err)
		}
		return *key
	}
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// Sign ... signs data with rsa-sha256 and populates the B field of
// an encryptedMessage type.
func (enc *encryptedMessage) Sign(privkey *rsa.PrivateKey) {
	hash := sha256.New()
	hash.Write(enc.A)
	hashed := hash.Sum(nil)
	signature, err := rsa.SignPSS(rand.Reader, privkey, crypto.SHA256, hashed, nil)
	if err != nil {
		os.Exit(1)
	}
	enc.B = signature
}
