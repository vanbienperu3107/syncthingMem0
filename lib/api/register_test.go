// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/config/mocks"
	"github.com/syncthing/syncthing/lib/protocol"
)

const testHubSecret = "0123456789abcdef0123456789abcdef"

func TestRegisterRequiresRegistrationSecret(t *testing.T) {
	svc := newRegisterTestService(config.Configuration{
		HubSecret:          testHubSecret,
		RegistrationSecret: "registration-secret",
		TokenTTL:           config.DefaultTokenTTLH,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(`{"device_name":"Laptop"}`))
	rr := httptest.NewRecorder()
	svc.handleRegister(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestRegisterDisabledWhenNoRegistrationSecret(t *testing.T) {
	// With no registration secret configured, registration must fail closed:
	// an empty secret must never be treated as "open to anyone".
	svc := newRegisterTestService(config.Configuration{
		HubSecret: testHubSecret,
		TokenTTL:  config.DefaultTokenTTLH,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(`{"device_name":"Laptop"}`))
	rr := httptest.NewRecorder()
	svc.handleRegister(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestRegisterReturnsTokenAndStoresDevice(t *testing.T) {
	cfg := config.Configuration{
		HubSecret:          testHubSecret,
		RegistrationSecret: "registration-secret",
		TokenTTL:           config.DefaultTokenTTLH,
	}
	svc := newRegisterTestService(cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(`{"device_name":"Laptop"}`))
	req.Header.Set("X-Registration-Secret", "registration-secret")
	rr := httptest.NewRecorder()
	svc.handleRegister(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp registerResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.DeviceID == "" || resp.Token == "" {
		t.Fatalf("expected device ID and token, got %#v", resp)
	}
	if _, err := protocol.DeviceIDFromString(resp.DeviceID); err != nil {
		t.Fatal(err)
	}

	tokenService, err := tokenServiceFromConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := tokenService.Verify(resp.Token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.DeviceID != resp.DeviceID || claims.DeviceName != "Laptop" {
		t.Fatalf("unexpected claims: %#v", claims)
	}

	modified := false
	modifyFn := svc.cfg.(*mocks.Wrapper).ModifyArgsForCall(0)
	modifyFn(&cfg)
	for _, device := range cfg.Devices {
		if device.DeviceID.String() == resp.DeviceID && device.Name == "Laptop" {
			modified = true
			break
		}
	}
	if !modified {
		t.Fatal("registered device was not added to config")
	}
}

func TestTokenRefreshRequiresValidBearer(t *testing.T) {
	cfg := config.Configuration{
		HubSecret: testHubSecret,
		TokenTTL:  config.DefaultTokenTTLH,
	}
	svc := newRegisterTestService(cfg)
	tokenService, err := tokenServiceFromConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	token, err := tokenService.Generate("device-1", "Laptop")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/token/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	svc.bearerAuthMiddleware(http.HandlerFunc(svc.handleTokenRefresh)).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp tokenRefreshResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	claims, err := tokenService.Verify(resp.Token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.DeviceID != "device-1" || claims.DeviceName != "Laptop" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestTokenRefreshRejectsInvalidBearer(t *testing.T) {
	cfg := config.Configuration{
		HubSecret: testHubSecret,
		TokenTTL:  config.DefaultTokenTTLH,
	}
	svc := newRegisterTestService(cfg)

	req := httptest.NewRequest(http.MethodPost, "/api/token/refresh", nil)
	req.Header.Set("Authorization", "Bearer invalid.token")
	rr := httptest.NewRecorder()
	svc.bearerAuthMiddleware(http.HandlerFunc(svc.handleTokenRefresh)).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestRegisterRejectsShortHubSecret(t *testing.T) {
	svc := newRegisterTestService(config.Configuration{
		HubSecret:          "short",
		RegistrationSecret: "registration-secret",
		TokenTTL:           config.DefaultTokenTTLH,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(`{"device_name":"Laptop"}`))
	req.Header.Set("X-Registration-Secret", "registration-secret")
	rr := httptest.NewRecorder()
	svc.handleRegister(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("got status %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

func newRegisterTestService(cfg config.Configuration) *service {
	m := newMockedConfig()
	m.RawCopyReturns(cfg)
	m.ModifyReturns(noopWaiter{}, nil)
	return &service{cfg: m}
}
