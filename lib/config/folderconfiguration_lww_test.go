// Copyright (C) 2026 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package config

import (
	"encoding/json"
	"encoding/xml"
	"testing"
)

func TestFolderConfigurationLWWReconcilerDefault(t *testing.T) {
	var cfg FolderConfiguration
	if err := json.Unmarshal([]byte(`{"id":"default"}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.UseLWWReconciler {
		t.Fatal("UseLWWReconciler should default to false")
	}
}

func TestFolderConfigurationLWWReconcilerJSON(t *testing.T) {
	var cfg FolderConfiguration
	if err := json.Unmarshal([]byte(`{"id":"default","useLWWReconciler":true}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.UseLWWReconciler {
		t.Fatal("UseLWWReconciler should be configurable through JSON")
	}
}

func TestFolderConfigurationLWWReconcilerXML(t *testing.T) {
	var cfg FolderConfiguration
	if err := xml.Unmarshal([]byte(`<folder id="default"><useLWWReconciler>true</useLWWReconciler></folder>`), &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.UseLWWReconciler {
		t.Fatal("UseLWWReconciler should be configurable through XML")
	}
}
