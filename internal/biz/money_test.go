package biz

import "testing"

func TestUstdToFenTruncatesTo4Decimals(t *testing.T) {
	if UstdToFen(10.12345) != 101234 {
		t.Fatalf("got %d want 101234", UstdToFen(10.12345))
	}
	if UstdToFen(10.1234) != 101234 {
		t.Fatalf("exact 4dp %d", UstdToFen(10.1234))
	}
	if UstdToFen(1.99999) != 19999 {
		t.Fatalf("no round up %d", UstdToFen(1.99999))
	}
	if UstdToFen(FenToUstd(101234)) != 101234 {
		t.Fatalf("roundtrip %v", FenToUstd(101234))
	}
	if U(10) != 100000 {
		t.Fatalf("U(10)=%d", U(10))
	}
	if FormatUstd(101234) != "10.1234" {
		t.Fatalf("format %s", FormatUstd(101234))
	}
	if FormatUstd(UstdToFen(1.99999)) != "1.9999" {
		t.Fatalf("trunc format %s", FormatUstd(UstdToFen(1.99999)))
	}
}

func TestWithdrawFeeTruncates(t *testing.T) {
	if WithdrawFeeAmount(10001, 10) != 1000 {
		t.Fatalf("10%% of 1.0001 should drop leftover, got %d", WithdrawFeeAmount(10001, 10))
	}
}
