package audiobox

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed htdocs
var embeddedHtdocs embed.FS

// asyncState tracks the status of an asynchronous operation (scan or sweep).
type asyncState struct {
	mu          sync.Mutex
	running     bool
	startedAt   time.Time
	completedAt time.Time
	count       int // records affected (sweep only)
	err         error
}

/** Serve starts a localhost HTTP server for the collection.
 *
 * The server binds to 127.0.0.1 on cfg.Port (default 8010).
 * /api/* endpoints expose collection operations as JSON.
 * Static files are served from cfg.Htdocs when that field is set.
 * CORS headers are added according to cfg.CORSOrigin ("" → "*", "off" → none).
 * Serve blocks until the server exits cleanly or returns an error.
 *
 * Parameters:
 *   logger (*log.Logger) — receives startup and request-error messages
 *
 * Returns:
 *   error — non-nil if the server fails to start or exits unexpectedly
 *
 * Example:
 *   logger := log.New(os.Stderr, "", log.LstdFlags)
 *   if err := col.Serve(logger); err != nil { log.Fatal(err) }
 */
func (c *Collection) Serve(logger *log.Logger) error {
	port := c.cfg.Port
	if port == 0 {
		port = 8010
	}

	scanSt := &asyncState{}
	sweepSt := &asyncState{}
	shutdownCh := make(chan struct{})

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/status", c.handleStatus(logger))
	mux.HandleFunc("POST /api/init", c.handleInit(logger))
	mux.HandleFunc("GET /api/list/albums", c.handleListAlbums(logger))
	mux.HandleFunc("GET /api/list/artists", c.handleListArtists(logger))
	mux.HandleFunc("GET /api/list/titles", c.handleListTitles(logger))
	mux.HandleFunc("GET /api/search", c.handleSearch(logger))
	mux.HandleFunc("GET /api/show/{id}", c.handleShow(logger))
	mux.HandleFunc("DELETE /api/show/{id}", c.handleDelete(logger))
	mux.HandleFunc("POST /api/scan", c.handleScan(scanSt, logger))
	mux.HandleFunc("GET /api/scan/status", c.handleAsyncStatus(scanSt))
	mux.HandleFunc("POST /api/sweep", c.handleSweep(sweepSt, logger))
	mux.HandleFunc("GET /api/sweep/status", c.handleSweepStatus(sweepSt))
	mux.HandleFunc("GET /api/audio/{id}", c.handleAudio(logger))
	mux.HandleFunc("GET /api/help", handleAPIHelp())
	mux.HandleFunc("POST /api/shutdown", handleShutdown(shutdownCh, logger))

	if c.cfg.Htdocs != "" {
		mux.Handle("/", http.FileServer(http.Dir(c.cfg.Htdocs)))
	} else {
		// Serve the embedded htdocs as the default web UI.
		sub, err := fs.Sub(embeddedHtdocs, "htdocs")
		if err != nil {
			return fmt.Errorf("embedded htdocs: %w", err)
		}
		mux.Handle("/", http.FileServer(http.FS(sub)))
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	srv := &http.Server{
		Addr:    addr,
		Handler: corsMiddleware(c.cfg.CORSOrigin, mux),
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Printf("audiobox server listening on http://%s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		} else {
			errCh <- nil
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-shutdownCh:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		logger.Printf("audiobox server shutting down")
		return srv.Shutdown(ctx)
	}
}

// corsMiddleware adds CORS headers to every response.
// origin is used as the Access-Control-Allow-Origin value.
// An empty origin defaults to "*"; "off" or "none" disables CORS headers.
func corsMiddleware(origin string, next http.Handler) http.Handler {
	if origin == "off" || origin == "none" {
		return next
	}
	if origin == "" {
		origin = "*"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// writeJSON serialises v as indented JSON.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v) //nolint:errcheck
}

// writeJSONError writes a JSON error body with the given HTTP status code.
func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}

func (c *Collection) handleStatus(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		count, err := c.Count()
		if err != nil {
			logger.Printf("status count: %v", err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]any{
			"initialized":     true,
			"version":         Version,
			"collection_name": c.cfg.Name,
			"audio_dir":       c.cfg.AudioDir,
			"track_count":     count,
		})
	}
}

func (c *Collection) handleInit(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		col, err := InitAudiobox()
		if err != nil {
			logger.Printf("init: %v", err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		col.Close()
		writeJSON(w, map[string]string{
			"status":    "initialized",
			"audio_dir": col.cfg.AudioDir,
		})
	}
}

func (c *Collection) handleListAlbums(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := c.GetAlbumEntries()
		if err != nil {
			logger.Printf("list albums: %v", err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		names := make([]string, len(items))
		for i, a := range items {
			names[i] = a.DisplayName
		}
		writeJSON(w, names)
	}
}

func (c *Collection) handleListArtists(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := c.GetArtists()
		if err != nil {
			logger.Printf("list artists: %v", err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, items)
	}
}

func (c *Collection) handleListTitles(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := c.GetTitles()
		if err != nil {
			logger.Printf("list titles: %v", err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, items)
	}
}

func (c *Collection) handleSearch(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		results, err := c.SearchAudioFiles(q)
		if err != nil {
			logger.Printf("search %q: %v", q, err)
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, results)
	}
}

func (c *Collection) handleShow(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		info, err := c.Read(id)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "record not found")
			return
		}
		writeJSON(w, info)
	}
}

