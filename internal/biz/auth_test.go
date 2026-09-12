package biz

import (
	"context"
	"testing"
	"time"
)

func TestParseTokenRoundTrip(t *testing.T) {
	InitAuth("test-secret")
	tok, err := GenerateToken(42, "0xabc", 3)
	if err != nil {
		t.Fatal(err)
	}
	uid, wallet, ver, err := ParseToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if uid != 42 || wallet != "0xabc" || ver != 3 {
		t.Fatalf("got %d %s ver=%d", uid, wallet, ver)
	}
}

func TestParseTokenRejectsTampered(t *testing.T) {
	InitAuth("test-secret")
	tok, err := GenerateToken(1, "0x1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ParseToken(tok + "x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseSIWEMessage(t *testing.T) {
	issued := time.Now().UTC().Format(time.RFC3339)
	msg := "127.0.0.1:8000 wants you to sign in with your Ethereum account:\n" +
		"0x1234567890abcdef1234567890abcdef12345678\n\nSign in\n\nURI: http://127.0.0.1:8000\nVersion: 1\nChain ID: 1\nNonce: abc\nIssued At: " + issued
	m, err := ParseSIWEMessage(msg)
	if err != nil {
		t.Fatal(err)
	}
	if m.Domain != "127.0.0.1:8000" || m.Nonce != "abc" {
		t.Fatalf("%+v", m)
	}
	if err := m.Validate(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestHostMatches(t *testing.T) {
	if !hostMatches("127.0.0.1:8000", "127.0.0.1:8000") {
		t.Fatal("exact")
	}
	if !validEthAddress("0x1234567890abcdef1234567890abcdef12345678") {
		t.Fatal("addr")
	}
	if validEthAddress("0xzz") {
		t.Fatal("bad addr")
	}
}
