package auth

import "testing"

func TestNewTokenAndHash(t *testing.T) {
	raw, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 43 { // 32 bytes, base64url without padding
		t.Fatalf("raw length %d", len(raw))
	}
	if hash != HashToken(raw) || len(hash) != 64 {
		t.Fatalf("hash mismatch: %s", hash)
	}
	raw2, _, _ := NewToken()
	if raw == raw2 {
		t.Fatal("tokens must be unique")
	}
}

func TestNewID(t *testing.T) {
	a, b := NewID(), NewID()
	if len(a) != 32 || a == b {
		t.Fatalf("bad ids: %s %s", a, b)
	}
}
