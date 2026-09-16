package biz

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	DefaultUSDTBSC = "0x55d398326f99059fF775485246999027B3197955"
	BSCChainID     = 56
)

// HotWalletPayer 把净额 USDT 转到用户登录钱包。netFen 是申请额−手续费，不是申请额。
type HotWalletPayer interface {
	SendUSDT(ctx context.Context, to string, netFen int64) (txHash string, err error)
}

// FenToUsdtWei 内部 fen → BSC USDT wei。1 USDT = 10000 fen = 1e18 wei，所以 wei = fen * 1e14。
func FenToUsdtWei(netFen int64) *big.Int {
	if netFen <= 0 {
		return big.NewInt(0)
	}
	return new(big.Int).Mul(big.NewInt(netFen), new(big.Int).Exp(big.NewInt(10), big.NewInt(14), nil))
}

// WithdrawNetFen 链上打款额 = 申请额 − 手续费。手续费留在热钱包。
func WithdrawNetFen(amountFen, feePercent int64) int64 {
	if amountFen <= 0 {
		return 0
	}
	n := amountFen - WithdrawFeeAmount(amountFen, feePercent)
	if n < 0 {
		return 0
	}
	return n
}

type disabledHotWallet struct{}

func (disabledHotWallet) SendUSDT(context.Context, string, int64) (string, error) {
	return "", ErrWithdrawHotWallet
}

func DefaultHotWalletPayer() HotWalletPayer {
	key := loadHotWalletKey()
	if key == "" {
		return disabledHotWallet{}
	}
	return NewRPCHotWallet(DefaultBSCRPC(), key, usdtTokenAddress())
}

// loadHotWalletKey 优先环境变量；没有则读 MALL_HOT_WALLET_KEY_FILE（KEY= 行）。
// 只认文件路径，不默认扫家目录，避免 go test 误用真钥打链。
func loadHotWalletKey() string {
	if v := strings.TrimSpace(os.Getenv("MALL_HOT_WALLET_KEY")); v != "" {
		return v
	}
	path := strings.TrimSpace(os.Getenv("MALL_HOT_WALLET_KEY_FILE"))
	if path == "" {
		return ""
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return keyFromEnvFile(raw)
}

func keyFromEnvFile(raw []byte) string {
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if rest, ok := strings.CutPrefix(line, "MALL_HOT_WALLET_KEY="); ok {
			return strings.Trim(strings.TrimSpace(rest), `"'`)
		}
	}
	return ""
}

func HotWalletAddress(p HotWalletPayer) string {
	hw, ok := p.(*rpcHotWallet)
	if !ok {
		return ""
	}
	key, err := parseHotKey(hw.keyHex)
	if err != nil {
		return ""
	}
	return crypto.PubkeyToAddress(key.PublicKey).Hex()
}

func usdtTokenAddress() string {
	if v := strings.TrimSpace(os.Getenv("MALL_USDT_ADDRESS")); v != "" {
		return NormalizeEthAddress(v)
	}
	return DefaultUSDTBSC
}

type rpcHotWallet struct {
	mu     sync.Mutex
	rpcURL string
	keyHex string
	usdt   string
	client *http.Client
}

