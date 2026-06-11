// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMiddlewareValidToken(t *testing.T) {
	ts := NewTokenService([]byte("test-secret-32-bytes-long!!!!!!"), 24*time.Hour)
	token, err := ts.Generate("dev-1", "test")
	if err != nil {
		t.Fatal(err)
	}

	handler := Middleware(ts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || claims.DeviceID != "dev-1" {
			t.Error("expected claims in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestMiddlewareNoHeader(t *testing.T) {
	ts := NewTokenService([]byte("secret"), 24*time.Hour)
	handler := Middleware(ts)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareInvalidToken(t *testing.T) {
	ts := NewTokenService([]byte("test-secret-32-bytes-long!!!!!!"), 24*time.Hour)
	handler := Middleware(ts)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid.token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestBearerToken(t *testing.T) {
	token, ok := BearerToken("bearer abc.def")
	if !ok || token != "abc.def" {
		t.Fatalf("got token=%q ok=%v", token, ok)
	}
}
