package auth

import "testing"

func TestGenerateTokenIsUniqueAndPrefixed(t *testing.T) {
	a, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	b, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if a == b {
		t.Fatalf("two calls to GenerateToken produced the same token")
	}
	const prefix = "opusflow_pt_"
	if len(a) <= len(prefix) || a[:len(prefix)] != prefix {
		t.Fatalf("token %q missing expected prefix %q", a, prefix)
	}
}

func TestHashTokenIsDeterministicAndDistinct(t *testing.T) {
	h1 := HashToken("opusflow_pt_abc123")
	h2 := HashToken("opusflow_pt_abc123")
	if h1 != h2 {
		t.Fatalf("HashToken not deterministic: %q != %q", h1, h2)
	}

	h3 := HashToken("opusflow_pt_different")
	if h1 == h3 {
		t.Fatalf("HashToken produced the same hash for different tokens")
	}
}
