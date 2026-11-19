package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// JWTClaims holds user identity data embedded in the token payload
type JWTClaims struct {
	UserID string `json:"user_id,omitempty"`
	NIM    string `json:"nim"`
	Email  string `json:"email,omitempty"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Exp    int64  `json:"exp"`
	Iat    int64  `json:"iat"`
}

// GenerateToken creates an HMAC-SHA256 signed JWT token string
func GenerateToken(secret string, claims JWTClaims, ttl time.Duration) (string, error) {
	if secret == "" {
		return "", errors.New("kunci rahasia JWT tidak boleh kosong")
	}

	now := time.Now()
	claims.Iat = now.Unix()
	claims.Exp = now.Add(ttl).Unix()

	headerJSON, _ := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	headerEnc := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadEnc := base64.RawURLEncoding.EncodeToString(payloadJSON)

	signedData := headerEnc + "." + payloadEnc
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signedData))
	sigEnc := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return signedData + "." + sigEnc, nil
}

// ValidateToken verifies token signature and expiration, returning parsed claims
func ValidateToken(secret string, tokenStr string) (*JWTClaims, error) {
	if secret == "" {
		return nil, errors.New("kunci rahasia JWT tidak boleh kosong")
	}

	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("format token tidak valid")
	}

	signedData := parts[0] + "." + parts[1]
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(signedData))
	expectedSig := h.Sum(nil)

	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, expectedSig) {
		return nil, errors.New("tanda tangan token tidak valid")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("payload token tidak valid")
	}

	var claims JWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, errors.New("gagal membaca data klaim token")
	}

	if time.Now().Unix() > claims.Exp {
		return nil, errors.New("token telah kedaluwarsa")
	}

	return &claims, nil
}
