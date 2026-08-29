package util

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	syncMutex       sync.Mutex
	syncPending     bool
	syncTimer       *time.Timer
	currentSavePath string

	githubToken  string
	gistID       string
	gistFilename string
	saveURL      string
)

type gistResponse struct {
	Files map[string]struct {
		Filename string `json:"filename"`
		Content  string `json:"content"`
		RawURL   string `json:"raw_url"`
		Size     int    `json:"size"`
	} `json:"files"`
}

type gistPatchPayload struct {
	Description string                     `json:"description"`
	Files       map[string]gistFilePayload `json:"files"`
}

type gistFilePayload struct {
	Content string `json:"content"`
}

// InitSaveSync configures cloud save sync parameters from environment variables
// and downloads the save file if not already present or if Gist is configured.
func InitSaveSync(savePath string) {
	currentSavePath = savePath
	githubToken = os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		githubToken = os.Getenv("GIST_TOKEN")
	}
	gistID = os.Getenv("GIST_ID")
	gistFilename = os.Getenv("GIST_FILENAME")
	if gistFilename == "" {
		gistFilename = filepath.Base(savePath)
	}
	saveURL = os.Getenv("SAVE_URL")

	log.Printf("[SaveSync] Initializing save sync for %s...", savePath)

	// Ensure directory exists
	saveDir := filepath.Dir(savePath)
	if saveDir != "" && saveDir != "." {
		if err := os.MkdirAll(saveDir, 0755); err != nil {
			log.Printf("[SaveSync] Failed to create directory %s: %v", saveDir, err)
		}
	}

	// Try loading from Gist first if configured
	if gistID != "" {
		log.Printf("[SaveSync] Gist ID provided (%s). Fetching save from GitHub Gist...", gistID)
		if err := DownloadSaveFromGist(gistID, githubToken, gistFilename, savePath); err != nil {
			log.Printf("[SaveSync] Warning: Failed to download save from Gist: %v", err)
		} else {
			log.Printf("[SaveSync] Successfully restored save from GitHub Gist!")
			return
		}
	}

	// If no Gist or Gist failed, try SAVE_URL if provided and file doesn't exist yet
	if saveURL != "" {
		if _, err := os.Stat(savePath); os.IsNotExist(err) {
			log.Printf("[SaveSync] SAVE_URL provided (%s). Downloading initial save...", saveURL)
			if err := DownloadSaveFromURL(saveURL, savePath); err != nil {
				log.Printf("[SaveSync] Warning: Failed to download save from SAVE_URL: %v", err)
			} else {
				log.Printf("[SaveSync] Successfully downloaded initial save from SAVE_URL!")
			}
		}
	}
}

