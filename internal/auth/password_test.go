package auth

import (
	"errors"
	"strings"
	"testing"
)

var cheap = Hasher{Time: 1, Memory: 8 * 1024, Threads: 1}

func TestHashAndVerify(t *testing.T) {
	h, err := cheap.Hash("correct-horse-battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=8192,t=1,p=1$") {
		t.Fatalf("unexpected format: %s", h)
	}
	if ok, err := Verify("correct-horse-battery", h); err != nil || !ok {
		t.Fatalf("expected match, ok=%v err=%v", ok, err)
	}
	if ok, _ := Verify("wrong-password-123", h); ok {
		t.Fatal("wrong password must not verify")
	}
}

func TestHashUsesRandomSalt(t *testing.T) {
	a, _ := cheap.Hash("same-password-123")
	b, _ := cheap.Hash("same-password-123")
	if a == b {
		t.Fatal("hashes must differ")
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	for _, enc := range []string{"", "plain", "$argon2id$v=19$m=1,t=1,p=1$abc", "$bcrypt$v=19$m=1,t=1,p=1$YQ$YQ"} {
		if _, err := Verify("x", enc); !errors.Is(err, ErrInvalidHash) {
			t.Errorf("%q: expected ErrInvalidHash, got %v", enc, err)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		pw string
		ok bool
	}{
		{strings.Repeat("a", 11), false},
		{strings.Repeat("a", 12), true},
		{strings.Repeat("a", 128), true},
		{strings.Repeat("a", 129), false},
		{strings.Repeat("ą", 12), true}, // counted in runes, not bytes
		{strings.Repeat("ą", 129), false},
	}
	for _, c := range cases {
		err := ValidatePassword(c.pw)
		if c.ok && err != nil || !c.ok && !errors.Is(err, ErrPasswordLength) {
			t.Errorf("len=%d ok=%v err=%v", len([]rune(c.pw)), c.ok, err)
		}
	}
}
