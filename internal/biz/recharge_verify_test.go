package biz

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestNormalizeTxHash(t *testing.T) {
	want := "0xabcd1234567890abcdef1234567890abcdef1234567890abcdef1234567890ab"
	if len(want) != 66 {
		t.Fatalf("fixture len %d", len(want))
	}
	if got := NormalizeTxHash("  " + strings.ToUpper(want) + "  "); got != want {
		t.Fatalf("got %s", got)
	}
	if NormalizeTxHash("zz") != "" || NormalizeTxHash("") != "" {
		t.Fatal("invalid hash must be empty")
	}
}

func TestParseRechargedLogs(t *testing.T) {
	buy := "0x1111111111111111111111111111111111111111"
	payer := common.HexToAddress("0x2222222222222222222222222222222222222222")
	lg := ChainLog{
		Address: buy,
		Topics:  []string{RechargedEventTopic.Hex(), common.BytesToHash(payer.Bytes()).Hex()},
		Data:    "0x" + common.Bytes2Hex(common.LeftPadBytes(big.NewInt(100).Bytes(), 32)),
	}
	gotPayer, gotNum, err := ParseRechargedLogs([]ChainLog{lg}, buy)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(gotPayer, payer.Hex()) || gotNum != 100 {
		t.Fatalf("payer %s num %d", gotPayer, gotNum)
	}
	if _, _, err := ParseRechargedLogs([]ChainLog{lg}, "0x3333333333333333333333333333333333333333"); err == nil {
		t.Fatal("expected reject when log is not from buy")
	}
}
