//go:build ignore

package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
)

func main() {
	privKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating key: %v\n", err)
		os.Exit(1)
	}

	publicKey := base64.RawURLEncoding.EncodeToString(privKey.PublicKey().Bytes())
	privateKey := base64.RawURLEncoding.EncodeToString(privKey.Bytes())

	fmt.Println("Add these to your env vars:")
	fmt.Printf("VAPID_PUBLIC_KEY=%s\n", publicKey)
	fmt.Printf("VAPID_PRIVATE_KEY=%s\n", privateKey)
}
