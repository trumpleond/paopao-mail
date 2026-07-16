package store

import (
	"path/filepath"
	"testing"

	"paopao-api/internal/db"
)

func setupStore(t *testing.T) *AccountStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return NewAccountStore(database)
}

func TestParseCredentialLine(t *testing.T) {
	email, pass, ok := ParseCredentialLine("a@b.com----secret")
	if !ok || email != "a@b.com" || pass != "secret" {
		t.Fatalf("got %q %q %v", email, pass, ok)
	}
	if _, _, ok := ParseCredentialLine("# comment"); ok {
		t.Fatal("comment should skip")
	}
	if _, _, ok := ParseCredentialLine("nocolon"); ok {
		t.Fatal("invalid should fail")
	}
}

func TestImportPickMark(t *testing.T) {
	s := setupStore(t)
	text := "u1@test.com----p1\nu2@test.com----p2\ninvalid\n#c\n"
	res, err := s.ImportLines(text, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Inserted != 2 || res.Invalid != 1 || res.Total != 3 {
		t.Fatalf("import result: %+v", res)
	}

	// second import skips
	res2, err := s.ImportLines("u1@test.com----p1\n", false)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Skipped != 1 {
		t.Fatalf("expected skip, got %+v", res2)
	}

	p1, err := s.PickRandom("xai")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Mark(p1.ID, "xai"); err != nil {
		t.Fatal(err)
	}
	// mark again idempotent
	if err := s.Mark(p1.ID, "xai"); err != nil {
		t.Fatal(err)
	}

	p2, err := s.PickRandom("xai")
	if err != nil {
		t.Fatal(err)
	}
	if p2.ID == p1.ID {
		t.Fatal("should not pick marked account")
	}
	if err := s.Mark(p2.ID, "xai"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.PickRandom("xai"); err != ErrNoAvailable {
		t.Fatalf("expected no available, got %v", err)
	}

	// other platform still available
	p3, err := s.PickRandom("openai")
	if err != nil {
		t.Fatal(err)
	}
	if p3.Email == "" {
		t.Fatal("empty pick")
	}

	if err := s.Unmark(p1.ID, "xai"); err != nil {
		t.Fatal(err)
	}
	p4, err := s.PickRandom("xai")
	if err != nil {
		t.Fatal(err)
	}
	if p4.ID != p1.ID {
		t.Fatalf("after unmark expected id %d got %d", p1.ID, p4.ID)
	}

	st, err := s.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 2 || st.Active != 2 {
		t.Fatalf("stats: %+v", st)
	}
}

func TestDisableRemovesFromPick(t *testing.T) {
	s := setupStore(t)
	if _, err := s.ImportLines("d1@test.com----p1\nd2@test.com----p2\n", false); err != nil {
		t.Fatal(err)
	}
	a, err := s.GetByEmail("d1@test.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Disable(a.ID, "auto-disabled: upstream 201"); err != nil {
		t.Fatal(err)
	}
	// both would be candidates for platform "p"; disabled must never appear
	for i := 0; i < 20; i++ {
		got, err := s.PickRandom("plat-x")
		if err != nil {
			t.Fatal(err)
		}
		if got.ID == a.ID || got.Email == "d1@test.com" {
			t.Fatal("disabled account must not be picked")
		}
	}
	// disable the remaining one → no available
	b, err := s.GetByEmail("d2@test.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Disable(b.ID, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PickRandom("plat-x"); err != ErrNoAvailable {
		t.Fatalf("want ErrNoAvailable, got %v", err)
	}
}

func TestDisableByEmail(t *testing.T) {
	s := setupStore(t)
	if _, err := s.ImportLines("z@test.com----pz\n", false); err != nil {
		t.Fatal(err)
	}
	id, err := s.DisableByEmail("z@test.com", "upstream 201")
	if err != nil || id <= 0 {
		t.Fatalf("disable: id=%d err=%v", id, err)
	}
	d, err := s.GetByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != "disabled" {
		t.Fatalf("status=%s", d.Status)
	}
	if d.Note == "" {
		t.Fatal("expected note")
	}
}
