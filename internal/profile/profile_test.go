package profile

import "testing"

func TestStoreRoundTrip(t *testing.T) {
	t.Setenv("GITSWITCH_HOME", t.TempDir())
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Add(Profile{Name: "work", GitName: "A", Email: "a@work.com"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(Profile{Name: "WORK", GitName: "B", Email: "b@x.com"}); err == nil {
		t.Error("duplicate name (case-insensitive) should fail")
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	s2, _ := Load()
	if p, ok := s2.Get("Work"); !ok || p.Email != "a@work.com" {
		t.Errorf("round trip lost data: %+v", s2.Profiles)
	}
	if p, ok := s2.MatchIdentity("", "A@WORK.com"); !ok || p.Name != "work" {
		t.Error("email match should be case-insensitive")
	}
	if !s2.Remove("work") || len(s2.Profiles) != 0 {
		t.Error("remove failed")
	}
}

func TestValidate(t *testing.T) {
	bad := []Profile{
		{Name: "", GitName: "x", Email: "a@b.c"},
		{Name: "has space", GitName: "x", Email: "a@b.c"},
		{Name: "ok", GitName: " ", Email: "a@b.c"},
		{Name: "ok", GitName: "x", Email: "nope"},
	}
	for _, p := range bad {
		if Validate(p) == nil {
			t.Errorf("expected error for %+v", p)
		}
	}
}
