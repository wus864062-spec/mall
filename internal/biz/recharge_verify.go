package biz

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// RechargedEventTopic = keccak256("Recharged(address,uint256)")，与 BuySomething 事件一致。
var RechargedEventTopic = crypto.Keccak256Hash([]byte("Recharged(address,uint256)"))

const defaultBSCRPC = "https://bsc-dataseed.binance.org/"

// ChainReceipt 是 buy() 回执里核对入账要用的字段：谁付、付了几 U、打到哪个合约。
type ChainReceipt struct {
	Payer string
	Num   int64
	To    string
}

type ChainLog struct {
	Address string
	Topics  []string
	Data    string
}

type ChainVerifier interface {
	VerifyRechargeTx(ctx context.Context, txHash, buyAddr string) (*ChainReceipt, error)
}

func NormalizeTxHash(h string) string {
	h = strings.TrimSpace(strings.ToLower(h))
	if h == "" {
		return ""
	}
	if !strings.HasPrefix(h, "0x") {
		h = "0x" + h
	}
	if len(h) != 66 {
		return ""
	}
	for i := 2; i < len(h); i++ {
		c := h[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return ""
		}
	}
	return h
}

func ParseRechargedLogs(logs []ChainLog, buyAddr string) (payer string, num int64, err error) {
	buy := strings.ToLower(NormalizeEthAddress(buyAddr))
	topic0 := strings.ToLower(RechargedEventTopic.Hex())
	for _, lg := range logs {
		if buy != "" && strings.ToLower(NormalizeEthAddress(lg.Address)) != buy {
			continue
		}
		if len(lg.Topics) < 2 || strings.ToLower(lg.Topics[0]) != topic0 {
			continue
		}
		payer = common.HexToAddress(lg.Topics[1]).Hex()
		data := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(lg.Data)), "0x")
		raw, err := parseHexBytes(data)
		if err != nil {
			return "", 0, ErrRechargeTxInvalid
		}
		n := new(big.Int).SetBytes(raw)
		if !n.IsInt64() || n.Sign() <= 0 {
			return "", 0, ErrRechargeTxInvalid
		}
		return payer, n.Int64(), nil
	}
	return "", 0, ErrRechargeTxInvalid
}

func parseHexBytes(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	if len(s)%2 == 1 {
		s = "0" + s
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(out); i++ {
		a, ok1 := fromHex(s[2*i])
		b, ok2 := fromHex(s[2*i+1])
		if !ok1 || !ok2 {
			return nil, ErrRechargeTxInvalid
		}
		out[i] = a<<4 | b
	}
	return out, nil
}

func fromHex(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

func DefaultBSCRPC() string {
	if v := strings.TrimSpace(os.Getenv("MALL_BSC_RPC")); v != "" {
		return v
	}
	return defaultBSCRPC
}

func DefaultChainVerifier() ChainVerifier {
	return NewRPCChainVerifier(DefaultBSCRPC())
}

func NewRPCChainVerifier(rpcURL string) ChainVerifier {
	if strings.TrimSpace(rpcURL) == "" {
		rpcURL = DefaultBSCRPC()
	}
	return &rpcChainVerifier{
		rpcURL: rpcURL,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

type rpcChainVerifier struct {
	rpcURL string
	client *http.Client
}

type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type rpcEnvelope struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type rpcTx struct {
	To string `json:"to"`
}

type rpcReceipt struct {
	Status string `json:"status"`
	To     string `json:"to"`
	Logs   []struct {
		Address string   `json:"address"`
		Topics  []string `json:"topics"`
		Data    string   `json:"data"`
	} `json:"logs"`
}

func (v *rpcChainVerifier) VerifyRechargeTx(ctx context.Context, txHash, buyAddr string) (*ChainReceipt, error) {
	hash := NormalizeTxHash(txHash)
	buy := common.HexToAddress(buyAddr)
	if hash == "" || buy == (common.Address{}) {
		return nil, ErrRechargeTxInvalid
	}
	var tx rpcTx
	if err := v.call(ctx, "eth_getTransactionByHash", []interface{}{hash}, &tx); err != nil {
		return nil, ErrRechargeTxInvalid
	}
	if common.HexToAddress(tx.To) != buy {
		return nil, ErrRechargeTxInvalid
	}
	var receipt rpcReceipt
	if err := v.call(ctx, "eth_getTransactionReceipt", []interface{}{hash}, &receipt); err != nil {
		return nil, ErrRechargeTxInvalid
	}
	if strings.ToLower(strings.TrimSpace(receipt.Status)) != "0x1" {
		return nil, ErrRechargeTxInvalid
	}
	if receipt.To != "" && common.HexToAddress(receipt.To) != buy {
		return nil, ErrRechargeTxInvalid
	}
	logs := make([]ChainLog, 0, len(receipt.Logs))
	for _, lg := range receipt.Logs {
		logs = append(logs, ChainLog{Address: lg.Address, Topics: lg.Topics, Data: lg.Data})
	}
	payer, num, err := ParseRechargedLogs(logs, buy.Hex())
	if err != nil {
		return nil, err
	}
	return &ChainReceipt{Payer: payer, Num: num, To: buy.Hex()}, nil
}

func (v *rpcChainVerifier) call(ctx context.Context, method string, params []interface{}, out interface{}) error {
	body, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.rpcURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	var env rpcEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return err
	}
	if env.Error != nil || len(env.Result) == 0 || string(env.Result) == "null" {
		return ErrRechargeTxInvalid
	}
	return json.Unmarshal(env.Result, out)
}
