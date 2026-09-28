package audiobox

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// podcastTestStates holds the asyncState values a test may want to inspect
// directly (e.g. to poll running/completed) alongside the httptest.Server
// newPodcastTestServer returns.
type podcastTestStates struct {
	sync  *asyncState
	sweep *asyncState
}

// newPodcastTestServer wires up just the podcast HTTP endpoints (the same
// registrations Serve makes) around col, using client for outbound feed and
// download requests.
func newPodcastTestServer(t *testing.T, col *Collection, client *http.Client) (*httptest.Server, podcastTestStates) {
	t.Helper()
	logger := log.New(io.Discard, "", 0)
	states := podcastTestStates{sync: &asyncState{}, sweep: &asyncState{}}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/podcasts/subscriptions", col.handlePodcastSubscriptions(logger))
	mux.HandleFunc("POST /api/podcasts/subscriptions", col.handlePodcastAddSubscription(logger))
	mux.HandleFunc("POST /api/podcasts/sync", col.handlePodcastSync(states.sync, client, logger))
	mux.HandleFunc("GET /api/podcasts/sync/status", col.handleAsyncStatus(states.sync))
	mux.HandleFunc("GET /api/podcasts/shows", col.handlePodcastShows(logger))
	mux.HandleFunc("GET /api/podcasts/shows/{label}/episodes", col.handlePodcastShowEpisodes(logger))
	mux.HandleFunc("POST /api/podcasts/episodes/{id}/download", col.handlePodcastEpisodeDownload(client, logger))
	mux.HandleFunc("GET /api/podcasts/episodes/{id}/audio", col.handlePodcastEpisodeAudio(logger))
	mux.HandleFunc("POST /api/podcasts/episodes/{id}/listened", col.handlePodcastEpisodeListened(logger))
	mux.HandleFunc("POST /api/podcasts/episodes/{id}/unlistened", col.handlePodcastEpisodeUnlistened(logger))
	mux.HandleFunc("POST /api/podcasts/episodes/{id}/keep", col.handlePodcastEpisodeKeep(logger))
	mux.HandleFunc("DELETE /api/podcasts/episodes/{id}", col.handlePodcastEpisodeDelete(logger))
	mux.HandleFunc("POST /api/podcasts/episodes/{id}/migrate", col.handlePodcastEpisodeMigrate(logger))
	mux.HandleFunc("POST /api/podcasts/sweep", col.handlePodcastSweep(states.sweep, logger))
	mux.HandleFunc("GET /api/podcasts/sweep/status", col.handleAsyncStatus(states.sweep))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, states
}

func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decoding JSON response: %v", err)
	}
	return v
}

