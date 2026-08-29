package static

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/HFO4/gbc-in-cloud/driver"
	"github.com/HFO4/gbc-in-cloud/gb"
	"github.com/HFO4/gbc-in-cloud/util"
	"github.com/gorilla/websocket"
)

type StaticServer struct {
	Port     int
	GamePath string

	driver   *driver.StaticImage
	core     *gb.Core
	upgrader websocket.Upgrader
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[HTTP Panic Recovered] %v on %s %s", err, r.Method, r.URL.Path)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Run Running the static-image gaming server
func (server *StaticServer) Run() {
	// startup the emulator
	server.driver = &driver.StaticImage{}
	server.upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		return true
	}}
	server.core = &gb.Core{
		FPS:           60,
		Clock:         4194304,
		Debug:         false,
		DisplayDriver: server.driver,
		Controller:    server.driver,
		SpeedMultiple: 0,
		ToggleSound:   false,
	}
	server.core.Init(server.GamePath)
	go server.core.DisplayDriver.Run(server.core.DrawSignal, func() {})
	go server.core.Run()

	mux := http.NewServeMux()

	// image and control server
	mux.HandleFunc("/image", showImage(server))
	mux.HandleFunc("/stream", streamImages(server))
	mux.HandleFunc("/svg", showSVG(server))
	mux.HandleFunc("/control", newInput(server))
	mux.HandleFunc("/save/download", downloadSave(server))
	mux.HandleFunc("/save/upload", uploadSave(server))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Printf("[Server] Starting static server on port %d...", server.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", server.Port), recoveryMiddleware(mux)); err != nil {
		log.Fatalf("[Server] Failed to listen: %v", err)
	}
}

func streamImages(server *StaticServer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		c, err := server.upgrader.Upgrade(w, req, nil)
		if err != nil {
			log.Print(":upgrade error: ", err)
			return
		}
		defer c.Close()
		go func() {
			for {
				_, msg, err2 := c.ReadMessage()
				stringMsg := string(msg)
				if err2 != nil {
					break
				}
				buttonByte, err3 := strconv.ParseUint(stringMsg, 10, 32)
				if err3 != nil {
					continue
				}
				if buttonByte > 7 {
					log.Printf("Received input (%s) > 7", stringMsg)
					continue
				}
				server.driver.EnqueueInput(byte(buttonByte))
			}
		}()

		// Throttle streaming to 30 FPS to prevent 100% CPU spikes
		ticker := time.NewTicker(time.Second / 30)
		defer ticker.Stop()

		for range ticker.C {
			img := server.driver.Render()
			buf := new(bytes.Buffer)
			err = png.Encode(buf, img)
			if err != nil {
				continue
			}
			err = c.WriteMessage(websocket.BinaryMessage, buf.Bytes())
			if err != nil {
				break
			}
		}
	}
}

func showSVG(server *StaticServer) func(http.ResponseWriter, *http.Request) {
	svg, err := os.ReadFile("gb.svg")
	if err != nil {
		log.Printf("[Warning] gb.svg not found: %v", err)
	}

	return func(w http.ResponseWriter, req *http.Request) {
		callback := req.URL.Query().Get("callback")

		w.Header().Set("Cache-control", "no-cache,max-age=0")
		w.Header().Set("Content-type", "image/svg+xml")
		w.Header().Set("Expires", time.Now().Add(time.Duration(-1)*time.Hour).UTC().Format(http.TimeFormat))

		// Encode image to Base64
		img := server.driver.Render()
		var imageBuf bytes.Buffer
		png.Encode(&imageBuf, img)
		encoded := base64.StdEncoding.EncodeToString(imageBuf.Bytes())

		// Embed image into svg template
		res := strings.ReplaceAll(string(svg), "{image}", "data:image/png;base64,"+encoded)

		// Replace callback url safely
		res = strings.ReplaceAll(res, "{callback}", callback)

		w.Write([]byte(res))
	}
}

func showImage(server *StaticServer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-control", "no-cache,max-age=0")
		w.Header().Set("Content-type", "image/png")
		w.Header().Set("Expires", time.Now().Add(time.Duration(-1)*time.Hour).UTC().Format(http.TimeFormat))
		img := server.driver.Render()
		png.Encode(w, img)
	}
}

func newInput(server *StaticServer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		keys, ok := req.URL.Query()["button"]
		callback := req.URL.Query().Get("callback")

		if !ok || len(keys) < 1 {
			if callback != "" {
				http.Redirect(w, req, callback, http.StatusSeeOther)
			} else {
				w.WriteHeader(http.StatusOK)
			}
			return
		}

		buttonByte, err := strconv.ParseUint(keys[0], 10, 32)
		if err != nil || buttonByte > 7 {
			if callback != "" {
				http.Redirect(w, req, callback, http.StatusSeeOther)
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
			return
		}

		server.driver.EnqueueInput(byte(buttonByte))
		time.Sleep(time.Duration(500) * time.Millisecond)

		if callback != "" {
			http.Redirect(w, req, callback, http.StatusSeeOther)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Input OK"))
		}
	}
}

func downloadSave(server *StaticServer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if server.core == nil || server.core.RamPath == "" {
			http.Error(w, "Save path not configured", http.StatusNotFound)
			return
		}

		// Force RAM save first
		server.core.SaveRAM()

		data, err := os.ReadFile(server.core.RamPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Save file not found: %v", err), http.StatusNotFound)
			return
		}

		filename := filepath.Base(server.core.RamPath)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Write(data)
	}
}

func uploadSave(server *StaticServer) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "Method not allowed. Use POST.", http.StatusMethodNotAllowed)
			return
		}

		if server.core == nil || server.core.RamPath == "" {
			http.Error(w, "Save path not configured", http.StatusNotFound)
			return
		}

		file, _, err := req.FormFile("save")
		var data []byte
		if err == nil {
			defer file.Close()
			data, err = io.ReadAll(file)
		} else {
			// Try reading direct request body
			data, err = io.ReadAll(req.Body)
		}

		if err != nil || len(data) == 0 {
			http.Error(w, "Failed to read uploaded save data", http.StatusBadRequest)
			return
		}

		if err := os.WriteFile(server.core.RamPath, data, 0644); err != nil {
			http.Error(w, fmt.Sprintf("Failed to write save file: %v", err), http.StatusInternalServerError)
			return
		}

		// Trigger immediate cloud sync
		util.TriggerSaveSync(server.core.RamPath)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Save uploaded successfully (%d bytes). Reload or restart container to apply to memory.", len(data))))
	}
}