func (c *Collection) handleDelete(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := c.Delete(id); err != nil {
			logger.Printf("delete %s: %v", id, err)
			writeJSONError(w, http.StatusNotFound, "record not found")
			return
		}
		writeJSON(w, map[string]string{"status": "deleted", "id": id})
	}
}

func (c *Collection) handleScan(state *asyncState, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state.mu.Lock()
		if state.running {
			state.mu.Unlock()
			writeJSONError(w, http.StatusConflict, "scan already in progress")
			return
		}
		state.running = true
		state.startedAt = time.Now()
		state.completedAt = time.Time{}
		state.err = nil
		started := state.startedAt
		state.mu.Unlock()

		go func() {
			err := c.ScanDirectories()
			state.mu.Lock()
			state.running = false
			state.completedAt = time.Now()
			state.err = err
			state.mu.Unlock()
			if err != nil {
				logger.Printf("scan error: %v", err)
			} else {
				logger.Printf("scan completed")
			}
		}()

		w.WriteHeader(http.StatusAccepted)
		writeJSON(w, map[string]any{
			"status":     "started",
			"started_at": started,
		})
	}
}

func (c *Collection) handleSweep(state *asyncState, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state.mu.Lock()
		if state.running {
			state.mu.Unlock()
			writeJSONError(w, http.StatusConflict, "sweep already in progress")
			return
		}
		state.running = true
		state.startedAt = time.Now()
		state.completedAt = time.Time{}
		state.count = 0
		state.err = nil
		started := state.startedAt
		state.mu.Unlock()

		go func() {
			n, err := c.Sweep()
			state.mu.Lock()
			state.running = false
			state.completedAt = time.Now()
			state.count = n
			state.err = err
			state.mu.Unlock()
			if err != nil {
				logger.Printf("sweep error: %v", err)
			} else {
				logger.Printf("sweep completed: %d stale record(s) removed", n)
			}
		}()

		w.WriteHeader(http.StatusAccepted)
		writeJSON(w, map[string]any{
			"status":     "started",
			"started_at": started,
		})
	}
}

// handleAsyncStatus serves a generic async operation status (used for scan).
func (c *Collection) handleAsyncStatus(state *asyncState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state.mu.Lock()
		defer state.mu.Unlock()
		writeJSON(w, buildAsyncStatusResponse(state))
	}
}

// handleSweepStatus serves sweep status including the removed-record count.
func (c *Collection) handleSweepStatus(state *asyncState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state.mu.Lock()
		defer state.mu.Unlock()
		resp := buildAsyncStatusResponse(state)
		if !state.running && !state.completedAt.IsZero() && state.err == nil {
			resp["records_removed"] = state.count
		}
		writeJSON(w, resp)
	}
}

