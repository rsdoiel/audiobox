package audioinfo

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// scanState tracks the status of an asynchronous scan operation.
type scanState struct {
	mu          sync.Mutex
	running     bool
	startedAt   time.Time
	completedAt time.Time
	err         error
}

/** Serve starts a localhost HTTP server for the collection.
 *
 * The server binds to 127.0.0.1 on cfg.Port (default 8010).
 * /api/* endpoints expose collection operations as JSON.
 * Static files are served from cfg.Htdocs when that field is set.
 * CORS headers are added according to cfg.CORSOrigin ("" → "*", "off" → none).
 * Serve blocks until the server exits.
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

	state := &scanState{}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/list/albums", c.handleListAlbums(logger))
	mux.HandleFunc("GET /api/list/artists", c.handleListArtists(logger))
	mux.HandleFunc("GET /api/list/titles", c.handleListTitles(logger))
	mux.HandleFunc("GET /api/search", c.handleSearch(logger))
	mux.HandleFunc("GET /api/show/{id}", c.handleShow(logger))
	mux.HandleFunc("POST /api/scan", c.handleScan(state, logger))
	mux.HandleFunc("GET /api/scan/status", c.handleScanStatus(state))
	mux.HandleFunc("GET /api/audio/{id}", c.handleAudio(logger))
	mux.HandleFunc("GET /api/help", handleAPIHelp())

	if c.cfg.Htdocs != "" {
		mux.Handle("/", http.FileServer(http.Dir(c.cfg.Htdocs)))
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	logger.Printf("audioinfo server listening on http://%s", addr)
	return http.ListenAndServe(addr, corsMiddleware(c.cfg.CORSOrigin, mux))
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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

func (c *Collection) handleListAlbums(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := c.GetAlbumEntries()
		if err != nil {
			logger.Printf("list albums: %v", err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, items)
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

func (c *Collection) handleScan(state *scanState, logger *log.Logger) http.HandlerFunc {
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

func (c *Collection) handleScanStatus(state *scanState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state.mu.Lock()
		defer state.mu.Unlock()

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
		writeJSON(w, resp)
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
const apiHelpMarkdown = `# audioinfo API Reference

All endpoints are relative to http://127.0.0.1:<port> (default port 8010).

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

## Show

` + "```" + `
GET /api/show/{id}
` + "```" + `

Returns the full AudioInfo metadata for the record identified by UUID.
Status 404 when not found.

## Scan

` + "```" + `
POST /api/scan           — start async re-scan of audioDir
GET  /api/scan/status    — poll scan progress
` + "```" + `

POST returns HTTP 202 with ` + "`{\"status\":\"started\",\"started_at\":\"...\"}`" + `.
Returns HTTP 409 if a scan is already running.

GET /api/scan/status returns:

` + "```json" + `
{
  "status": "idle" | "running" | "completed" | "error",
  "started_at": "...",
  "completed_at": "...",
  "error": "..."
}
` + "```" + `

## Audio playback

` + "```" + `
GET /api/audio/{id}
` + "```" + `

Streams the audio file for the record identified by UUID.
Supports HTTP Range requests so browser ` + "`<audio>`" + ` elements can seek.
Returns 403 if the file is outside the collection's audioDir.

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
