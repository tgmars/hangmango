package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
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
