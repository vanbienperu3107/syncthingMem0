// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package api

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	deviceauth "github.com/syncthing/syncthing/lib/auth"
	"github.com/syncthing/syncthing/lib/config"
	"github.com/syncthing/syncthing/lib/protocol"
)

const (
	minHubSecretLength = 32
	maxRegisterBody    = 1 << 10
)

var errHubSecretTooShort = errors.New("hubSecret must be at least 32 bytes")

type registerRequest struct {
	DeviceName string `json:"device_name"`
}

type registerResponse struct {
	DeviceID string `json:"device_id"`
	Token    string `json:"token"`
}

type tokenRefreshResponse struct {
	Token string `json:"token"`
}

func (s *service) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cfg := s.cfg.RawCopy()
	if cfg.RegistrationSecret != "" && r.Header.Get("X-Registration-Secret") != cfg.RegistrationSecret {
		http.Error(w, "invalid registration secret", http.StatusForbidden)
		return
	}

	var req registerRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRegisterBody)).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.DeviceName == "" {
		http.Error(w, "device_name required", http.StatusBadRequest)
		return
	}

	deviceID, err := newRegisteredDeviceID()
	if err != nil {
		http.Error(w, "failed to generate device ID", http.StatusInternalServerError)
		return
	}

	tokenService, err := tokenServiceFromConfig(cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	token, err := tokenService.Generate(deviceID.String(), req.DeviceName)
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	waiter, err := s.cfg.Modify(func(cfg *config.Configuration) {
		device := cfg.Defaults.Device.Copy()
		device.DeviceID = deviceID
		device.Name = req.DeviceName
		cfg.SetDevice(device)
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	waiter.Wait()

	sendJSON(w, registerResponse{
		DeviceID: deviceID.String(),
		Token:    token,
	})
}

func (s *service) handleTokenRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, ok := deviceauth.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	tokenService, err := tokenServiceFromConfig(s.cfg.RawCopy())
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	token, err := tokenService.Generate(claims.DeviceID, claims.DeviceName)
	if err != nil {
		http.Error(w, "failed to refresh token", http.StatusInternalServerError)
		return
	}

	sendJSON(w, tokenRefreshResponse{Token: token})
}

func (s *service) bearerAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenService, err := tokenServiceFromConfig(s.cfg.RawCopy())
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		deviceauth.Middleware(tokenService)(next).ServeHTTP(w, r)
	})
}

func tokenServiceFromConfig(cfg config.Configuration) (*deviceauth.TokenService, error) {
	if len(cfg.HubSecret) < minHubSecretLength {
		return nil, errHubSecretTooShort
	}
	ttlHours := cfg.TokenTTL
	if ttlHours <= 0 {
		ttlHours = config.DefaultTokenTTLH
	}
	return deviceauth.NewTokenService([]byte(cfg.HubSecret), time.Duration(ttlHours)*time.Hour), nil
}

func newRegisteredDeviceID() (protocol.DeviceID, error) {
	var bs [protocol.DeviceIDLength]byte
	if _, err := rand.Read(bs[:]); err != nil {
		return protocol.EmptyDeviceID, err
	}
	return protocol.DeviceIDFromBytes(bs[:])
}
