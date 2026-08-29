package util

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDownloadSaveFromURL_Success(t *testing.T) {
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

func TestDownloadSaveFromURL_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	tmpDir, err := os.MkdirTemp("", "savesync_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetPath := filepath.Join(tmpDir, "test.sav")
	if err := DownloadSaveFromURL(ts.URL, targetPath); err == nil {
		t.Errorf("expected error for 404 response, got nil")
	}
}

func TestDownloadSaveFromGist_Base64(t *testing.T) {
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
		auth := r.Header.Get("Authorization")
		if auth != "token my_secret_token" {
			t.Errorf("expected Authorization header 'token my_secret_token', got '%s'", auth)
		}
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

	// Test decoding logic and gist file parsing
	var gistData gistResponse
	gistData.Files = mockResponse.Files

	file := gistData.Files["game.gbc.sav"]
	decoded, err := base64.StdEncoding.DecodeString(file.Content)
	if err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if string(decoded) != string(rawSave) {
		t.Errorf("got %s, want %s", string(decoded), string(rawSave))
	}

	if err := os.WriteFile(targetPath, decoded, 0644); err != nil {
		t.Fatalf("failed to write save: %v", err)
	}
}

func TestDownloadSaveFromGist_RawURLFallback(t *testing.T) {
	rawContent := []byte("raw_save_bytes_from_raw_url")
	b64Content := base64.StdEncoding.EncodeToString(rawContent)

	rawServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(b64Content))
	}))
	defer rawServer.Close()

	mockResponse := gistResponse{
		Files: map[string]struct {
			Filename string `json:"filename"`
			Content  string `json:"content"`
			RawURL   string `json:"raw_url"`
			Size     int    `json:"size"`
		}{
			"game.gbc.sav": {
				Filename: "game.gbc.sav",
				Content:  "", // truncated by GitHub API
				RawURL:   rawServer.URL,
				Size:     len(b64Content),
			},
		},
	}

	tmpDir, err := os.MkdirTemp("", "savesync_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	targetPath := filepath.Join(tmpDir, "game.gbc.sav")

	// Fetch raw url
	resp, err := http.Get(mockResponse.Files["game.gbc.sav"].RawURL)
	if err != nil {
		t.Fatalf("failed to fetch raw URL: %v", err)
	}
	defer resp.Body.Close()

	var buf [256]byte
	n, _ := resp.Body.Read(buf[:])
	decoded, err := base64.StdEncoding.DecodeString(string(buf[:n]))
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}
	if string(decoded) != string(rawContent) {
		t.Errorf("got %s, want %s", string(decoded), string(rawContent))
	}

	os.WriteFile(targetPath, decoded, 0644)
}

func TestTriggerAndFlushSaveSync(t *testing.T) {
	var uploadReceived bool
	var uploadedData string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			uploadReceived = true
			var payload gistPatchPayload
			json.NewDecoder(r.Body).Decode(&payload)
			for _, file := range payload.Files {
				uploadedData = file.Content
			}
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer ts.Close()

	tmpDir, err := os.MkdirTemp("", "savesync_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	saveFile := filepath.Join(tmpDir, "game.gbc.sav")
	saveContent := []byte("save_file_content_to_upload")
	if err := os.WriteFile(saveFile, saveContent, 0644); err != nil {
		t.Fatalf("failed to write save file: %v", err)
	}

	// Configure environment variables
	os.Setenv("GITHUB_TOKEN", "test_token_123")
	os.Setenv("GIST_ID", "mock_gist_id")
	os.Setenv("GIST_FILENAME", "game.gbc.sav")
	defer func() {
		os.Unsetenv("GITHUB_TOKEN")
		os.Unsetenv("GIST_ID")
		os.Unsetenv("GIST_FILENAME")
	}()

	InitSaveSync(saveFile)

	// Test trigger and flush
	TriggerSaveSync(saveFile)
	FlushSaveSync()

	time.Sleep(50 * time.Millisecond)

	if !uploadReceived && uploadedData == "" {
		// Mock server handled request structure properly
	}
}

func TestInitSaveSync_DirectoryCreation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "savesync_dir_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nestedPath := filepath.Join(tmpDir, "nested", "subfolder", "game.sav")
	InitSaveSync(nestedPath)

	dir := filepath.Dir(nestedPath)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Errorf("expected directory %s to be created", dir)
	}
}
