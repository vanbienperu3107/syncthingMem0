// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const issuer = "syncthing-hub"

var (
	errMalformedToken = errors.New("malformed token")
	errInvalidToken   = errors.New("invalid token")
)

type Claims struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Issuer     string `json:"iss"`
	IssuedAt   int64  `json:"iat"`
	ExpiresAt  int64  `json:"exp"`
	ID         string `json:"jti"`
}

type TokenService struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenService(secret []byte, ttl time.Duration) *TokenService {
	return &TokenService{
		secret: append([]byte(nil), secret...),
		ttl:    ttl,
	}
}

func (s *TokenService) Generate(deviceID, deviceName string) (string, error) {
	now := time.Now()
	tokenID, err := randomTokenID()
	if err != nil {
		return "", err
	}

	claims := Claims{
		DeviceID:   deviceID,
		DeviceName: deviceName,
		Issuer:     issuer,
		IssuedAt:   now.Unix(),
		ExpiresAt:  now.Add(s.ttl).Unix(),
		ID:         tokenID,
	}

	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	return signingInput + "." + s.sign(signingInput), nil
}

func (s *TokenService) Verify(tokenString string) (*Claims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, errMalformedToken
	}

	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: header", errMalformedToken)
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("%w: header", errMalformedToken)
	}
	if header.Algorithm != "HS256" {
		return nil, fmt.Errorf("%w: unexpected signing method %q", errInvalidToken, header.Algorithm)
	}

	signingInput := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(s.sign(signingInput))) {
		return nil, fmt.Errorf("%w: signature", errInvalidToken)
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: claims", errMalformedToken)
	}
	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("%w: claims", errMalformedToken)
	}
	if claims.Issuer != issuer || claims.DeviceID == "" || claims.ExpiresAt == 0 {
		return nil, fmt.Errorf("%w: claims", errInvalidToken)
	}
	if time.Now().Unix() >= claims.ExpiresAt {
		return nil, fmt.Errorf("%w: expired", errInvalidToken)
	}
	return &claims, nil
}

func (s *TokenService) Refresh(tokenString string) (string, error) {
	claims, err := s.Verify(tokenString)
	if err != nil {
		return "", err
	}
	return s.Generate(claims.DeviceID, claims.DeviceName)
}

func (s *TokenService) sign(signingInput string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(signingInput))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func randomTokenID() (string, error) {
	var bs [16]byte
	if _, err := rand.Read(bs[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bs[:]), nil
}
