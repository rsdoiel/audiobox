package audiobox

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
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

// shareState holds the current network-sharing status, protected by a RWMutex.
type shareState struct {
	mu        sync.RWMutex
	sharing   bool
	shareAddr string // e.g. "192.168.1.5"
}

// listenRequest asks the listenerManager to swap to a new bind address.
type listenRequest struct {
	addr      string       // TCP address, e.g. "127.0.0.1:8010" or "0.0.0.0:8010"
	sharing   bool         // new sharing value to record in shareState
	shareAddr string       // human-readable LAN address stored in shareState
	done      chan struct{} // closed by manager after state is updated
}

// listenerManager owns a single http.Server goroutine and can hot-swap its
// bind address by receiving listenRequests over ctrl.
type listenerManager struct {
	port    int
	handler http.Handler
	ctrl    chan listenRequest
	state   *shareState
	logger  *log.Logger
}

// run starts the initial listener and blocks until shutdownCh is closed.
// On each listenRequest the current server is cleanly shut down and a new
// one is started on the requested address before state is updated.
func (lm *listenerManager) run(initialAddr string, shutdownCh <-chan struct{}) error {
	srv := &http.Server{Addr: initialAddr, Handler: lm.handler}
	lm.logger.Printf("audiobox server listening on http://%s", initialAddr)
	go srv.ListenAndServe() //nolint:errcheck

	for {
		select {
		case req := <-lm.ctrl:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := srv.Shutdown(ctx); err != nil {
				lm.logger.Printf("listener swap shutdown: %v", err)
			}
			cancel()
			srv = &http.Server{Addr: req.addr, Handler: lm.handler}
			lm.logger.Printf("audiobox server listening on http://%s", req.addr)
			go srv.ListenAndServe() //nolint:errcheck
			lm.state.mu.Lock()
			lm.state.sharing = req.sharing
			lm.state.shareAddr = req.shareAddr
			lm.state.mu.Unlock()
			close(req.done)
		case <-shutdownCh:
			lm.logger.Printf("audiobox server shutting down")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return srv.Shutdown(ctx)
		}
	}
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

	ss := &shareState{}
	if c.cfg.ShareAddress != "" {
		ss.sharing = true
		ss.shareAddr = c.cfg.ShareAddress
	}

	lm := &listenerManager{
		port:   port,
		ctrl:   make(chan listenRequest, 1),
		state:  ss,
		logger: logger,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/status", c.handleStatus(logger))
	mux.HandleFunc("POST /api/init", c.handleInit(logger))
	mux.HandleFunc("GET /api/list/albums", c.handleListAlbums(logger))
	mux.HandleFunc("GET /api/list/artists", c.handleListArtists(logger))
	mux.HandleFunc("GET /api/list/titles", c.handleListTitles(logger))
	mux.HandleFunc("GET /api/list/folders", c.handleListFolders(logger))
	mux.HandleFunc("GET /api/list/folder-tracks", c.handleListFolderTracks(logger))
	mux.HandleFunc("GET /api/list/album-tracks", c.handleListAlbumTracks(logger))
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
	mux.HandleFunc("GET /api/share/status", handleShareStatus(ss, port))
	mux.HandleFunc("GET /api/share/addresses", handleShareAddresses())
	mux.HandleFunc("POST /api/share/on", c.handleShareOn(lm, logger))
	mux.HandleFunc("POST /api/share/off", c.handleShareOff(lm, logger))
	mux.HandleFunc("GET /api/excluded-folders", c.handleGetExcludedFolders())
	mux.HandleFunc("POST /api/excluded-folders", c.handleSetExcludedFolders(logger))
	mux.HandleFunc("GET /api/playlists", c.handleListPlaylists(logger))
	mux.HandleFunc("POST /api/playlists", c.handleSavePlaylist(logger))
	mux.HandleFunc("GET /api/playlists/{id}", c.handleLoadPlaylist(logger))
	mux.HandleFunc("DELETE /api/playlists/{id}", c.handleDeletePlaylist(logger))

	if c.cfg.Htdocs != "" {
		mux.Handle("/", http.FileServer(http.Dir(c.cfg.Htdocs)))
	} else {
		sub, err := fs.Sub(embeddedHtdocs, "htdocs")
		if err != nil {
			return fmt.Errorf("embedded htdocs: %w", err)
		}
		mux.Handle("/", http.FileServer(http.FS(sub)))
	}

	lm.handler = remoteAccessMiddleware(corsMiddleware(c.cfg.CORSOrigin, mux))

	initialAddr := fmt.Sprintf("127.0.0.1:%d", port)
	if c.cfg.ShareAddress != "" {
		initialAddr = fmt.Sprintf("0.0.0.0:%d", port)
	}
	return lm.run(initialAddr, shutdownCh)
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

// remoteAccessMiddleware enforces read-only access for non-loopback clients.
// POST, PUT, and DELETE requests from any address other than 127.0.0.1 or ::1
// are rejected with 403 so that LAN clients can browse and stream but cannot
// modify the collection or toggle sharing.
func remoteAccessMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		isLoopback := host == "127.0.0.1" || host == "::1"
		if !isLoopback {
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodDelete:
				writeJSONError(w, http.StatusForbidden, "read-only in share mode")
				return
			}
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

func parseExcludeFolders(r *http.Request) []string {
	raw := r.URL.Query().Get("exclude")
	if raw == "" {
		return nil
	}
	var out []string
	for _, f := range strings.Split(raw, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func (c *Collection) handleListAlbums(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := c.GetAlbumEntries(parseExcludeFolders(r)...)
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
		items, err := c.GetArtists(parseExcludeFolders(r)...)
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
		items, err := c.GetTitles(parseExcludeFolders(r)...)
		if err != nil {
			logger.Printf("list titles: %v", err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, items)
	}
}

func (c *Collection) handleListFolders(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		folders, err := c.GetFolders()
		if err != nil {
			logger.Printf("list folders: %v", err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, folders)
	}
}

func (c *Collection) handleListFolderTracks(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dir := r.URL.Query().Get("dir")
		if dir == "" {
			writeJSONError(w, http.StatusBadRequest, "dir parameter required")
			return
		}
		tracks, err := c.GetFolderTracks(dir)
		if err != nil {
			logger.Printf("list folder-tracks %q: %v", dir, err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, tracks)
	}
}

func (c *Collection) handleListAlbumTracks(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dir := r.URL.Query().Get("dir")
		if dir == "" {
			writeJSONError(w, http.StatusBadRequest, "dir parameter required")
			return
		}
		tracks, err := c.GetTracksByAlbumDir(dir)
		if err != nil {
			logger.Printf("list album-tracks %q: %v", dir, err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, tracks)
	}
}

func handleShareStatus(ss *shareState, port int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ss.mu.RLock()
		sharing := ss.sharing
		addr := ss.shareAddr
		ss.mu.RUnlock()
		shareURL := ""
		if sharing && addr != "" {
			shareURL = fmt.Sprintf("http://%s:%d", addr, port)
		}
		writeJSON(w, map[string]any{
			"sharing":       sharing,
			"share_address": addr,
			"share_url":     shareURL,
		})
	}
}

func handleShareAddresses() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ifaces, err := net.Interfaces()
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		var addrs []string
		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
				continue
			}
			ifAddrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, a := range ifAddrs {
				var ip net.IP
				switch v := a.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip == nil || ip.IsLoopback() {
					continue
				}
				if ip4 := ip.To4(); ip4 != nil {
					addrs = append(addrs, ip4.String())
				}
			}
		}
		if addrs == nil {
			addrs = []string{}
		}
		writeJSON(w, addrs)
	}
}

