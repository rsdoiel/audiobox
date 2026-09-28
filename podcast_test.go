package audiobox

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSubscriptions(t *testing.T) {
	data := []byte(`# Podcast Subscriptions

Some intro prose that should be ignored.

- [Radiolab](https://feeds.wnyc.org/radiolab)
* [Darknet Diaries](https://feeds.megaphone.fm/darknetdiaries)
- not a subscription line
- [Missing URL]()
- [](https://example.com/no-label)
`)

	subs := ParseSubscriptions(data)
	if len(subs) != 2 {
		t.Fatalf("expected 2 subscriptions, got %d: %+v", len(subs), subs)
	}
	if subs[0].Label != "Radiolab" || subs[0].FeedURL != "https://feeds.wnyc.org/radiolab" {
		t.Errorf("unexpected first subscription: %+v", subs[0])
	}
	if subs[1].Label != "Darknet Diaries" || subs[1].FeedURL != "https://feeds.megaphone.fm/darknetdiaries" {
		t.Errorf("unexpected second subscription: %+v", subs[1])
	}
}

func TestLoadSubscriptionsMissingFile(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	subs, err := col.LoadSubscriptions()
	if err != nil {
		t.Fatalf("LoadSubscriptions: %v", err)
	}
	if subs != nil {
		t.Errorf("expected nil subscriptions when file is missing, got %+v", subs)
	}
}

