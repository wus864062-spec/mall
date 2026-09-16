package biz

import "testing"

func TestNormalizeProfile(t *testing.T) {
	n, a, err := NormalizeProfile("  Alice  ", "")
	if err != nil || n != "Alice" || a != "" {
		t.Fatalf("got %q %q %v", n, a, err)
	}
	if _, _, err := NormalizeProfile("", ""); err == nil {
		t.Fatal("empty nickname")
	}
	if _, _, err := NormalizeProfile("ok", "javascript:alert(1)"); err == nil {
		t.Fatal("bad avatar")
	}
	if _, _, err := NormalizeProfile("ok", "https://cdn.example.com/a.png"); err != nil {
		t.Fatal(err)
	}
	long := make([]rune, 33)
	for i := range long {
		long[i] = '啊'
	}
	if _, _, err := NormalizeProfile(string(long), ""); err == nil {
		t.Fatal("too long")
	}
}

func TestLineUserIDs(t *testing.T) {
	users := map[int64]*User{
		1: {ID: 1},
		2: {ID: 2, InviterID: 1},
		3: {ID: 3, InviterID: 2},
		4: {ID: 4, InviterID: 1},
		5: {ID: 5, InviterID: 9},
	}
	got := LineUserIDs(users, 1)
	seen := map[int64]bool{}
	for _, id := range got {
		seen[id] = true
	}
	if !seen[1] || !seen[2] || !seen[3] || !seen[4] || seen[5] || len(got) != 4 {
		t.Fatalf("line=%v", got)
	}
}
