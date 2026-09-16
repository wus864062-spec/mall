package biz

import (
	"context"
	"encoding/hex"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestFenToUsdtWeiUsesFenNotUstdDisplay(t *testing.T) {
	if FenToUsdtWei(0).Sign() != 0 {
		t.Fatal("zero")
	}
	got := FenToUsdtWei(U(1))
	want := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	if got.Cmp(want) != 0 {
		t.Fatalf("1U wei %s want %s", got, want)
	}
	if FenToUsdtWei(U(9)).Cmp(new(big.Int).Mul(want, big.NewInt(9))) != 0 {
		t.Fatal("9U")
	}
}

func TestWithdrawNetFenLeavesFeeInHotWallet(t *testing.T) {
	if WithdrawNetFen(U(20), 10) != U(18) {
		t.Fatal("20 申请打 18，手续费 2 留热钱包")
	}
	if WithdrawNetFen(U(8), 5) != U(8)-U(8)*5/100 {
		t.Fatal("5%")
	}
	if WithdrawNetFen(U(10), 100) != 0 {
		t.Fatal("全额手续费净额 0")
	}
}

func TestParseHotKeyRejectsEmpty(t *testing.T) {
	if _, err := parseHotKey(""); err != ErrWithdrawHotWallet {
		t.Fatalf("got %v", err)
	}
	if _, err := parseHotKey("zz"); err != ErrWithdrawHotWallet {
		t.Fatalf("bad hex %v", err)
	}
}

func TestErc20TransferDataSelector(t *testing.T) {
	to := common.HexToAddress("0xcccccccccccccccccccccccccccccccccccccccc")
	data := erc20TransferData(to, FenToUsdtWei(U(18)))
	if len(data) != 68 {
		t.Fatalf("len %d", len(data))
	}
	if hex.EncodeToString(data[:4]) != "a9059cbb" {
		t.Fatalf("selector %x", data[:4])
	}
}

func TestDisabledHotWallet(t *testing.T) {
	_, err := disabledHotWallet{}.SendUSDT(context.Background(), "0xcccccccccccccccccccccccccccccccccccccccc", U(1))
	if err != ErrWithdrawHotWallet {
		t.Fatalf("%v", err)
	}
}

func TestLoadHotWalletKeyFromFileNotEnv(t *testing.T) {
	t.Setenv("MALL_HOT_WALLET_KEY", "")
	t.Setenv("MALL_HOT_WALLET_KEY_FILE", "")
	if _, ok := DefaultHotWalletPayer().(disabledHotWallet); !ok {
		t.Fatal("无钥应为 disabled")
	}
	k, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	hexKey := hex.EncodeToString(crypto.FromECDSA(k))
	path := filepath.Join(t.TempDir(), "hot.env")
	if err := os.WriteFile(path, []byte("MALL_HOT_WALLET_KEY=0x"+hexKey+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MALL_HOT_WALLET_KEY_FILE", path)
	p := DefaultHotWalletPayer()
	want := crypto.PubkeyToAddress(k.PublicKey).Hex()
	if HotWalletAddress(p) != want {
		t.Fatalf("addr %s want %s", HotWalletAddress(p), want)
	}
}
