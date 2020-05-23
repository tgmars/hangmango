package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"log"
	"os"
)

// initialiseEncryption() ... returns the 2048 bit RSA private and public keys used for encryption
// to the server. If a key has been generated on this server it won't regenerate and will use the
// existing one.
// TODO: Encrypt the private key at rest.
func initialiseEncryption() (rsa.PrivateKey, rsa.PublicKey, []byte) {
	keypath := "./app/server/hangmangoprivate.pem"

	if !fileExists(keypath) {
		log.Printf("- CRYPTO - No keys identified on disk, generating...")
		size := 2048
		key, err := rsa.GenerateKey(rand.Reader, size)
		if err != nil {
			log.Printf("- CRYPTO - %s", err)
		}
		err = key.Validate()
		if err != nil {
			log.Printf("- CRYPTO - key failed to validate - %s", err)
		}
		kObj, err := json.Marshal(key.PublicKey)
		if err != nil {
			log.Printf("- ENCODING - %s", err)
		}

		log.Printf("- CRYPTO - Keys generated, writing private key to file.")

		pemPrivateFile, err := os.Create(keypath)
		if err != nil {
			log.Printf(" - CRYPTO - failed to open handle to %s - %s", keypath, err)
			os.Exit(1)
		}

		pemPrivateBlock := &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		}

		err = pem.Encode(pemPrivateFile, pemPrivateBlock)
		if err != nil {
			log.Printf(" - CRYPTO - failed to encode encryption private key %s - %s", keypath, err)
			os.Exit(1)
		}
		pemPrivateFile.Close()

		return *key, key.PublicKey, kObj
	} else {
		// Keys are on disk, we just read them in.
		privateKeyFile, err := os.Open(keypath)
		if err != nil {
			log.Printf("- CRYPTO - Expected to read from %s but failed to open handle - %s", keypath, err)
			os.Exit(1)
		}
		pemfileinfo, _ := privateKeyFile.Stat()
		var size int64 = pemfileinfo.Size()
		pembytes := make([]byte, size)
		buffer := bufio.NewReader(privateKeyFile)
		_, err = buffer.Read(pembytes)
		data, _ := pem.Decode([]byte(pembytes))
		privateKeyFile.Close()

		key, err := x509.ParsePKCS1PrivateKey(data.Bytes)
		if err != nil {
			log.Printf("- CRYPTO - Failed to unmarshal private key object from %s bytes - %s", keypath, err)
			os.Exit(1)
		}
		err = key.Validate()
		if err != nil {
			log.Printf("- CRYPTO - key failed to validate - %s", err)
		}
		kObj, err := json.Marshal(key.PublicKey)
		if err != nil {
			log.Printf("- ENCODING - %s", err)
		}
		return *key, key.PublicKey, kObj
	}
}

func encrypt(message []byte, pubkey rsa.PublicKey) []byte {
	hash := sha256.New()
	out, err := rsa.EncryptOAEP(hash, rand.Reader, &pubkey, message, nil)
	if err != nil {
		log.Printf("- ERROR - Encryption - Output will not be passed on - %s", err)
		return nil
	}
	return out
}

func decrypt(message []byte, privkey rsa.PrivateKey) []byte {
	hash := sha256.New()
	plaintext, err := rsa.DecryptOAEP(hash, rand.Reader, &privkey, message, nil)
	if err != nil {
		log.Printf("- ERROR - Decryption - Output will not be passed on - %s", err)
		return nil
	}
	return plaintext
}

// generateAESKeyBytes ... returns securely generated random bytes.
// Returns an error if it can't read from the OS's secure random source.
func generateSymmetricKeyBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// encryptAEADGCM ... encrypts the plaintext with AEAD in GCM mode
// and panics if any cipher or block operations error.
func encryptAEADGCM(key []byte, plaintext []byte) ([]byte, []byte) {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}

	// Never use more than 2^32 random nonces with a given key because of the risk of a repeat.
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err.Error())
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	return aesgcm.Seal(nil, nonce, plaintext, nil), nonce
}

// encryptAEADGCM ... encrypts the plaintext with AEAD in GCM mode
// and panics if any cipher or block operations error.
func decryptAEADGCM(key []byte, ciphertext []byte, nonce []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		panic(err.Error())
	}
	return plaintext
}