func buildAsyncStatusResponse(state *asyncState) map[string]any {
	resp := map[string]any{}
	switch {
	case state.running:
		resp["status"] = "running"
		resp["started_at"] = state.startedAt
	case state.startedAt.IsZero():
		resp["status"] = "idle"
	case state.err != nil:
		resp["status"] = "error"
		resp["started_at"] = state.startedAt
		resp["completed_at"] = state.completedAt
		resp["error"] = state.err.Error()
	default:
		resp["status"] = "completed"
		resp["started_at"] = state.startedAt
		resp["completed_at"] = state.completedAt
	}
	return resp
}

func handleShutdown(shutdownCh chan struct{}, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "shutting down"})
		logger.Printf("shutdown requested via API")
		go func() { close(shutdownCh) }()
	}
}

func (c *Collection) handleAudio(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		info, err := c.Read(id)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "record not found")
			return
		}

		absPath, err := filepath.Abs(info.ContentURL)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "resolving path")
			return
		}

		// Guard: file must be within AudioDir.
		audioDir := filepath.Clean(c.cfg.AudioDir) + string(filepath.Separator)
		if !strings.HasPrefix(absPath+string(filepath.Separator), audioDir) {
			logger.Printf("audio: path %q outside AudioDir", absPath)
			writeJSONError(w, http.StatusForbidden, "forbidden")
			return
		}

		f, err := os.Open(absPath)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, "file not found")
			return
		}
		defer f.Close()

		fi, err := f.Stat()
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "stat failed")
			return
		}

		if info.EncodingFormat != "" {
			w.Header().Set("Content-Type", info.EncodingFormat)
		}
		http.ServeContent(w, r, filepath.Base(absPath), fi.ModTime(), f)
	}
}

// apiHelpMarkdown is the Markdown reference document returned by GET /api/help.
const apiHelpMarkdown = `# audiobox API Reference

All endpoints are relative to http://127.0.0.1:<port> (default port 8010).

## Status & Init

` + "```" + `
GET  /api/status   — collection status (initialized, version, track_count)
POST /api/init     — initialise or upgrade the ~/Audio collection
` + "```" + `

## List

` + "```" + `
GET /api/list/albums    — distinct album names
GET /api/list/artists   — distinct artist names
GET /api/list/titles    — distinct recording titles
` + "```" + `

Each returns a JSON array of strings.

## Search

` + "```" + `
GET /api/search?q=QUERY
` + "```" + `

Returns a JSON array of AudioInfo objects. Supports the same query syntax as
the CLI search action:

- Plain term: ` + "`Bach`" + `
- Regex: ` + "`/pattern/`" + `
- Field-scoped: ` + "`artist:Gould`" + `, ` + "`album:Bach`" + `
- Field regex: ` + "`artist:/Glenn Gould/`" + `

## Show / Delete

` + "```" + `
GET    /api/show/{id}   — full metadata for one record
DELETE /api/show/{id}   — remove a record from the collection
` + "```" + `

Status 404 when not found.

## Scan

` + "```" + `
POST /api/scan           — start async re-scan of audioDir
GET  /api/scan/status    — poll scan progress
` + "```" + `

POST returns HTTP 202 with ` + "`{\"status\":\"started\",\"started_at\":\"...\"}`" + `.
Returns HTTP 409 if a scan is already running.

## Sweep

` + "```" + `
POST /api/sweep          — start async sweep (remove stale records)
GET  /api/sweep/status   — poll sweep progress
` + "```" + `

Completed sweep status includes ` + "`records_removed`" + ` count.

## Audio playback

` + "```" + `
GET /api/audio/{id}
` + "```" + `

Streams the audio file for the record identified by UUID.
Supports HTTP Range requests so browser ` + "`<audio>`" + ` elements can seek.
Returns 403 if the file is outside the collection's audioDir.

## Shutdown

` + "```" + `
POST /api/shutdown
` + "```" + `

Gracefully stops the server. Returns ` + "`{\"status\":\"shutting down\"}`" + ` before exiting.

## Help

` + "```" + `
GET /api/help
` + "```" + `

Returns this document as ` + "`text/markdown`" + `.
`

func handleAPIHelp() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		fmt.Fprint(w, apiHelpMarkdown)
	}
}
