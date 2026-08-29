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

func TestShowImage(t *testing.T) {
	server := &StaticServer{
		driver: &driver.StaticImage{},
	}
	server.driver.Init(&[160][144][3]uint8{}, "")

	handler := showImage(server)
	req := httptest.NewRequest("GET", "/image", nil)
	rr := httptest.NewRecorder()

	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("showImage status = %d; want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %s; want image/png", ct)
	}
}

func TestShowSVGWithAndWithoutCallback(t *testing.T) {
	svgContent := []byte(`<svg><a href="{callback}"><image href="{image}"/></a></svg>`)
	_ = os.WriteFile("gb.svg", svgContent, 0644)
	defer os.Remove("gb.svg")

	server := &StaticServer{
		driver: &driver.StaticImage{},
	}
	server.driver.Init(&[160][144][3]uint8{}, "")

	handler := showSVG(server)

	// Without callback
	req1 := httptest.NewRequest("GET", "/svg", nil)
	rr1 := httptest.NewRecorder()
	handler(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Errorf("showSVG without callback status = %d; want 200", rr1.Code)
	}

	// With callback URL
	req2 := httptest.NewRequest("GET", "/svg?callback=https://github.com/myuser", nil)
	rr2 := httptest.NewRecorder()
	handler(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Errorf("showSVG with callback status = %d; want 200", rr2.Code)
	}
	if !bytes.Contains(rr2.Body.Bytes(), []byte("https://github.com/myuser")) {
		t.Errorf("SVG body does not contain replaced callback URL")
	}
}

func TestNewInput_Routes(t *testing.T) {
	status := byte(0xFF)
	drv := &driver.StaticImage{}
	drv.InitStatus(&status)

	server := &StaticServer{
		driver: drv,
	}

	handler := newInput(server)

	// Valid input without callback -> Status 200
	req1 := httptest.NewRequest("GET", "/control?button=4", nil)
	rr1 := httptest.NewRecorder()
	handler(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Errorf("valid button status = %d; want 200", rr1.Code)
	}

	// Valid input with callback -> Status 303 Redirect to callback
	req2 := httptest.NewRequest("GET", "/control?button=7&callback=https://github.com", nil)
	rr2 := httptest.NewRecorder()
	handler(rr2, req2)
	if rr2.Code != http.StatusSeeOther {
		t.Errorf("callback redirect status = %d; want 303 (See Other)", rr2.Code)
	}
	if loc := rr2.Header().Get("Location"); loc != "https://github.com" {
		t.Errorf("Redirect Location = %s; want https://github.com", loc)
	}

	// Invalid button (> 7) without callback -> Status 400 Bad Request
	req3 := httptest.NewRequest("GET", "/control?button=99", nil)
	rr3 := httptest.NewRecorder()
	handler(rr3, req3)
	if rr3.Code != http.StatusBadRequest {
		t.Errorf("invalid button status = %d; want 400", rr3.Code)
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

	// Test upload via multipart form
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

	// Test upload via raw body
	rawTestData := []byte("raw_direct_upload_save_bytes_4444")
	reqRaw := httptest.NewRequest("POST", "/save/upload", bytes.NewReader(rawTestData))
	rrRaw := httptest.NewRecorder()
	upHandler(rrRaw, reqRaw)

	if rrRaw.Code != http.StatusOK {
		t.Errorf("raw upload expected status 200, got %d", rrRaw.Code)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("simulated critical crash")
	})

	protectedHandler := recoveryMiddleware(panicHandler)

	req := httptest.NewRequest("GET", "/test-panic", nil)
	rr := httptest.NewRecorder()

	// Should not crash the process, should return 500
	protectedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("recoveryMiddleware status = %d; want 500", rr.Code)
	}
}
