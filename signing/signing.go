package signing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	Header = "Bowline-Signature"
	Skew   = 300 * time.Second
)

var (
	ErrMissingSignature   = errors.New("signing: missing signature")
	ErrMalformedSignature = errors.New("signing: malformed signature")
	ErrSkew               = errors.New("signing: timestamp outside the accepted window")
	ErrUnknownKey         = errors.New("signing: unknown key id")
	ErrSignatureMismatch  = errors.New("signing: signature mismatch")
)

type SecretProvider interface {
	Secret(ctx context.Context, keyID string) ([]byte, error)
}

type StaticSecrets map[string][]byte

func (s StaticSecrets) Secret(ctx context.Context, keyID string) ([]byte, error) {
	secret, ok := s[keyID]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownKey, keyID)
	}
	return secret, nil
}

func canonical(method, pathWithQuery string, body []byte, timestamp int64) []byte {
	sum := sha256.Sum256(body)
	var b strings.Builder
	b.WriteString(strings.ToUpper(method))
	b.WriteByte('\n')
	b.WriteString(pathWithQuery)
	b.WriteByte('\n')
	b.WriteString(hex.EncodeToString(sum[:]))
	b.WriteByte('\n')
	b.WriteString(strconv.FormatInt(timestamp, 10))
	b.WriteByte('\n')
	return []byte(b.String())
}

func mac(secret, message []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write(message)
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func Sign(method, pathWithQuery string, body []byte, keyID string, secret []byte, now time.Time) string {
	timestamp := now.Unix()
	signature := mac(secret, canonical(method, pathWithQuery, body, timestamp))
	return fmt.Sprintf("v1,t=%d,kid=%s,sig=%s", timestamp, keyID, signature)
}

type parsed struct {
	timestamp int64
	keyID     string
	signature string
}

func parse(header string) (parsed, error) {
	var out parsed
	if strings.TrimSpace(header) == "" {
		return out, ErrMissingSignature
	}
	parts := strings.Split(header, ",")
	if len(parts) != 4 || strings.TrimSpace(parts[0]) != "v1" {
		return out, ErrMalformedSignature
	}
	seen := map[string]string{}
	for _, part := range parts[1:] {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || value == "" {
			return out, ErrMalformedSignature
		}
		if _, dup := seen[key]; dup {
			return out, ErrMalformedSignature
		}
		seen[key] = value
	}
	raw, ok := seen["t"]
	if !ok {
		return out, ErrMalformedSignature
	}
	timestamp, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return out, ErrMalformedSignature
	}
	keyID, ok := seen["kid"]
	if !ok {
		return out, ErrMalformedSignature
	}
	signature, ok := seen["sig"]
	if !ok {
		return out, ErrMalformedSignature
	}
	if _, err := base64.StdEncoding.DecodeString(signature); err != nil {
		return out, ErrMalformedSignature
	}
	return parsed{timestamp: timestamp, keyID: keyID, signature: signature}, nil
}

func Verify(ctx context.Context, provider SecretProvider, header, method, pathWithQuery string, body []byte, now time.Time) error {
	p, err := parse(header)
	if err != nil {
		return err
	}
	drift := now.Unix() - p.timestamp
	if drift < 0 {
		drift = -drift
	}
	if time.Duration(drift)*time.Second > Skew {
		return fmt.Errorf("%w: %d seconds", ErrSkew, drift)
	}
	if provider == nil {
		return ErrUnknownKey
	}
	secret, err := provider.Secret(ctx, p.keyID)
	if err != nil {
		return err
	}
	want := mac(secret, canonical(method, pathWithQuery, body, p.timestamp))
	if !hmac.Equal([]byte(want), []byte(p.signature)) {
		return ErrSignatureMismatch
	}
	return nil
}