// DownloadSaveFromGist downloads the save file from GitHub Gist.
func DownloadSaveFromGist(gist, token, filename, targetPath string) error {
	reqURL := fmt.Sprintf("https://api.github.com/gists/%s", gist)
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "GameBoyLive-SaveSync")
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github API returned status %d: %s", resp.StatusCode, string(body))
	}

	var gistData gistResponse
	if err := json.NewDecoder(resp.Body).Decode(&gistData); err != nil {
		return fmt.Errorf("failed to decode gist JSON: %w", err)
	}

	var fileContent string
	var rawURL string

	// Look for the specific filename or any .sav file
	if file, ok := gistData.Files[filename]; ok {
		fileContent = file.Content
		rawURL = file.RawURL
	} else {
		for name, file := range gistData.Files {
			if filepath.Ext(name) == ".sav" || len(gistData.Files) == 1 {
				fileContent = file.Content
				rawURL = file.RawURL
				log.Printf("[SaveSync] Using Gist file: %s", name)
				break
			}
		}
	}

	var saveBytes []byte

	if fileContent != "" {
		// Content might be Base64-encoded
		decoded, err := base64.StdEncoding.DecodeString(fileContent)
		if err == nil && len(decoded) > 0 {
			saveBytes = decoded
		} else {
			saveBytes = []byte(fileContent)
		}
	} else if rawURL != "" {
		// If content is truncated by GitHub (for large files), fetch raw URL
		rawReq, err := http.NewRequest("GET", rawURL, nil)
		if err == nil {
			if token != "" {
				rawReq.Header.Set("Authorization", "token "+token)
			}
			rawResp, err := client.Do(rawReq)
			if err == nil && rawResp.StatusCode == http.StatusOK {
				defer rawResp.Body.Close()
				rawBody, _ := io.ReadAll(rawResp.Body)
				decoded, err := base64.StdEncoding.DecodeString(string(rawBody))
				if err == nil && len(decoded) > 0 {
					saveBytes = decoded
				} else {
					saveBytes = rawBody
				}
			}
		}
	}

	if len(saveBytes) == 0 {
		return fmt.Errorf("no save content found in Gist %s for file %s", gist, filename)
	}

	if len(saveBytes) <= 4 && bytes.Equal(saveBytes, make([]byte, len(saveBytes))) {
		return fmt.Errorf("gist content in %s for file %s is an empty placeholder (%d bytes), skipping restore", gist, filename, len(saveBytes))
	}

	return os.WriteFile(targetPath, saveBytes, 0644)
}

// DownloadSaveFromURL downloads raw save bytes from a direct URL.
func DownloadSaveFromURL(url, targetPath string) error {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return os.WriteFile(targetPath, data, 0644)
}

// TriggerSaveSync queues a debounced upload of the save file to GitHub Gist.
func TriggerSaveSync(savePath string) {
	if gistID == "" || githubToken == "" {
		return
	}

	syncMutex.Lock()
	defer syncMutex.Unlock()

	currentSavePath = savePath
	syncPending = true

	// Debounce: wait 5 seconds after the latest save before uploading to avoid API rate limits
	if syncTimer != nil {
		syncTimer.Stop()
	}

	syncTimer = time.AfterFunc(5*time.Second, func() {
		performGistUpload()
	})
}

// FlushSaveSync forces an immediate sync if any is pending.
func FlushSaveSync() {
	if gistID == "" || githubToken == "" {
		return
	}

	syncMutex.Lock()
	if !syncPending {
		syncMutex.Unlock()
		return
	}
	syncMutex.Unlock()

	performGistUpload()
}

func performGistUpload() {
	syncMutex.Lock()
	if !syncPending || currentSavePath == "" {
		syncMutex.Unlock()
		return
	}
	syncPending = false
	savePath := currentSavePath
	syncMutex.Unlock()

	data, err := os.ReadFile(savePath)
	if err != nil {
		log.Printf("[SaveSync] Failed to read save file for upload: %v", err)
		return
	}

	b64Content := base64.StdEncoding.EncodeToString(data)

	filename := gistFilename
	if filename == "" {
		filename = filepath.Base(savePath)
	}

	payload := gistPatchPayload{
		Description: fmt.Sprintf("GameBoyLive Save Backup (%s)", time.Now().UTC().Format(time.RFC3339)),
		Files: map[string]gistFilePayload{
			filename: {
				Content: b64Content,
			},
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[SaveSync] Failed to marshal Gist payload: %v", err)
		return
	}

	reqURL := fmt.Sprintf("https://api.github.com/gists/%s", gistID)
	req, err := http.NewRequest("PATCH", reqURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		log.Printf("[SaveSync] Failed to create HTTP PATCH request: %v", err)
		return
	}

	req.Header.Set("User-Agent", "GameBoyLive-SaveSync")
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token "+githubToken)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[SaveSync] Failed to upload save to Gist: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[SaveSync] Gist upload returned non-200 status %d: %s", resp.StatusCode, string(body))
	} else {
		log.Printf("[SaveSync] Save successfully backed up to GitHub Gist (%d bytes) at %s", len(data), time.Now().Format("15:04:05"))
	}
}
