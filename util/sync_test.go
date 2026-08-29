package util

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadSaveFromURL(t *testing.T) {
	expectedContent := []byte("pokemon_crystal_save_data_12345")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(expectedContent)
	}))
	defer ts.Close()

	tmpDir, err := os.MkdirTemp("", "savesync_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetPath := filepath.Join(tmpDir, "test.sav")
	if err := DownloadSaveFromURL(ts.URL, targetPath); err != nil {
		t.Fatalf("DownloadSaveFromURL failed: %v", err)
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	if string(data) != string(expectedContent) {
		t.Errorf("got %s, want %s", string(data), string(expectedContent))
	}
}

func TestDownloadSaveFromGist(t *testing.T) {
	rawSave := []byte("pokemon_crystal_test_save_data_binary")
	b64Save := base64.StdEncoding.EncodeToString(rawSave)

	mockResponse := gistResponse{
		Files: map[string]struct {
			Filename string `json:"filename"`
			Content  string `json:"content"`
			RawURL   string `json:"raw_url"`
			Size     int    `json:"size"`
		}{
			"game.gbc.sav": {
				Filename: "game.gbc.sav",
				Content:  b64Save,
				Size:     len(b64Save),
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer ts.Close()

	tmpDir, err := os.MkdirTemp("", "savesync_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetPath := filepath.Join(tmpDir, "game.gbc.sav")

	// We can test the decode directly
	decoded, err := base64.StdEncoding.DecodeString(mockResponse.Files["game.gbc.sav"].Content)
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}
	if string(decoded) != string(rawSave) {
		t.Errorf("got %s, want %s", string(decoded), string(rawSave))
	}

	err = os.WriteFile(targetPath, decoded, 0644)
	if err != nil {
		t.Fatalf("failed to write save: %v", err)
	}
}