func (c *Collection) handleShareOn(lm *listenerManager, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Address string `json:"address"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Address == "" {
			writeJSONError(w, http.StatusBadRequest, "address required")
			return
		}
		if err := c.SetShareAddress(body.Address); err != nil {
			logger.Printf("share on: save config: %v", err)
		}
		pollURL := fmt.Sprintf("http://127.0.0.1:%d/api/share/status", lm.port)
		writeJSON(w, map[string]string{"status": "restarting", "poll_url": pollURL})
		go func() {
			// Small delay so the HTTP response is fully delivered before the
			// listener restarts and closes the current connection.
			time.Sleep(300 * time.Millisecond)
			lm.ctrl <- listenRequest{
				addr:      fmt.Sprintf("0.0.0.0:%d", lm.port),
				sharing:   true,
				shareAddr: body.Address,
				done:      make(chan struct{}),
			}
		}()
	}
}

func (c *Collection) handleShareOff(lm *listenerManager, logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pollURL := fmt.Sprintf("http://127.0.0.1:%d/api/share/status", lm.port)
		writeJSON(w, map[string]string{"status": "restarting", "poll_url": pollURL})
		go func() {
			time.Sleep(300 * time.Millisecond)
			lm.ctrl <- listenRequest{
				addr: fmt.Sprintf("127.0.0.1:%d", lm.port),
				done: make(chan struct{}),
			}
		}()
	}
}

func (c *Collection) handleGetExcludedFolders() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		folders := c.cfg.ExcludedFolders
		if folders == nil {
			folders = []string{}
		}
		writeJSON(w, folders)
	}
}

func (c *Collection) handleSetExcludedFolders(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Excluded []string `json:"excluded"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if err := c.SetExcludedFolders(body.Excluded); err != nil {
			logger.Printf("set excluded folders: %v", err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if body.Excluded == nil {
			body.Excluded = []string{}
		}
		writeJSON(w, body.Excluded)
	}
}

func (c *Collection) handleListPlaylists(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lists, err := c.GetPlaylists()
		if err != nil {
			logger.Printf("list playlists: %v", err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, lists)
	}
}

func (c *Collection) handleSavePlaylist(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name     string   `json:"name"`
			TrackIDs []string `json:"trackIds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			writeJSONError(w, http.StatusBadRequest, "name and trackIds required")
			return
		}
		id, err := c.SavePlaylist(body.Name, body.TrackIDs)
		if err != nil {
			logger.Printf("save playlist: %v", err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, map[string]string{"status": "created", "id": id})
	}
}

func (c *Collection) handleLoadPlaylist(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		tracks, err := c.LoadPlaylist(id)
		if err != nil {
			if strings.Contains(err.Error(), "no rows") {
				writeJSONError(w, http.StatusNotFound, "playlist not found")
				return
			}
			logger.Printf("load playlist %s: %v", id, err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, tracks)
	}
}

func (c *Collection) handleDeletePlaylist(logger *log.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := c.DeletePlaylist(id); err != nil {
			if strings.Contains(err.Error(), "no rows") {
				writeJSONError(w, http.StatusNotFound, "playlist not found")
				return
			}
			logger.Printf("delete playlist %s: %v", id, err)
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]string{"status": "deleted", "id": id})
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

		// content_url is stored relative to AudioDir; resolve to absolute for open + security check.
		absPath := filepath.Join(filepath.Clean(c.cfg.AudioDir), info.ContentURL)

		// Guard: file must be within AudioDir (catches any .. traversal in stored paths).
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
GET /api/list/albums         — album entries derived from directory structure
GET /api/list/artists        — distinct artist names
GET /api/list/titles         — distinct recording titles
GET /api/list/folders        — directories containing audio files
GET /api/list/folder-tracks  — tracks inside a folder (?dir=relative/path)
GET /api/list/album-tracks   — tracks inside an album directory (?dir=Album.Dir from /api/list/albums)
` + "```" + `

albums returns a JSON array of Album objects: {name, displayName, dir}.
artists and titles each return a JSON array of strings.
folders returns a JSON array of FolderEntry objects: {path, name, trackCount}.
folder-tracks and album-tracks each return a JSON array of AudioInfo objects, resolved by
directory rather than by tag — use these (not a field-scoped search) when the caller already
knows the exact directory, e.g. from a prior list/albums or list/folders response, so that
tag/directory-name mismatches or ambiguous tags never mix tracks from a different release.

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

## Share

` + "```" + `
GET  /api/share/status     — {sharing, share_address, share_url}
GET  /api/share/addresses  — []string of available LAN IPv4 addresses
POST /api/share/on         — body: {"address":"192.168.1.5"}; responds immediately with {status, poll_url}
POST /api/share/off        — responds immediately with {status, poll_url}
` + "```" + `

share/on and share/off restart the listener asynchronously. Poll share/status until
sharing changes. POST endpoints are blocked (403) for non-loopback clients.

## Playlists

` + "```" + `
GET    /api/playlists      — []PlaylistInfo (id, name, trackCount, created)
POST   /api/playlists      — body: {"name":"…","trackIds":["uuid",…]}; returns {status, id}
GET    /api/playlists/{id} — []AudioInfo for the playlist's tracks in order
DELETE /api/playlists/{id} — remove playlist; 404 when not found
` + "```" + `

POST and DELETE are blocked (403) for non-loopback clients in share mode.

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
