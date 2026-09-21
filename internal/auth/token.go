package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Claims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Expiry  int64  `json:"exp"`
}

func IssueToken(secret, subject, email string, ttl time.Duration) (string, error) {
	if len(secret) < 32 {
		return "", errors.New("JWT_SECRET must be at least 32 bytes")
	}
	header := encode(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload := encode(Claims{Subject: subject, Email: email, Expiry: time.Now().Add(ttl).Unix()})
	unsigned := header + "." + payload
	signature := sign(secret, unsigned)
	return unsigned + "." + signature, nil
}

func ParseToken(secret, token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(secret) < 32 {
		return Claims{}, errors.New("invalid token")
	}
	unsigned := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(sign(secret, unsigned)), []byte(parts[2])) {
		return Claims{}, errors.New("invalid token signature")
	}
	var claims Claims
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || json.Unmarshal(decoded, &claims) != nil || claims.Subject == "" || claims.Expiry <= time.Now().Unix() {
		return Claims{}, errors.New("expired or malformed token")
	}
	return claims, nil
}

func encode(value any) string {
	data, _ := json.Marshal(value)
	return base64.RawURLEncoding.EncodeToString(data)
}

func sign(secret, value string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
