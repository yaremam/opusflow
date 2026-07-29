package auth

import (
	"errors"
	"testing"
)

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

func TestListAndDeleteToken(t *testing.T) {
	store := testStore(t)
	svc := NewService(store)

	_, tok, err := svc.CreateToken(ctx(), "Phone")
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	// A second token so deleting the first isn't deleting the last one
	// remaining — that's TestDeleteTokenRefusesToDeleteTheLastRemainingOne's
	// job (issue #59: deleting every token, including the default one,
	// locked the whole app out).
	_, _, err = svc.CreateToken(ctx(), "Tablet")
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	list, err := svc.ListTokens(ctx())
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListTokens = %+v, want two rows", list)
	}

	if err := svc.DeleteToken(ctx(), tok.ID); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}

	list, err = svc.ListTokens(ctx())
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}
	if len(list) != 1 || list[0].ID == tok.ID {
		t.Fatalf("ListTokens after delete = %+v, want only the surviving token", list)
	}
}

func TestDeleteTokenRefusesToDeleteTheLastRemainingOne(t *testing.T) {
	store := testStore(t)
	svc := NewService(store)

	_, tok, err := svc.CreateToken(ctx(), "Only Device")
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	err = svc.DeleteToken(ctx(), tok.ID)
	if !errors.Is(err, ErrLastToken) {
		t.Fatalf("DeleteToken on the only remaining token: err = %v, want ErrLastToken", err)
	}

	list, err := svc.ListTokens(ctx())
	if err != nil {
		t.Fatalf("ListTokens: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListTokens after refused delete = %+v, want the token still present", list)
	}
}