func TestLoadSubscriptionsReadsFile(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	dir := filepath.Join(col.cfg.AudioDir, "Podcasts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := "- [Test Show](https://example.com/feed.xml)\n"
	if err := os.WriteFile(filepath.Join(dir, "subscriptions.md"), []byte(content), 0644); err != nil {
		t.Fatalf("write subscriptions.md: %v", err)
	}

	subs, err := col.LoadSubscriptions()
	if err != nil {
		t.Fatalf("LoadSubscriptions: %v", err)
	}
	if len(subs) != 1 || subs[0].Label != "Test Show" || subs[0].FeedURL != "https://example.com/feed.xml" {
		t.Fatalf("unexpected subscriptions: %+v", subs)
	}
}

const testFeedTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>Test Show</title>
    <item>
      <title>Episode One</title>
      <guid>episode-1</guid>
      <pubDate>Mon, 01 Sep 2026 12:00:00 GMT</pubDate>
      <itunes:duration>1:02:03</itunes:duration>
      <enclosure url="https://example.com/ep1.mp3" length="1000" type="audio/mpeg"/>
    </item>
    <item>
      <title>Episode Two</title>
      <guid>episode-2</guid>
      <pubDate>Tue, 02 Sep 2026 12:00:00 GMT</pubDate>
      <enclosure url="https://example.com/ep2.mp3" length="2000" type="audio/mpeg"/>
    </item>
    <item>
      <title>No identifier at all</title>
    </item>
  </channel>
</rss>`

// newTestFeedServer serves testFeedTemplate, honoring conditional GET via a
// fixed ETag, and counts how many requests actually received a 200 (as
// opposed to a 304).
func newTestFeedServer(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	const etag = `"feed-v1"`
	fullFetches := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		fullFetches++
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, testFeedTemplate)
	}))
	return srv, &fullFetches
}

func TestSyncPodcastsInsertsNewEpisodesAndIsIdempotent(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	srv, fullFetches := newTestFeedServer(t)
	defer srv.Close()

	dir := filepath.Join(col.cfg.AudioDir, "Podcasts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	subContent := fmt.Sprintf("- [Test Show](%s)\n", srv.URL)
	if err := os.WriteFile(filepath.Join(dir, "subscriptions.md"), []byte(subContent), 0644); err != nil {
		t.Fatalf("write subscriptions.md: %v", err)
	}

	result, err := col.SyncPodcasts()
	if err != nil {
		t.Fatalf("SyncPodcasts (first): %v", err)
	}
	if result.FeedsChecked != 1 {
		t.Errorf("expected 1 feed checked, got %d", result.FeedsChecked)
	}
	if result.NewEpisodes != 2 {
		t.Errorf("expected 2 new episodes (the item with no guid/enclosure is skipped), got %d", result.NewEpisodes)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %+v", result.Errors)
	}

	episodes, err := col.ListPodcastEpisodes(srv.URL)
	if err != nil {
		t.Fatalf("ListPodcastEpisodes: %v", err)
	}
	if len(episodes) != 2 {
		t.Fatalf("expected 2 stored episodes, got %d: %+v", len(episodes), episodes)
	}
	for _, ep := range episodes {
		if ep.Status != PodcastStatusNew {
			t.Errorf("expected status %q, got %q", PodcastStatusNew, ep.Status)
		}
		if ep.ShowLabel != "Test Show" {
			t.Errorf("expected show label %q, got %q", "Test Show", ep.ShowLabel)
		}
	}

	// Second sync: server should see the cached ETag and answer 304; no new
	// rows, no duplicate insert, and the underlying feed is fetched in full
	// exactly once across both syncs.
	result2, err := col.SyncPodcasts()
	if err != nil {
		t.Fatalf("SyncPodcasts (second): %v", err)
	}
	if result2.NewEpisodes != 0 {
		t.Errorf("expected 0 new episodes on second sync, got %d", result2.NewEpisodes)
	}
	if *fullFetches != 1 {
		t.Errorf("expected exactly 1 full fetch across both syncs (conditional GET should short-circuit the second), got %d", *fullFetches)
	}

	episodesAfter, err := col.ListPodcastEpisodes(srv.URL)
	if err != nil {
		t.Fatalf("ListPodcastEpisodes (after second sync): %v", err)
	}
	if len(episodesAfter) != 2 {
		t.Fatalf("expected still 2 stored episodes after second sync, got %d", len(episodesAfter))
	}
}

func TestSyncPodcastsRecordsFeedError(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir := filepath.Join(col.cfg.AudioDir, "Podcasts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	subContent := fmt.Sprintf("- [Broken Show](%s)\n", srv.URL)
	if err := os.WriteFile(filepath.Join(dir, "subscriptions.md"), []byte(subContent), 0644); err != nil {
		t.Fatalf("write subscriptions.md: %v", err)
	}

	result, err := col.SyncPodcasts()
	if err != nil {
		t.Fatalf("SyncPodcasts: %v", err)
	}
	if result.FeedsChecked != 1 {
		t.Errorf("expected 1 feed checked, got %d", result.FeedsChecked)
	}
	if result.NewEpisodes != 0 {
		t.Errorf("expected 0 new episodes, got %d", result.NewEpisodes)
	}
	if msg, ok := result.Errors[srv.URL]; !ok || msg == "" {
		t.Errorf("expected a recorded error for %s, got %+v", srv.URL, result.Errors)
	}
}

func TestItunesDurationToISO8601(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"45", "PT45S"},
		{"5:30", "PT5M30S"},
		{"1:02:03", "PT1H2M3S"},
		{"0:00:00", ""},
		{"not-a-duration", ""},
	}
	for _, c := range cases {
		got := itunesDurationToISO8601(c.in)
		if got != c.want {
			t.Errorf("itunesDurationToISO8601(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestListPodcastShows(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	srv, _ := newTestFeedServer(t)
	defer srv.Close()

	dir := filepath.Join(col.cfg.AudioDir, "Podcasts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	subContent := fmt.Sprintf("- [Test Show](%s)\n", srv.URL)
	if err := os.WriteFile(filepath.Join(dir, "subscriptions.md"), []byte(subContent), 0644); err != nil {
		t.Fatalf("write subscriptions.md: %v", err)
	}
	if _, err := col.SyncPodcasts(); err != nil {
		t.Fatalf("SyncPodcasts: %v", err)
	}

	shows, err := col.ListPodcastShows()
	if err != nil {
		t.Fatalf("ListPodcastShows: %v", err)
	}
	if len(shows) != 1 {
		t.Fatalf("expected 1 show, got %d: %+v", len(shows), shows)
	}
	if shows[0].Label != "Test Show" || shows[0].FeedURL != srv.URL {
		t.Errorf("unexpected show: %+v", shows[0])
	}
	if shows[0].EpisodeCount != 2 {
		t.Errorf("expected episode count 2, got %d", shows[0].EpisodeCount)
	}
	if shows[0].LastSyncedAt == "" {
		t.Errorf("expected a non-empty last synced timestamp")
	}
	if shows[0].LastError != "" {
		t.Errorf("expected no last error, got %q", shows[0].LastError)
	}
}

func TestListPodcastEpisodesByShow(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	srv, _ := newTestFeedServer(t)
	defer srv.Close()

	dir := filepath.Join(col.cfg.AudioDir, "Podcasts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	subContent := fmt.Sprintf("- [Test Show](%s)\n", srv.URL)
	if err := os.WriteFile(filepath.Join(dir, "subscriptions.md"), []byte(subContent), 0644); err != nil {
		t.Fatalf("write subscriptions.md: %v", err)
	}
	if _, err := col.SyncPodcasts(); err != nil {
		t.Fatalf("SyncPodcasts: %v", err)
	}

	episodes, err := col.ListPodcastEpisodesByShow("Test Show")
	if err != nil {
		t.Fatalf("ListPodcastEpisodesByShow: %v", err)
	}
	if len(episodes) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(episodes))
	}

	none, err := col.ListPodcastEpisodesByShow("No Such Show")
	if err != nil {
		t.Fatalf("ListPodcastEpisodesByShow (no match): %v", err)
	}
	if len(none) != 0 {
		t.Errorf("expected 0 episodes for an unknown show, got %d", len(none))
	}
}

// newAudioTestServer serves fixed bytes with the given content type, counting
// how many requests it actually receives (so a test can assert idempotency).
func newAudioTestServer(t *testing.T, body []byte, contentType string) (*httptest.Server, *int) {
	t.Helper()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	return srv, &hits
}

// syncOneEpisode wires up a one-episode feed under label pointing its
// enclosure at audioSrv, syncs it, and returns that episode's id.
func syncOneEpisode(t *testing.T, col *Collection, label, audioURL string) string {
	t.Helper()
	feedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `<?xml version="1.0"?>
<rss version="2.0"><channel><title>%s</title>
<item><title>Ep</title><guid>ep-1</guid>
<enclosure url="%s" length="10" type="audio/mpeg"/></item>
</channel></rss>`, label, audioURL)
	}))
	t.Cleanup(feedSrv.Close)

	dir := filepath.Join(col.cfg.AudioDir, "Podcasts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	subContent := fmt.Sprintf("- [%s](%s)\n", label, feedSrv.URL)
	if err := os.WriteFile(filepath.Join(dir, "subscriptions.md"), []byte(subContent), 0644); err != nil {
		t.Fatalf("write subscriptions.md: %v", err)
	}
	if _, err := col.SyncPodcasts(); err != nil {
		t.Fatalf("SyncPodcasts: %v", err)
	}
	// Scoped by feed_url, not show_label: callers may sync several
	// distinctly-fed episodes under the same show label in one test.
	episodes, err := col.ListPodcastEpisodes(feedSrv.URL)
	if err != nil || len(episodes) != 1 {
		t.Fatalf("expected exactly 1 synced episode, got %d, err=%v", len(episodes), err)
	}
	return episodes[0].ID
}

func TestDownloadPodcastEpisode(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, hits := newAudioTestServer(t, []byte("fake mp3 bytes"), "audio/mpeg")
	defer audioSrv.Close()

	id := syncOneEpisode(t, col, "Test Show", audioSrv.URL)

	ep, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, id)
	if err != nil {
		t.Fatalf("DownloadPodcastEpisodeWithClient: %v", err)
	}
	if ep.Status != PodcastStatusDownloaded {
		t.Errorf("expected status %q, got %q", PodcastStatusDownloaded, ep.Status)
	}
	if ep.ContentURL == "" {
		t.Fatal("expected a non-empty content URL")
	}
	fullPath := filepath.Join(col.cfg.AudioDir, ep.ContentURL)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("reading downloaded file %s: %v", fullPath, err)
	}
	if string(data) != "fake mp3 bytes" {
		t.Errorf("unexpected file contents: %q", data)
	}
	if !strings.Contains(filepath.ToSlash(ep.ContentURL), "Podcasts/Test-Show/") {
		t.Errorf("expected content URL under Podcasts/Test-Show/, got %q", ep.ContentURL)
	}

	// Second call must not re-fetch: already downloaded, so it's a no-op
	// that returns the same result.
	ep2, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, id)
	if err != nil {
		t.Fatalf("DownloadPodcastEpisodeWithClient (second): %v", err)
	}
	if ep2.ContentURL != ep.ContentURL {
		t.Errorf("expected same content URL on repeat download, got %q vs %q", ep2.ContentURL, ep.ContentURL)
	}
	if *hits != 1 {
		t.Errorf("expected exactly 1 fetch across both calls (idempotent), got %d", *hits)
	}
}

func TestDownloadPodcastEpisodeRejectsNonAudioContentType(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, _ := newAudioTestServer(t, []byte("<html>not audio</html>"), "text/html")
	defer audioSrv.Close()

	id := syncOneEpisode(t, col, "Test Show", audioSrv.URL)

	_, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, id)
	if err == nil {
		t.Fatal("expected an error for a non-audio content type")
	}

	episodes, err := col.ListPodcastEpisodesByShow("Test Show")
	if err != nil {
		t.Fatalf("ListPodcastEpisodesByShow: %v", err)
	}
	if len(episodes) != 1 || episodes[0].Status != PodcastStatusNew || episodes[0].ContentURL != "" {
		t.Fatalf("expected episode to remain unchanged (status new, no content URL), got %+v", episodes)
	}
}

func TestDownloadPodcastEpisodeNotFound(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	_, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, "no-such-id")
	if err == nil {
		t.Fatal("expected an error for an unknown episode id")
	}
}

func TestMarkPodcastEpisodeListenedAndUnlistened(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, _ := newAudioTestServer(t, []byte("fake mp3 bytes"), "audio/mpeg")
	defer audioSrv.Close()
	id := syncOneEpisode(t, col, "Test Show", audioSrv.URL)
	if _, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, id); err != nil {
		t.Fatalf("download: %v", err)
	}

	listened, err := col.MarkPodcastEpisodeListened(id)
	if err != nil {
		t.Fatalf("MarkPodcastEpisodeListened: %v", err)
	}
	if listened.Status != PodcastStatusListened {
		t.Errorf("expected status %q, got %q", PodcastStatusListened, listened.Status)
	}
	if listened.ListenedAt == "" {
		t.Error("expected a non-empty ListenedAt")
	}

	unlistened, err := col.MarkPodcastEpisodeUnlistened(id)
	if err != nil {
		t.Fatalf("MarkPodcastEpisodeUnlistened: %v", err)
	}
	if unlistened.Status != PodcastStatusDownloaded {
		t.Errorf("expected status reverted to %q, got %q", PodcastStatusDownloaded, unlistened.Status)
	}
	if unlistened.ListenedAt != "" {
		t.Errorf("expected ListenedAt cleared, got %q", unlistened.ListenedAt)
	}
}

func TestMarkPodcastEpisodeListenedNotFound(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	if _, err := col.MarkPodcastEpisodeListened("no-such-id"); err == nil {
		t.Fatal("expected an error for an unknown episode id")
	}
	if _, err := col.MarkPodcastEpisodeUnlistened("no-such-id"); err == nil {
		t.Fatal("expected an error for an unknown episode id")
	}
}

func TestSetPodcastEpisodeKeep(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, _ := newAudioTestServer(t, []byte("fake mp3 bytes"), "audio/mpeg")
	defer audioSrv.Close()
	id := syncOneEpisode(t, col, "Test Show", audioSrv.URL)

	kept, err := col.SetPodcastEpisodeKeep(id, true)
	if err != nil {
		t.Fatalf("SetPodcastEpisodeKeep(true): %v", err)
	}
	if !kept.Keep {
		t.Error("expected Keep to be true")
	}

	unkept, err := col.SetPodcastEpisodeKeep(id, false)
	if err != nil {
		t.Fatalf("SetPodcastEpisodeKeep(false): %v", err)
	}
	if unkept.Keep {
		t.Error("expected Keep to be false")
	}
}

func TestDeletePodcastEpisode(t *testing.T) {
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
	if _, err := os.Stat(fullPath); err != nil {
		t.Fatalf("expected downloaded file to exist before delete: %v", err)
	}

	if err := col.DeletePodcastEpisode(id); err != nil {
		t.Fatalf("DeletePodcastEpisode: %v", err)
	}
	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		t.Errorf("expected downloaded file to be removed, stat err = %v", err)
	}
	if _, err := col.getPodcastEpisode(id); err == nil {
		t.Error("expected episode row to be gone after delete")
	}
}

func TestDeletePodcastEpisodeNeverDownloaded(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, _ := newAudioTestServer(t, []byte("fake mp3 bytes"), "audio/mpeg")
	defer audioSrv.Close()
	id := syncOneEpisode(t, col, "Test Show", audioSrv.URL)

	if err := col.DeletePodcastEpisode(id); err != nil {
		t.Fatalf("DeletePodcastEpisode (never downloaded, no file to remove): %v", err)
	}
}

// setListenedAt directly backdates an episode's listened_at (and marks it
// listened), simulating a listen that happened daysAgo days in the past —
// exercising the sweep's age comparison without waiting in real time.
func setListenedAt(t *testing.T, col *Collection, id string, daysAgo int) {
	t.Helper()
	_, err := col.db.Exec(
		`UPDATE podcast_episodes SET status = ?, listened_at = datetime('now', ?) WHERE id = ?`,
		PodcastStatusListened, fmt.Sprintf("-%d days", daysAgo), id,
	)
	if err != nil {
		t.Fatalf("backdating listened_at: %v", err)
	}
}

func TestSweepPodcastEpisodesRemovesOnlyStaleListened(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, _ := newAudioTestServer(t, []byte("fake mp3 bytes"), "audio/mpeg")
	defer audioSrv.Close()

	staleID := syncOneEpisode(t, col, "Show A", audioSrv.URL)
	if _, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, staleID); err != nil {
		t.Fatalf("download stale: %v", err)
	}
	setListenedAt(t, col, staleID, 20) // older than the 14-day default

	keptID := syncOneEpisode(t, col, "Show A", audioSrv.URL)
	if _, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, keptID); err != nil {
		t.Fatalf("download kept: %v", err)
	}
	setListenedAt(t, col, keptID, 20)
	if _, err := col.SetPodcastEpisodeKeep(keptID, true); err != nil {
		t.Fatalf("SetPodcastEpisodeKeep: %v", err)
	}

	recentID := syncOneEpisode(t, col, "Show A", audioSrv.URL)
	if _, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, recentID); err != nil {
		t.Fatalf("download recent: %v", err)
	}
	setListenedAt(t, col, recentID, 1) // within the 14-day default

	unlistenedID := syncOneEpisode(t, col, "Show A", audioSrv.URL)
	if _, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, unlistenedID); err != nil {
		t.Fatalf("download unlistened: %v", err)
	}

	n, err := col.SweepPodcastEpisodes()
	if err != nil {
		t.Fatalf("SweepPodcastEpisodes: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected exactly 1 episode removed, got %d", n)
	}
	if _, err := col.getPodcastEpisode(staleID); err == nil {
		t.Error("expected the stale listened episode to be removed")
	}
	for _, survivorID := range []string{keptID, recentID, unlistenedID} {
		if _, err := col.getPodcastEpisode(survivorID); err != nil {
			t.Errorf("expected episode %s to survive the sweep: %v", survivorID, err)
		}
	}
}

func TestSweepPodcastEpisodesDisabledByNegativeRetention(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, _ := newAudioTestServer(t, []byte("fake mp3 bytes"), "audio/mpeg")
	defer audioSrv.Close()
	id := syncOneEpisode(t, col, "Test Show", audioSrv.URL)
	if _, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, id); err != nil {
		t.Fatalf("download: %v", err)
	}
	setListenedAt(t, col, id, 999)

	col.cfg.PodcastRetentionDays = -1
	n, err := col.SweepPodcastEpisodes()
	if err != nil {
		t.Fatalf("SweepPodcastEpisodes: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 removed when retention is disabled, got %d", n)
	}
	if _, err := col.getPodcastEpisode(id); err != nil {
		t.Errorf("expected episode to survive when sweep is disabled: %v", err)
	}
}

func TestPodcastRetentionDays(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	if got := col.PodcastRetentionDays(); got != 14 {
		t.Errorf("expected default retention of 14 days, got %d", got)
	}

	col.cfg.PodcastRetentionDays = -1
	if got := col.PodcastRetentionDays(); got != 0 {
		t.Errorf("expected negative retention to disable (0), got %d", got)
	}

	col.cfg.PodcastRetentionDays = 30
	if got := col.PodcastRetentionDays(); got != 30 {
		t.Errorf("expected explicit retention of 30, got %d", got)
	}
}

func TestMigratePodcastEpisodeToLibrary(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, _ := newAudioTestServer(t, []byte("fake mp3 bytes"), "audio/mpeg")
	defer audioSrv.Close()
	id := syncOneEpisode(t, col, "Test Show", audioSrv.URL)
	ep, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, id)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	oldPath := filepath.Join(col.cfg.AudioDir, ep.ContentURL)

	audioID, err := col.MigratePodcastEpisodeToLibrary(id, "Classical/Lectures")
	if err != nil {
		t.Fatalf("MigratePodcastEpisodeToLibrary: %v", err)
	}
	if audioID == "" {
		t.Fatal("expected a non-empty audio_files id")
	}

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Errorf("expected old podcast file to be gone, stat err = %v", err)
	}

	info, err := col.Read(audioID)
	if err != nil {
		t.Fatalf("Read migrated record: %v", err)
	}
	if info.SchemaType != "AudioObject" {
		t.Errorf("expected SchemaType AudioObject, got %q", info.SchemaType)
	}
	if info.Name != "Ep" {
		t.Errorf("expected Name %q, got %q", "Ep", info.Name)
	}
	if info.InAlbum != "Test Show" {
		t.Errorf("expected InAlbum %q, got %q", "Test Show", info.InAlbum)
	}
	if len(info.ByArtist) != 1 || info.ByArtist[0].Type != "Organization" || info.ByArtist[0].Name != "Test Show" {
		t.Errorf("unexpected ByArtist: %+v", info.ByArtist)
	}
	if info.Checksum == "" {
		t.Error("expected a non-empty checksum")
	}
	newPath := filepath.Join(col.cfg.AudioDir, info.ContentURL)
	if !strings.Contains(filepath.ToSlash(info.ContentURL), "Classical/Lectures/") {
		t.Errorf("expected content URL under Classical/Lectures/, got %q", info.ContentURL)
	}
	if data, err := os.ReadFile(newPath); err != nil || string(data) != "fake mp3 bytes" {
		t.Errorf("expected migrated file at %s with original contents, err=%v data=%q", newPath, err, data)
	}

	if _, err := col.getPodcastEpisode(id); err == nil {
		t.Error("expected the podcast episode row to be gone after migration")
	}
}

func TestMigratePodcastEpisodeToLibraryRejectsPathEscape(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, _ := newAudioTestServer(t, []byte("fake mp3 bytes"), "audio/mpeg")
	defer audioSrv.Close()
	id := syncOneEpisode(t, col, "Test Show", audioSrv.URL)
	if _, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, id); err != nil {
		t.Fatalf("download: %v", err)
	}

	if _, err := col.MigratePodcastEpisodeToLibrary(id, "../../../etc"); err == nil {
		t.Fatal("expected an error for a destination that escapes AudioDir")
	}

	// Nothing should have changed: the episode is still there, unmigrated.
	if _, err := col.getPodcastEpisode(id); err != nil {
		t.Errorf("expected the podcast episode to remain after a rejected migration: %v", err)
	}
}

func TestMigratePodcastEpisodeToLibraryNoContentURL(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, _ := newAudioTestServer(t, []byte("fake mp3 bytes"), "audio/mpeg")
	defer audioSrv.Close()
	id := syncOneEpisode(t, col, "Test Show", audioSrv.URL) // never downloaded

	if _, err := col.MigratePodcastEpisodeToLibrary(id, "Classical"); err == nil {
		t.Fatal("expected an error migrating an episode with no downloaded file")
	}
}

func TestMigratePodcastEpisodeToLibraryNotFound(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	if _, err := col.MigratePodcastEpisodeToLibrary("no-such-id", "Classical"); err == nil {
		t.Fatal("expected an error for an unknown episode id")
	}
}

func TestMigratePodcastEpisodeToLibraryFilenameCollision(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	audioSrv, _ := newAudioTestServer(t, []byte("fake mp3 bytes"), "audio/mpeg")
	defer audioSrv.Close()

	id1 := syncOneEpisode(t, col, "Test Show", audioSrv.URL)
	if _, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, id1); err != nil {
		t.Fatalf("download 1: %v", err)
	}
	id2 := syncOneEpisode(t, col, "Test Show", audioSrv.URL) // same title "Ep" -> same base filename
	if _, err := col.DownloadPodcastEpisodeWithClient(http.DefaultClient, id2); err != nil {
		t.Fatalf("download 2: %v", err)
	}

	audioID1, err := col.MigratePodcastEpisodeToLibrary(id1, "Classical")
	if err != nil {
		t.Fatalf("migrate 1: %v", err)
	}
	audioID2, err := col.MigratePodcastEpisodeToLibrary(id2, "Classical")
	if err != nil {
		t.Fatalf("migrate 2: %v", err)
	}

	info1, err := col.Read(audioID1)
	if err != nil {
		t.Fatalf("read 1: %v", err)
	}
	info2, err := col.Read(audioID2)
	if err != nil {
		t.Fatalf("read 2: %v", err)
	}
	if info1.ContentURL == info2.ContentURL {
		t.Errorf("expected distinct content URLs for colliding filenames, both got %q", info1.ContentURL)
	}
}

func TestSeedPodcastSubscriptionsCreatesFile(t *testing.T) {
	audioDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(audioDir, "Podcasts"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := seedPodcastSubscriptions(audioDir); err != nil {
		t.Fatalf("seedPodcastSubscriptions: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(audioDir, "Podcasts", "subscriptions.md"))
	if err != nil {
		t.Fatalf("reading seeded subscriptions.md: %v", err)
	}
	subs := ParseSubscriptions(data)
	if len(subs) != 1 || subs[0].Label != "Marketplace" || subs[0].FeedURL != "https://feeds.publicradio.org/public_feeds/marketplace" {
		t.Fatalf("unexpected seeded subscriptions: %+v", subs)
	}
}

func TestSeedPodcastSubscriptionsDoesNotOverwriteExisting(t *testing.T) {
	audioDir := t.TempDir()
	dir := filepath.Join(audioDir, "Podcasts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	custom := "- [My Own Show](https://example.com/feed.xml)\n"
	if err := os.WriteFile(filepath.Join(dir, "subscriptions.md"), []byte(custom), 0644); err != nil {
		t.Fatalf("write existing subscriptions.md: %v", err)
	}

	if err := seedPodcastSubscriptions(audioDir); err != nil {
		t.Fatalf("seedPodcastSubscriptions: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "subscriptions.md"))
	if err != nil {
		t.Fatalf("reading subscriptions.md: %v", err)
	}
	if string(data) != custom {
		t.Errorf("expected existing subscriptions.md to survive untouched, got %q", data)
	}
}

func TestAppendSubscription(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	if err := col.AppendSubscription("Radiolab", "https://feeds.wnyc.org/radiolab"); err != nil {
		t.Fatalf("AppendSubscription: %v", err)
	}
	if err := col.AppendSubscription("Darknet Diaries", "https://feeds.megaphone.fm/darknetdiaries"); err != nil {
		t.Fatalf("AppendSubscription (second): %v", err)
	}

	subs, err := col.LoadSubscriptions()
	if err != nil {
		t.Fatalf("LoadSubscriptions: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 subscriptions, got %d: %+v", len(subs), subs)
	}
	if subs[0].Label != "Radiolab" || subs[1].Label != "Darknet Diaries" {
		t.Errorf("unexpected subscription order/content: %+v", subs)
	}
}

func TestAppendSubscriptionRejectsEmptyFields(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	if err := col.AppendSubscription("", "https://example.com/feed.xml"); err == nil {
		t.Error("expected an error for an empty label")
	}
	if err := col.AppendSubscription("Show", ""); err == nil {
		t.Error("expected an error for an empty feed URL")
	}

	subs, err := col.LoadSubscriptions()
	if err != nil {
		t.Fatalf("LoadSubscriptions: %v", err)
	}
	if len(subs) != 0 {
		t.Errorf("expected no subscriptions written, got %+v", subs)
	}
}
