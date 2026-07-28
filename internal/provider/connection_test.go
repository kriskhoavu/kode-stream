package provider

import "testing"

func TestConnectionStoreScopesCredentialsByUserAndInstance(t *testing.T) {
	store := NewConnectionStore()
	if err := store.Save("user-a", "corp", "secret"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Token("user-b", "corp"); ok {
		t.Fatal("credential leaked across users")
	}
	if token, ok := store.Token("user-a", "corp"); !ok || token != "secret" {
		t.Fatalf("token = %q, %v", token, ok)
	}
	store.Revoke("user-a", "corp")
	if _, ok := store.Token("user-a", "corp"); ok {
		t.Fatal("credential remained after revoke")
	}
}
