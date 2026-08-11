package provider

import (
	"strings"
	"testing"
)

func TestCredentialCipherEncryptsAndAuthenticatesCredentials(t *testing.T) {
	cipher := NewCredentialCipher("deployment-secret")
	encrypted, err := cipher.Encrypt("plain-provider-token")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encrypted, "plain-provider-token") {
		t.Fatalf("ciphertext disclosed plaintext: %q", encrypted)
	}
	plaintext, err := cipher.Decrypt(encrypted)
	if err != nil || plaintext != "plain-provider-token" {
		t.Fatalf("Decrypt() = %q, %v", plaintext, err)
	}
	if _, err := NewCredentialCipher("wrong-secret").Decrypt(encrypted); err == nil {
		t.Fatal("wrong key decrypted credential")
	}
}
