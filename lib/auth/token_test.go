// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package auth

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateAndVerify(t *testing.T) {
	ts := NewTokenService([]byte("test-secret-32-bytes-long!!!!!!"), 24*time.Hour)

	token, err := ts.Generate("device-123", "My Laptop")
	if err != nil {
		t.Fatal(err)
	}

	claims, err := ts.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.DeviceID != "device-123" {
		t.Errorf("got device_id=%q, want %q", claims.DeviceID, "device-123")
	}
	if claims.DeviceName != "My Laptop" {
		t.Errorf("got device_name=%q, want %q", claims.DeviceName, "My Laptop")
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	ts := NewTokenService([]byte("test-secret-32-bytes-long!!!!!!"), -time.Second)
	token, err := ts.Generate("device-123", "test")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ts.Verify(token); err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestWrongSecretRejected(t *testing.T) {
	ts1 := NewTokenService([]byte("secret-aaaaaaaaaaaaaaaaaaaaaaaa"), 24*time.Hour)
	ts2 := NewTokenService([]byte("secret-bbbbbbbbbbbbbbbbbbbbbbbb"), 24*time.Hour)

	token, err := ts1.Generate("device-123", "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ts2.Verify(token); err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestTamperedTokenRejected(t *testing.T) {
	ts := NewTokenService([]byte("test-secret-32-bytes-long!!!!!!"), 24*time.Hour)
	token, err := ts.Generate("device-123", "test")
	if err != nil {
		t.Fatal(err)
	}
	tampered := token[:len(token)/2] + "X" + token[len(token)/2+1:]

	if _, err := ts.Verify(tampered); err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestUnexpectedSigningMethodRejected(t *testing.T) {
	ts := NewTokenService([]byte("test-secret-32-bytes-long!!!!!!"), 24*time.Hour)
	token, err := ts.Generate("device-123", "test")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	parts[0] = "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0"

	if _, err := ts.Verify(strings.Join(parts, ".")); err == nil {
		t.Fatal("expected error for unexpected signing method")
	}
}

func TestRefresh(t *testing.T) {
	ts := NewTokenService([]byte("test-secret-32-bytes-long!!!!!!"), 24*time.Hour)
	token1, err := ts.Generate("device-123", "test")
	if err != nil {
		t.Fatal(err)
	}
	token2, err := ts.Refresh(token1)
	if err != nil {
		t.Fatal(err)
	}
	if token1 == token2 {
		t.Fatal("refreshed token should differ")
	}
	claims, err := ts.Verify(token2)
	if err != nil {
		t.Fatal(err)
	}
	if claims.DeviceID != "device-123" {
		t.Fatal("device_id should be preserved after refresh")
	}
}
