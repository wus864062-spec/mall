package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-kratos/kratos/v2/transport"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/golang-jwt/jwt/v5"
)

var (
	jwtMu     sync.RWMutex
	jwtSecret []byte

	nonceStore = make(map[string]nonceEntry)
	nonceMu    sync.Mutex
)

const nonceTTL = 5 * time.Minute
const siweSkew = 15 * time.Minute

type nonceEntry struct {
	Nonce     string
	ExpiresAt time.Time
}

func InitAuth(secret string) {
	if secret == "" {
		secret = os.Getenv("JWT_SECRET")
	}
	if secret == "" {
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		secret = hex.EncodeToString(b)
	}
	jwtMu.Lock()
	jwtSecret = []byte(secret)
	jwtMu.Unlock()
}

func currentJWTSecret() []byte {
	jwtMu.RLock()
	if len(jwtSecret) > 0 {
		s := jwtSecret
		jwtMu.RUnlock()
		return s
	}
	jwtMu.RUnlock()
	InitAuth("")
	jwtMu.RLock()
	defer jwtMu.RUnlock()
	return jwtSecret
}

func GenerateNonce(walletAddress string) (string, error) {
	walletAddress = strings.ToLower(strings.TrimSpace(walletAddress))
	if !validEthAddress(walletAddress) {
		return "", ErrInvalidArgument
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	nonce := hex.EncodeToString(b)

	nonceMu.Lock()
	defer nonceMu.Unlock()
	now := time.Now()
	for k, e := range nonceStore {
		if now.After(e.ExpiresAt) {
			delete(nonceStore, k)
		}
	}
	nonceStore[walletAddress] = nonceEntry{
		Nonce:     nonce,
		ExpiresAt: now.Add(nonceTTL),
	}
	return nonce, nil
}

func ConsumeNonce(walletAddress, nonce string) error {
	walletAddress = strings.ToLower(walletAddress)

	nonceMu.Lock()
	defer nonceMu.Unlock()

	entry, ok := nonceStore[walletAddress]
	if !ok {
		return ErrInvalidArgument
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(nonceStore, walletAddress)
		return ErrInvalidArgument
	}
	if entry.Nonce != nonce {
		return ErrInvalidArgument
	}
	delete(nonceStore, walletAddress)
	return nil
}

type SIWEMessage struct {
	Domain   string
	Address  string
	URI      string
	Version  string
	ChainID  string
	Nonce    string
	IssuedAt string
}

var siweFirstLineRe = regexp.MustCompile(`^(.+?) wants you to sign in with your Ethereum account:$`)

func ParseSIWEMessage(message string) (*SIWEMessage, error) {
	lines := strings.Split(message, "\n")
	if len(lines) < 8 {
		return nil, ErrInvalidArgument
	}

	m := &SIWEMessage{}

	matches := siweFirstLineRe.FindStringSubmatch(lines[0])
	if len(matches) != 2 {
		return nil, ErrInvalidArgument
	}
	m.Domain = matches[1]

	m.Address = strings.TrimSpace(lines[1])
	if !validEthAddress(m.Address) {
		return nil, ErrInvalidArgument
	}

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "URI: "):
			m.URI = strings.TrimPrefix(line, "URI: ")
		case strings.HasPrefix(line, "Version: "):
			m.Version = strings.TrimPrefix(line, "Version: ")
		case strings.HasPrefix(line, "Chain ID: "):
			m.ChainID = strings.TrimPrefix(line, "Chain ID: ")
		case strings.HasPrefix(line, "Nonce: "):
			m.Nonce = strings.TrimPrefix(line, "Nonce: ")
		case strings.HasPrefix(line, "Issued At: "):
			m.IssuedAt = strings.TrimPrefix(line, "Issued At: ")
		}
	}

	if m.URI == "" || m.ChainID == "" || m.Nonce == "" || m.IssuedAt == "" {
		return nil, ErrInvalidArgument
	}
	if m.Version != "" && m.Version != "1" {
		return nil, ErrInvalidArgument
	}
	return m, nil
}

