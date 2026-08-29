package static

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/HFO4/gbc-in-cloud/driver"
	"github.com/HFO4/gbc-in-cloud/gb"
)

func TestShowSVGWithoutCallback(t *testing.T) {
	// Create dummy gb.svg if not present
	svgContent := []byte(`<svg><a href="{callback}"><image href="{image}"/></a></svg>`)
	_ = os.WriteFile("gb.svg", svgContent, 0644)
	defer os.Remove("gb.svg")

	server := &StaticServer{
		driver: &driver.StaticImage{},
	}
	server.driver.Init(&[160][144][3]uint8{}, "")

	handler := showSVG(server)

	req := httptest.NewRequest("GET", "/svg", nil)
	rr := httptest.NewRecorder()

	// Must not panic
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestSaveDownloadAndUpload(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "static_save_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	saveFile := filepath.Join(tmpDir, "game.gbc.sav")
	testData := []byte("pokemon_crystal_save_bytes_12345678")
	if err := os.WriteFile(saveFile, testData, 0644); err != nil {
		t.Fatalf("failed to write test save: %v", err)
	}

	server := &StaticServer{
		core: &gb.Core{
			RamPath: saveFile,
		},
	}

	// Test download
	downHandler := downloadSave(server)
	reqDown := httptest.NewRequest("GET", "/save/download", nil)
	rrDown := httptest.NewRecorder()
	downHandler(rrDown, reqDown)

	if rrDown.Code != http.StatusOK {
		t.Errorf("download expected status 200, got %d", rrDown.Code)
	}
	if !bytes.Equal(rrDown.Body.Bytes(), testData) {
		t.Errorf("downloaded data mismatch: got %v, want %v", rrDown.Body.Bytes(), testData)
	}

	// Test upload
	upHandler := uploadSave(server)
	newTestData := []byte("new_pokemon_crystal_save_bytes_99999")

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("save", "game.gbc.sav")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	part.Write(newTestData)
	writer.Close()

	reqUp := httptest.NewRequest("POST", "/save/upload", &body)
	reqUp.Header.Set("Content-Type", writer.FormDataContentType())
	rrUp := httptest.NewRecorder()
	upHandler(rrUp, reqUp)

	if rrUp.Code != http.StatusOK {
		t.Errorf("upload expected status 200, got %d", rrUp.Code)
	}

	savedData, err := os.ReadFile(saveFile)
	if err != nil {
		t.Fatalf("failed to read updated save file: %v", err)
	}
	if !bytes.Equal(savedData, newTestData) {
		t.Errorf("uploaded file content mismatch: got %s, want %s", string(savedData), string(newTestData))
	}
}