func TestHandlePodcastSyncEndpoint(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	feedSrv, _ := newTestFeedServer(t)
	defer feedSrv.Close()

	dir := filepath.Join(col.cfg.AudioDir, "Podcasts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	subContent := fmt.Sprintf("- [Test Show](%s)\n", feedSrv.URL)
	if err := os.WriteFile(filepath.Join(dir, "subscriptions.md"), []byte(subContent), 0644); err != nil {
		t.Fatalf("write subscriptions.md: %v", err)
	}

	apiSrv, _ := newPodcastTestServer(t, col, http.DefaultClient)

	resp, err := http.Post(apiSrv.URL+"/api/podcasts/sync", "", nil)
	if err != nil {
		t.Fatalf("POST sync: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	started := decodeJSON[map[string]any](t, resp)
	if started["status"] != "started" {
		t.Errorf("expected status 'started', got %+v", started)
	}

	var final map[string]any
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		statusResp, err := http.Get(apiSrv.URL + "/api/podcasts/sync/status")
		if err != nil {
			t.Fatalf("GET sync/status: %v", err)
		}
		body := decodeJSON[map[string]any](t, statusResp)
		if body["status"] != "running" {
			final = body
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if final == nil {
		t.Fatal("sync did not complete within the deadline")
	}
	if final["status"] != "completed" {
		t.Fatalf("expected status 'completed', got %+v", final)
	}
	if n, ok := final["new_episodes"].(float64); !ok || n != 2 {
		t.Errorf("expected new_episodes=2, got %+v", final["new_episodes"])
	}
	if n, ok := final["feeds_checked"].(float64); !ok || n != 1 {
		t.Errorf("expected feeds_checked=1, got %+v", final["feeds_checked"])
	}
}

func TestHandlePodcastSyncEndpointConflict(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	// A feed server that blocks until released, so the test controls
	// exactly how long the sync stays "running".
	release := make(chan struct{})
	feedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, testFeedTemplate)
	}))
	defer feedSrv.Close()

	dir := filepath.Join(col.cfg.AudioDir, "Podcasts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	subContent := fmt.Sprintf("- [Test Show](%s)\n", feedSrv.URL)
	if err := os.WriteFile(filepath.Join(dir, "subscriptions.md"), []byte(subContent), 0644); err != nil {
		t.Fatalf("write subscriptions.md: %v", err)
	}

	apiSrv, states := newPodcastTestServer(t, col, http.DefaultClient)

	first, err := http.Post(apiSrv.URL+"/api/podcasts/sync", "", nil)
	if err != nil {
		t.Fatalf("POST sync (first): %v", err)
	}
	first.Body.Close()
	if first.StatusCode != http.StatusAccepted {
		t.Fatalf("expected first sync to return 202, got %d", first.StatusCode)
	}

	second, err := http.Post(apiSrv.URL+"/api/podcasts/sync", "", nil)
	if err != nil {
		t.Fatalf("POST sync (second): %v", err)
	}
	second.Body.Close()
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("expected second sync to return 409 while the first is running, got %d", second.StatusCode)
	}

	// Let the blocked first sync finish (and wait for it to) before
	// feedSrv/col are torn down by the deferred cleanups below — otherwise
	// feedSrv.Close() would deadlock waiting for the in-flight connection
	// that only unblocks once release is closed.
	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		states.sync.mu.Lock()
		running := states.sync.running
		states.sync.mu.Unlock()
		if !running {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("first sync did not finish after releasing the blocked feed request")
}

func TestHandlePodcastShowsAndEpisodesEndpoints(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	feedSrv, _ := newTestFeedServer(t)
	defer feedSrv.Close()

	dir := filepath.Join(col.cfg.AudioDir, "Podcasts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	subContent := fmt.Sprintf("- [Test Show](%s)\n", feedSrv.URL)
	if err := os.WriteFile(filepath.Join(dir, "subscriptions.md"), []byte(subContent), 0644); err != nil {
		t.Fatalf("write subscriptions.md: %v", err)
	}
	if _, err := col.SyncPodcasts(); err != nil {
		t.Fatalf("SyncPodcasts: %v", err)
	}

	apiSrv, _ := newPodcastTestServer(t, col, http.DefaultClient)

	showsResp, err := http.Get(apiSrv.URL + "/api/podcasts/shows")
	if err != nil {
		t.Fatalf("GET shows: %v", err)
	}
	shows := decodeJSON[[]PodcastShowSummary](t, showsResp)
	if len(shows) != 1 || shows[0].Label != "Test Show" {
		t.Fatalf("unexpected shows response: %+v", shows)
	}

	episodesResp, err := http.Get(apiSrv.URL + "/api/podcasts/shows/Test%20Show/episodes")
	if err != nil {
		t.Fatalf("GET show episodes: %v", err)
	}
	episodes := decodeJSON[[]PodcastEpisode](t, episodesResp)
	if len(episodes) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(episodes))
	}
}

func TestHandlePodcastEpisodeDownloadEndpoint(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, _ := newAudioTestServer(t, []byte("fake mp3 bytes"), "audio/mpeg")
	defer audioSrv.Close()

	id := syncOneEpisode(t, col, "Test Show", audioSrv.URL)

	apiSrv, _ := newPodcastTestServer(t, col, http.DefaultClient)

	resp, err := http.Post(fmt.Sprintf("%s/api/podcasts/episodes/%s/download", apiSrv.URL, id), "", nil)
	if err != nil {
		t.Fatalf("POST download: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	ep := decodeJSON[PodcastEpisode](t, resp)
	if ep.Status != PodcastStatusDownloaded {
		t.Errorf("expected status %q, got %q", PodcastStatusDownloaded, ep.Status)
	}
	if _, err := os.Stat(filepath.Join(col.cfg.AudioDir, ep.ContentURL)); err != nil {
		t.Errorf("expected downloaded file to exist: %v", err)
	}
}

func TestHandlePodcastEpisodeDownloadEndpointNotFound(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	apiSrv, _ := newPodcastTestServer(t, col, http.DefaultClient)

	resp, err := http.Post(apiSrv.URL+"/api/podcasts/episodes/no-such-id/download", "", nil)
	if err != nil {
		t.Fatalf("POST download: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown episode id, got %d", resp.StatusCode)
	}
}

func TestHandlePodcastEpisodeListenedUnlistenedEndpoints(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, _ := newAudioTestServer(t, []byte("fake mp3 bytes"), "audio/mpeg")
	defer audioSrv.Close()
	id := syncOneEpisode(t, col, "Test Show", audioSrv.URL)
	if _, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, id); err != nil {
		t.Fatalf("download: %v", err)
	}

	apiSrv, _ := newPodcastTestServer(t, col, http.DefaultClient)

	listenedResp, err := http.Post(fmt.Sprintf("%s/api/podcasts/episodes/%s/listened", apiSrv.URL, id), "", nil)
	if err != nil {
		t.Fatalf("POST listened: %v", err)
	}
	if listenedResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", listenedResp.StatusCode)
	}
	listened := decodeJSON[PodcastEpisode](t, listenedResp)
	if listened.Status != PodcastStatusListened || listened.ListenedAt == "" {
		t.Fatalf("unexpected episode after listened: %+v", listened)
	}

	unlistenedResp, err := http.Post(fmt.Sprintf("%s/api/podcasts/episodes/%s/unlistened", apiSrv.URL, id), "", nil)
	if err != nil {
		t.Fatalf("POST unlistened: %v", err)
	}
	if unlistenedResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", unlistenedResp.StatusCode)
	}
	unlistened := decodeJSON[PodcastEpisode](t, unlistenedResp)
	if unlistened.Status != PodcastStatusDownloaded || unlistened.ListenedAt != "" {
		t.Fatalf("unexpected episode after unlistened: %+v", unlistened)
	}
}

