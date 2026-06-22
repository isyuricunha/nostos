package crypto

import (
	"encoding/base64"
	"testing"
)

func TestPasswordHashVerification(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword(hash, "wrong password") {
		t.Fatal("expected wrong password to fail")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	encodedKey := base64.StdEncoding.EncodeToString(key)
	decodedKey, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		t.Fatalf("decode key: %v", err)
	}

	ciphertext, err := Encrypt(decodedKey, "secret-value")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if ciphertext == "secret-value" {
		t.Fatal("ciphertext must not equal plaintext")
	}
	plaintext, err := Decrypt(decodedKey, ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plaintext != "secret-value" {
		t.Fatalf("unexpected plaintext %q", plaintext)
	}
}

func TestHashTokenUsesSecret(t *testing.T) {
	first := HashToken("secret-one", "token")
	second := HashToken("secret-two", "token")
	if first == second {
		t.Fatal("token hashes should change when the secret changes")
	}
}
