package auth

import (
	"testing"
	"time"
)

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("s3cret-pw")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "s3cret-pw" {
		t.Fatal("password stored in plaintext")
	}
	if !CheckPassword(hash, "s3cret-pw") {
		t.Fatal("correct password rejected")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("wrong password accepted")
	}
}

func TestIssueAndVerify(t *testing.T) {
	iss := NewIssuer("test-secret", time.Hour)
	token, err := iss.Issue("user-123")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	sub, err := iss.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if sub != "user-123" {
		t.Fatalf("subject = %q, want user-123", sub)
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	iss := NewIssuer("test-secret", -time.Minute)
	token, _ := iss.Issue("user-123")
	if _, err := iss.Verify(token); err == nil {
		t.Fatal("expired token accepted")
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	token, _ := NewIssuer("secret-a", time.Hour).Issue("user-123")
	if _, err := NewIssuer("secret-b", time.Hour).Verify(token); err == nil {
		t.Fatal("token signed with a different secret accepted")
	}
}

func TestVerifyRejectsGarbage(t *testing.T) {
	iss := NewIssuer("test-secret", time.Hour)
	if _, err := iss.Verify("not.a.jwt"); err == nil {
		t.Fatal("garbage token accepted")
	}
}