func (m *SIWEMessage) Validate(ctx context.Context) error {
	issuedAt, err := time.Parse(time.RFC3339, m.IssuedAt)
	if err != nil {
		issuedAt, err = time.Parse(time.RFC3339Nano, m.IssuedAt)
		if err != nil {
			return ErrInvalidArgument
		}
	}
	now := time.Now()
	if issuedAt.After(now.Add(siweSkew)) || now.Sub(issuedAt) > siweSkew {
		return ErrInvalidArgument
	}

	host, origin := originFromCtx(ctx)
	if host != "" && !hostMatches(m.Domain, host) {
		return ErrInvalidArgument
	}
	if origin != "" {
		uri := strings.TrimRight(m.URI, "/")
		if uri != origin && uri != origin+"/" {
			if !strings.HasPrefix(m.URI, origin+"/") && uri != origin {
				return ErrInvalidArgument
			}
		}
	}
	return nil
}

func validEthAddress(addr string) bool {
	if len(addr) != 42 || !strings.HasPrefix(strings.ToLower(addr), "0x") {
		return false
	}
	for _, c := range addr[2:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func originFromCtx(ctx context.Context) (host, origin string) {
	if ctx == nil {
		return "", ""
	}
	tr, ok := transport.FromServerContext(ctx)
	if !ok {
		return "", ""
	}
	ht, ok := tr.(khttp.Transporter)
	if !ok {
		return "", ""
	}
	r := ht.Request()
	host = r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = fwd
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	origin = scheme + "://" + host
	return host, origin
}

func hostMatches(siweDomain, host string) bool {
	if strings.EqualFold(siweDomain, host) {
		return true
	}
	h, _, err := net.SplitHostPort(host)
	if err == nil && strings.EqualFold(siweDomain, h) {
		return true
	}
	d, _, err := net.SplitHostPort(siweDomain)
	if err == nil && strings.EqualFold(d, host) {
		return true
	}
	if err == nil {
		hh, _, herr := net.SplitHostPort(host)
		if herr == nil && strings.EqualFold(d, hh) {
			return true
		}
	}
	return false
}

func VerifySignature(message, signature string) (string, error) {
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	prefixed := []byte(prefix + message)
	hash := crypto.Keccak256Hash(prefixed)

	sigHex := strings.TrimPrefix(signature, "0x")
	sig, err := hex.DecodeString(sigHex)
	if err != nil || len(sig) != 65 {
		return "", ErrInvalidArgument
	}

	if sig[64] >= 27 {
		sig[64] -= 27
	}

	pubKey, err := crypto.SigToPub(hash.Bytes(), sig)
	if err != nil {
		return "", err
	}

	addr := crypto.PubkeyToAddress(*pubKey).Hex()
	return addr, nil
}

func GenerateToken(userID int64, wallet string, ver int64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"wallet":  wallet,
		"ver":     ver,
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(currentJWTSecret())
}

func ParseToken(tokenString string) (int64, string, int64, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return currentJWTSecret(), nil
	})
	if err != nil || token == nil || !token.Valid {
		return 0, "", 0, ErrInvalidToken
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, "", 0, ErrInvalidToken
	}
	uidRaw, ok := claims["user_id"]
	if !ok {
		return 0, "", 0, ErrInvalidToken
	}
	uidFloat, ok := uidRaw.(float64)
	if !ok {
		return 0, "", 0, ErrInvalidToken
	}
	wallet, _ := claims["wallet"].(string)
	var ver int64
	if v, ok := claims["ver"].(float64); ok {
		ver = int64(v)
	}
	return int64(uidFloat), wallet, ver, nil
}

type userIDKey struct{}
type walletKey struct{}
type tokenVerKey struct{}

func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

func UserIDFromContext(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(userIDKey{}).(int64)
	return v, ok
}

func RequireUserID(ctx context.Context) (int64, error) {
	id, ok := UserIDFromContext(ctx)
	if !ok || id <= 0 {
		return 0, ErrUnauthorized
	}
	return id, nil
}

func WithWallet(ctx context.Context, wallet string) context.Context {
	return context.WithValue(ctx, walletKey{}, wallet)
}

func WalletFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(walletKey{}).(string)
	return v, ok
}

func WithTokenVer(ctx context.Context, ver int64) context.Context {
	return context.WithValue(ctx, tokenVerKey{}, ver)
}

func TokenVerFromContext(ctx context.Context) int64 {
	v, _ := ctx.Value(tokenVerKey{}).(int64)
	return v
}
