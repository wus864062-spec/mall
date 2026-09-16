package biz

import "testing"

func TestParseReleaseDaysQuery(t *testing.T) {
	n, err := ParseReleaseDaysQuery("")
	if err != nil || n != 0 {
		t.Fatalf("empty got %d %v", n, err)
	}
	n, err = ParseReleaseDaysQuery("300")
	if err != nil || n != 300 {
		t.Fatalf("300 got %d %v", n, err)
	}
	if _, err := ParseReleaseDaysQuery("200"); err != ErrReleaseDays {
		t.Fatalf("illegal days %v", err)
	}
	if _, err := ParseReleaseDaysQuery("abc"); err != ErrReleaseDays {
		t.Fatalf("non-number %v", err)
	}
}

func TestPackageImageExt(t *testing.T) {
	if _, err := PackageImageExt(nil); err != ErrImageRequired {
		t.Fatalf("empty %v", err)
	}
	if _, err := PackageImageExt([]byte("hello")); err != ErrImageType {
		t.Fatalf("type %v", err)
	}
	ext, err := PackageImageExt([]byte{0xFF, 0xD8, 0xFF, 0x00})
	if err != nil || ext != ".jpg" {
		t.Fatalf("jpg %s %v", ext, err)
	}
	png := []byte("\x89PNG\r\n\x1a\nxxxx")
	ext, err = PackageImageExt(png)
	if err != nil || ext != ".png" {
		t.Fatalf("png %s %v", ext, err)
	}
	tooBig := make([]byte, MaxPackageImageBytes+1)
	tooBig[0], tooBig[1], tooBig[2] = 0xFF, 0xD8, 0xFF
	if _, err := PackageImageExt(tooBig); err != ErrImageTooLarge {
		t.Fatalf("size %v", err)
	}
}