func NewRPCHotWallet(rpcURL, keyHex, usdt string) *rpcHotWallet {
	return &rpcHotWallet{
		rpcURL: rpcURL,
		keyHex: strings.TrimSpace(keyHex),
		usdt:   NormalizeEthAddress(usdt),
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

func parseHotKey(hexKey string) (*ecdsa.PrivateKey, error) {
	hexKey = strings.TrimSpace(hexKey)
	hexKey = strings.TrimPrefix(hexKey, "0x")
	hexKey = strings.TrimPrefix(hexKey, "0X")
	key, err := crypto.HexToECDSA(hexKey)
	if err != nil {
		return nil, ErrWithdrawHotWallet
	}
	return key, nil
}

func (p *rpcHotWallet) SendUSDT(ctx context.Context, to string, netFen int64) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	to = NormalizeEthAddress(to)
	if len(to) < 42 {
		return "", ErrInvalidArgument
	}
	wei := FenToUsdtWei(netFen)
	if wei.Sign() <= 0 {
		return "", ErrInvalidArgument
	}
	key, err := parseHotKey(p.keyHex)
	if err != nil {
		return "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	from := crypto.PubkeyToAddress(key.PublicKey)
	usdt := common.HexToAddress(p.usdt)
	dest := common.HexToAddress(to)
	data := erc20TransferData(dest, wei)
	var chainHex string
	if err := p.call(ctx, "eth_chainId", nil, &chainHex); err != nil {
		return "", ErrWithdrawChain
	}
	chainID := hexToBig(chainHex)
	if chainID == nil || chainID.Int64() != BSCChainID {
		return "", ErrWithdrawChain
	}
	var nonceHex string
	if err := p.call(ctx, "eth_getTransactionCount", []interface{}{from.Hex(), "pending"}, &nonceHex); err != nil {
		return "", ErrWithdrawChain
	}
	nonce := hexToUint64(nonceHex)
	var tipHex string
	if err := p.call(ctx, "eth_maxPriorityFeePerGas", nil, &tipHex); err != nil {
		return "", ErrWithdrawChain
	}
	tip := hexToBig(tipHex)
	if tip == nil || tip.Sign() == 0 {
		tip = big.NewInt(1_000_000_000)
	}
	var block map[string]json.RawMessage
	if err := p.call(ctx, "eth_getBlockByNumber", []interface{}{"latest", false}, &block); err != nil {
		return "", ErrWithdrawChain
	}
	base := hexToBig(rawJSONString(block["baseFeePerGas"]))
	if base == nil {
		return "", ErrWithdrawChain
	}
	feeCap := new(big.Int).Add(new(big.Int).Mul(base, big.NewInt(2)), tip)
	estIn := map[string]string{
		"from": from.Hex(),
		"to":   usdt.Hex(),
		"data": "0x" + hex.EncodeToString(data),
	}
	var gasHex string
	if err := p.call(ctx, "eth_estimateGas", []interface{}{estIn}, &gasHex); err != nil {
		return "", ErrWithdrawChain
	}
	gas := hexToUint64(gasHex)
	if gas == 0 {
		gas = 80000
	}
	hash, err := p.signAndSend(ctx, key, chainID, nonce, tip, feeCap, gas, usdt, data)
	if err != nil {
		return "", err
	}
	if err := p.waitReceipt(ctx, hash); err != nil {
		return "", err
	}
	return hash, nil
}

func (p *rpcHotWallet) waitReceipt(ctx context.Context, hash string) error {
	deadline := time.Now().Add(40 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ErrWithdrawChain
		default:
		}
		raw, err := p.callRaw(ctx, "eth_getTransactionReceipt", []interface{}{hash})
		if err == nil && len(raw) > 0 && string(raw) != "null" {
			var rec rpcReceipt
			if json.Unmarshal(raw, &rec) == nil {
				if strings.ToLower(strings.TrimSpace(rec.Status)) == "0x1" {
					return nil
				}
				return ErrWithdrawChain
			}
		}
		time.Sleep(1500 * time.Millisecond)
	}
	return ErrWithdrawChain
}

func (p *rpcHotWallet) call(ctx context.Context, method string, params []interface{}, out interface{}) error {
	raw, err := p.callRaw(ctx, method, params)
	if err != nil {
		return err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return ErrWithdrawChain
	}
	return json.Unmarshal(raw, out)
}

func (p *rpcHotWallet) callRaw(ctx context.Context, method string, params []interface{}) (json.RawMessage, error) {
	if params == nil {
		params = []interface{}{}
	}
	body, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.rpcURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var env rpcEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if env.Error != nil {
		return nil, ErrWithdrawChain
	}
	return env.Result, nil
}

func (p *rpcHotWallet) signAndSend(ctx context.Context, key *ecdsa.PrivateKey, chainID *big.Int, nonce uint64, tip, feeCap *big.Int, gas uint64, to common.Address, data []byte) (string, error) {
	payload := rlpList(
		rlpBig(chainID),
		rlpUint(nonce),
		rlpBig(tip),
		rlpBig(feeCap),
		rlpUint(gas),
		rlpBytes(to.Bytes()),
		rlpBig(big.NewInt(0)),
		rlpBytes(data),
		[]byte{0xc0}, // empty accessList
	)
	sigHash := crypto.Keccak256Hash(append([]byte{0x02}, payload...))
	sig, err := crypto.Sign(sigHash.Bytes(), key)
	if err != nil || len(sig) != 65 {
		return "", ErrWithdrawChain
	}
	raw := append([]byte{0x02}, rlpList(
		rlpBig(chainID),
		rlpUint(nonce),
		rlpBig(tip),
		rlpBig(feeCap),
		rlpUint(gas),
		rlpBytes(to.Bytes()),
		rlpBig(big.NewInt(0)),
		rlpBytes(data),
		[]byte{0xc0},
		rlpUint(uint64(sig[64]%2)),
		rlpBytes(sig[:32]),
		rlpBytes(sig[32:64]),
	)...)
	var hash string
	if err := p.call(ctx, "eth_sendRawTransaction", []interface{}{"0x" + hex.EncodeToString(raw)}, &hash); err != nil {
		return "", ErrWithdrawChain
	}
	hash = NormalizeTxHash(hash)
	if hash == "" {
		hash = NormalizeTxHash(crypto.Keccak256Hash(raw).Hex())
	}
	return hash, nil
}

func rlpBytes(b []byte) []byte {
	if len(b) == 1 && b[0] < 0x80 {
		return b
	}
	return rlpPrefix(b, 0x80)
}

func rlpBig(n *big.Int) []byte {
	if n == nil || n.Sign() == 0 {
		return []byte{0x80}
	}
	return rlpBytes(n.Bytes())
}

func rlpUint(n uint64) []byte {
	return rlpBig(new(big.Int).SetUint64(n))
}

func rlpList(items ...[]byte) []byte {
	var body []byte
	for _, it := range items {
		body = append(body, it...)
	}
	return rlpPrefix(body, 0xc0)
}

func rlpPrefix(b []byte, offset byte) []byte {
	n := len(b)
	if n <= 55 {
		return append([]byte{offset + byte(n)}, b...)
	}
	ln := big.NewInt(int64(n)).Bytes()
	return append(append([]byte{offset + 55 + byte(len(ln))}, ln...), b...)
}

func erc20TransferData(to common.Address, amount *big.Int) []byte {
	selector := crypto.Keccak256([]byte("transfer(address,uint256)"))[:4]
	out := make([]byte, 4+64)
	copy(out, selector)
	copy(out[4+12:], to.Bytes())
	amt := amount.Bytes()
	if len(amt) > 32 {
		amt = amt[len(amt)-32:]
	}
	copy(out[4+64-len(amt):], amt)
	return out
}

func hexToBig(s string) *big.Int {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.ToLower(s), "0x")
	if s == "" {
		return big.NewInt(0)
	}
	n := new(big.Int)
	if _, ok := n.SetString(s, 16); !ok {
		return nil
	}
	return n
}

func hexToUint64(s string) uint64 {
	n := hexToBig(s)
	if n == nil || !n.IsUint64() {
		return 0
	}
	return n.Uint64()
}

func rawJSONString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return strings.Trim(string(raw), `"`)
}
