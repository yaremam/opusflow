package auth

import "testing"

func TestCreateTokenReturnsPlaintextOnceAndStoresOnlyItsHash(t *testing.T) {
	store := testStore(t)
	svc := NewService(store)

	plaintext, tok, err := svc.CreateToken(ctx(), "Kitchen iPad")
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if plaintext == "" {
		t.Fatalf("CreateToken returned empty plaintext")
	}
	if tok.Name != "Kitchen iPad" {
		t.Fatalf("Name = %q, want %q", tok.Name, "Kitchen iPad")
	}

	valid, err := store.ValidateAndTouch(ctx(), HashToken(plaintext))
	if err != nil {
		t.Fatalf("ValidateAndTouch: %v", err)
	}
	if !valid {
		t.Fatalf("the plaintext CreateToken returned doesn't validate against what was stored")
	}
}

// TestListAndDeleteToken deletes an install's only token — TDR 024:
// nothing enforces tokens anymore, so that can no longer lock anyone out
// of anything (contrast with the now-removed guard from issue #59).
func TestListAndDeleteToken(t *testing.T) {
	store := testStore(t)
	svc := NewService(store)

	_, tok, err := svc.CreateToken(ctx(), "Phone")
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	list, err := svc.ListTokens(ctx())
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}
	if len(list) != 1 || list[0].ID != tok.ID {
		t.Fatalf("ListTokens = %+v, want one row matching %+v", list, tok)
	}

	if err := svc.DeleteToken(ctx(), tok.ID); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}

	list, err = svc.ListTokens(ctx())
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("ListTokens after delete = %+v, want empty", list)
	}
}
