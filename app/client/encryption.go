package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"io"
	"log"
)

func initialiseEncryption() (rsa.PrivateKey, rsa.PublicKey) {
	size := 2048
	key, err := rsa.GenerateKey(rand.Reader, size)
	if err != nil {
		log.Printf(" - CRYPTO - %s", err)
	}
	err = key.Validate()
	if err != nil {
		log.Printf(" - CRYPTO - key failed to validate - %s", err)
	}
	return *key, key.PublicKey
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
