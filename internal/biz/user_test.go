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