func TestHandlePodcastEpisodeKeepEndpoint(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, _ := newAudioTestServer(t, []byte("fake mp3 bytes"), "audio/mpeg")
	defer audioSrv.Close()
	id := syncOneEpisode(t, col, "Test Show", audioSrv.URL)

	apiSrv, _ := newPodcastTestServer(t, col, http.DefaultClient)

	resp, err := http.Post(
		fmt.Sprintf("%s/api/podcasts/episodes/%s/keep", apiSrv.URL, id),
		"application/json",
		strings.NewReader(`{"keep":true}`),
	)
	if err != nil {
		t.Fatalf("POST keep: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	ep := decodeJSON[PodcastEpisode](t, resp)
	if !ep.Keep {
		t.Errorf("expected Keep=true, got %+v", ep)
	}
}

func TestHandlePodcastEpisodeDeleteEndpoint(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, _ := newAudioTestServer(t, []byte("fake mp3 bytes"), "audio/mpeg")
	defer audioSrv.Close()
	id := syncOneEpisode(t, col, "Test Show", audioSrv.URL)
	ep, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, id)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	fullPath := filepath.Join(col.cfg.AudioDir, ep.ContentURL)

	apiSrv, _ := newPodcastTestServer(t, col, http.DefaultClient)

	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/podcasts/episodes/%s", apiSrv.URL, id), nil)
	if err != nil {
		t.Fatalf("building DELETE request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE episode: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		t.Errorf("expected downloaded file to be removed, stat err = %v", err)
	}
}

func TestHandlePodcastSweepEndpoint(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, _ := newAudioTestServer(t, []byte("fake mp3 bytes"), "audio/mpeg")
	defer audioSrv.Close()
	id := syncOneEpisode(t, col, "Test Show", audioSrv.URL)
	if _, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, id); err != nil {
		t.Fatalf("download: %v", err)
	}
	setListenedAt(t, col, id, 20)

	apiSrv, _ := newPodcastTestServer(t, col, http.DefaultClient)

	resp, err := http.Post(apiSrv.URL+"/api/podcasts/sweep", "", nil)
	if err != nil {
		t.Fatalf("POST sweep: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	var final map[string]any
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		statusResp, err := http.Get(apiSrv.URL + "/api/podcasts/sweep/status")
		if err != nil {
			t.Fatalf("GET sweep/status: %v", err)
		}
		body := decodeJSON[map[string]any](t, statusResp)
		if body["status"] != "running" {
			final = body
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if final == nil {
		t.Fatal("sweep did not complete within the deadline")
	}
	if final["status"] != "completed" {
		t.Fatalf("expected status 'completed', got %+v", final)
	}
	if n, ok := final["episodes_removed"].(float64); !ok || n != 1 {
		t.Errorf("expected episodes_removed=1, got %+v", final["episodes_removed"])
	}
	if _, err := col.getPodcastEpisode(id); err == nil {
		t.Error("expected the swept episode to be gone")
	}
}

func TestHandlePodcastEpisodeMigrateEndpoint(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, _ := newAudioTestServer(t, []byte("fake mp3 bytes"), "audio/mpeg")
	defer audioSrv.Close()
	id := syncOneEpisode(t, col, "Test Show", audioSrv.URL)
	if _, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, id); err != nil {
		t.Fatalf("download: %v", err)
	}

	apiSrv, _ := newPodcastTestServer(t, col, http.DefaultClient)

	resp, err := http.Post(
		fmt.Sprintf("%s/api/podcasts/episodes/%s/migrate", apiSrv.URL, id),
		"application/json",
		strings.NewReader(`{"destination":"Classical/Lectures"}`),
	)
	if err != nil {
		t.Fatalf("POST migrate: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body := decodeJSON[map[string]any](t, resp)
	audioID, _ := body["audio_id"].(string)
	if audioID == "" {
		t.Fatalf("expected a non-empty audio_id in response: %+v", body)
	}

	info, err := col.Read(audioID)
	if err != nil {
		t.Fatalf("Read migrated record: %v", err)
	}
	if !strings.Contains(filepath.ToSlash(info.ContentURL), "Classical/Lectures/") {
		t.Errorf("expected content URL under Classical/Lectures/, got %q", info.ContentURL)
	}
}

func TestHandlePodcastEpisodeMigrateEndpointNotFound(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	apiSrv, _ := newPodcastTestServer(t, col, http.DefaultClient)

	resp, err := http.Post(
		apiSrv.URL+"/api/podcasts/episodes/no-such-id/migrate",
		"application/json",
		strings.NewReader(`{"destination":"Classical"}`),
	)
	if err != nil {
		t.Fatalf("POST migrate: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown episode id, got %d", resp.StatusCode)
	}
}

func TestHandlePodcastSubscriptionsEndpoints(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	apiSrv, _ := newPodcastTestServer(t, col, http.DefaultClient)

	empty, err := http.Get(apiSrv.URL + "/api/podcasts/subscriptions")
	if err != nil {
		t.Fatalf("GET subscriptions: %v", err)
	}
	emptySubs := decodeJSON[[]Subscription](t, empty)
	if len(emptySubs) != 0 {
		t.Fatalf("expected no subscriptions yet, got %+v", emptySubs)
	}

	addResp, err := http.Post(
		apiSrv.URL+"/api/podcasts/subscriptions",
		"application/json",
		strings.NewReader(`{"label":"Radiolab","feedURL":"https://feeds.wnyc.org/radiolab"}`),
	)
	if err != nil {
		t.Fatalf("POST subscriptions: %v", err)
	}
	if addResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", addResp.StatusCode)
	}
	subs := decodeJSON[[]Subscription](t, addResp)
	if len(subs) != 1 || subs[0].Label != "Radiolab" {
		t.Fatalf("unexpected subscriptions after add: %+v", subs)
	}

	badResp, err := http.Post(
		apiSrv.URL+"/api/podcasts/subscriptions",
		"application/json",
		strings.NewReader(`{"label":"","feedURL":""}`),
	)
	if err != nil {
		t.Fatalf("POST subscriptions (invalid): %v", err)
	}
	defer badResp.Body.Close()
	if badResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty label/feedURL, got %d", badResp.StatusCode)
	}
}

func TestHandlePodcastEpisodeAudioEndpoint(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, _ := newAudioTestServer(t, []byte("fake mp3 bytes"), "audio/mpeg")
	defer audioSrv.Close()
	id := syncOneEpisode(t, col, "Test Show", audioSrv.URL)
	if _, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, id); err != nil {
		t.Fatalf("download: %v", err)
	}

	apiSrv, _ := newPodcastTestServer(t, col, http.DefaultClient)

	resp, err := http.Get(fmt.Sprintf("%s/api/podcasts/episodes/%s/audio", apiSrv.URL, id))
	if err != nil {
		t.Fatalf("GET audio: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	if string(body) != "fake mp3 bytes" {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestHandlePodcastEpisodeAudioEndpointNotFound(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	apiSrv, _ := newPodcastTestServer(t, col, http.DefaultClient)

	resp, err := http.Get(apiSrv.URL + "/api/podcasts/episodes/no-such-id/audio")
	if err != nil {
		t.Fatalf("GET audio: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown episode id, got %d", resp.StatusCode)
	}
}

func TestHandlePodcastEpisodeAudioEndpointNeverDownloaded(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, _ := newAudioTestServer(t, []byte("fake mp3 bytes"), "audio/mpeg")
	defer audioSrv.Close()
	id := syncOneEpisode(t, col, "Test Show", audioSrv.URL) // never downloaded

	apiSrv, _ := newPodcastTestServer(t, col, http.DefaultClient)

	resp, err := http.Get(fmt.Sprintf("%s/api/podcasts/episodes/%s/audio", apiSrv.URL, id))
	if err != nil {
		t.Fatalf("GET audio: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for an episode with no downloaded file, got %d", resp.StatusCode)
	}
}
