package audiobox

import (
	"bufio"
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mmcdole/gofeed"
)

const (
	/** PodcastStatusNew marks an episode discovered by a sync but not yet downloaded. */
	PodcastStatusNew = "new"

	/** PodcastStatusDownloaded marks an episode whose audio file is present locally. */
	PodcastStatusDownloaded = "downloaded"

	/** PodcastStatusListened marks an episode the owner has finished listening to.
	 * Surviving the retention sweep regardless of age is the Keep flag's job
	 * (see PodcastEpisode.Keep), not a fifth status value — a listened episode
	 * that's also kept just carries both.
	 */
	PodcastStatusListened = "listened"
)

/** Subscription represents one entry parsed from Podcasts/subscriptions.md: a
 * display label and the feed URL it points at.
 *
 * Parameters:
 *   Label   (string) — display label shown in Audiobox
 *   FeedURL (string) — the RSS or Atom feed URL
 *
 * Example:
 *   sub := audiobox.Subscription{Label: "Radiolab", FeedURL: "https://feeds.wnyc.org/radiolab"}
 */
type Subscription struct {
	Label   string
	FeedURL string
}

// subscriptionLineRE matches a Markdown list item of the form "- [Label](URL)"
// or "* [Label](URL)", tolerating surrounding whitespace.
var subscriptionLineRE = regexp.MustCompile(`^\s*[-*]\s*\[([^\]]+)\]\(([^)]+)\)\s*$`)

/** ParseSubscriptions extracts podcast subscriptions from the contents of a
 * subscriptions.md file. Only Markdown list items shaped like
 * "- [Label](feed URL)" are recognised; headings, prose, and malformed
 * entries (empty label or empty URL) are silently skipped.
 *
 * Parameters:
 *   data ([]byte) — the file's contents
 *
 * Returns:
 *   []Subscription — subscriptions in file order; nil if none were found
 *
 * Example:
 *   subs := audiobox.ParseSubscriptions([]byte("- [Radiolab](https://feeds.wnyc.org/radiolab)\n"))
 */
func ParseSubscriptions(data []byte) []Subscription {
	var subs []Subscription
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		m := subscriptionLineRE.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		label := strings.TrimSpace(m[1])
		feedURL := strings.TrimSpace(m[2])
		if label == "" || feedURL == "" {
			continue
		}
		subs = append(subs, Subscription{Label: label, FeedURL: feedURL})
	}
	return subs
}

/** SubscriptionsPath returns the path to this collection's podcast subscription
 * file: Podcasts/subscriptions.md under AudioDir.
 *
 * Returns:
 *   string — absolute path to subscriptions.md
 *
 * Example:
 *   fmt.Println(col.SubscriptionsPath()) // "/home/alice/Audio/Podcasts/subscriptions.md"
 */
func (c *Collection) SubscriptionsPath() string {
	return filepath.Join(c.cfg.AudioDir, "Podcasts", "subscriptions.md")
}

// defaultPodcastSubscriptions seeds a freshly-initialised collection's
// Podcasts/subscriptions.md with one starter feed, so the file demonstrates
// its own format instead of being empty. Marketplace was the show requested
// for this seed.
const defaultPodcastSubscriptions = `# Podcast Subscriptions

- [Marketplace](https://feeds.publicradio.org/public_feeds/marketplace)
`

// seedPodcastSubscriptions writes defaultPodcastSubscriptions to
// audioDir/Podcasts/subscriptions.md if that file does not already exist.
// It never overwrites an existing file, hand-edited or otherwise — this is
// only ever a starting point for a brand-new collection.
func seedPodcastSubscriptions(audioDir string) error {
	path := filepath.Join(audioDir, "Podcasts", "subscriptions.md")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("checking %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(defaultPodcastSubscriptions), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

/** LoadSubscriptions reads and parses this collection's subscriptions.md.
 * A missing file is not an error: it returns a nil slice, since a fresh
 * collection has no subscriptions yet.
 *
 * Returns:
 *   []Subscription — subscriptions in file order; nil if the file is absent or empty
 *   error          — non-nil only on a read failure other than the file not existing
 *
 * Example:
 *   subs, err := col.LoadSubscriptions()
 */
func (c *Collection) LoadSubscriptions() ([]Subscription, error) {
	path := c.SubscriptionsPath()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading subscriptions %s: %w", path, err)
	}
	return ParseSubscriptions(data), nil
}

// errInvalidSubscription reports that AppendSubscription was called with an
// empty label or feed URL.
var errInvalidSubscription = errors.New("label and feed URL are both required")

/** AppendSubscription adds one subscription line to subscriptions.md,
 * creating the file (and its Podcasts directory) if neither exists yet.
 * Existing content is never rewritten — the new line is appended after
 * whatever is already there.
 *
 * Parameters:
 *   label   (string) — display label for the show
 *   feedURL (string) — the feed's URL
 *
 * Returns:
 *   error — non-nil if the collection is not open, either argument is
 *           empty, or the file can't be written
 *
 * Example:
 *   err := col.AppendSubscription("Radiolab", "https://feeds.wnyc.org/radiolab")
 */
func (c *Collection) AppendSubscription(label, feedURL string) error {
	if !c.isOpen {
		return fmt.Errorf("collection is not open")
	}
	if strings.TrimSpace(label) == "" || strings.TrimSpace(feedURL) == "" {
		return errInvalidSubscription
	}

	path := c.SubscriptionsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()
	if _, err := fmt.Fprintf(f, "- [%s](%s)\n", label, feedURL); err != nil {
		return fmt.Errorf("appending to %s: %w", path, err)
	}
	return nil
}

/** PodcastEpisode is one entry in the podcast_episodes table: an episode
 * discovered from a subscribed feed, or ingested from a file already on disk.
 *
 * Parameters:
 *   ID           (string)    — UUID v4
 *   FeedURL      (string)    — the subscribed feed this came from; empty for manually-ingested episodes
 *   ShowLabel    (string)    — denormalized show label, for display and matching
 *   GUID         (string)    — feed item GUID, or the enclosure URL when the feed has no GUID
 *   Title        (string)    — episode title
 *   Published    (string)    — RFC 3339 publish date/time from the feed
 *   EnclosureURL (string)    — remote audio URL; empty for manually-ingested episodes
 *   ContentURL   (string)    — local file path once downloaded or ingested
 *   Duration     (string)    — ISO 8601 duration (e.g. "PT1H2M3S"); empty if not known
 *   Status       (string)    — one of the PodcastStatus* constants
 *   ListenedAt   (string)    — RFC 3339 timestamp when marked listened; empty until then
 *   Keep         (bool)      — exempts this episode from the retention sweep
 *   Created      (time.Time) —
 *   Updated      (time.Time) —
 *
 * Example:
 *   ep := audiobox.PodcastEpisode{Title: "Episode One", Status: audiobox.PodcastStatusNew}
 */
type PodcastEpisode struct {
	ID           string
	FeedURL      string
	ShowLabel    string
	GUID         string
	Title        string
	Published    string
	EnclosureURL string
	ContentURL   string
	Duration     string
	Status       string
	ListenedAt   string
	Keep         bool
	Created      time.Time
	Updated      time.Time
}

/** SyncResult summarizes the outcome of a podcast feed sync.
 *
 * Parameters:
 *   FeedsChecked (int)               — number of subscriptions attempted
 *   NewEpisodes  (int)                — episodes newly discovered across all feeds
 *   Errors       (map[string]string) — feed URL to error message, for feeds that failed
 *
 * Example:
 *   result, err := col.SyncPodcasts()
 *   fmt.Printf("%d new across %d feeds\n", result.NewEpisodes, result.FeedsChecked)
 */
type SyncResult struct {
	FeedsChecked int
	NewEpisodes  int
	Errors       map[string]string
}

/** SyncPodcasts checks every feed in subscriptions.md for new episodes, using
 * http.DefaultClient. See SyncPodcastsWithClient for details.
 *
 * Returns:
 *   SyncResult — per-run summary; a per-feed failure is recorded in Errors, not returned as error
 *   error      — non-nil only if the collection is not open or subscriptions.md cannot be read
 *
 * Example:
 *   result, err := col.SyncPodcasts()
 */
func (c *Collection) SyncPodcasts() (SyncResult, error) {
	return c.SyncPodcastsWithClient(http.DefaultClient)
}

/** SyncPodcastsWithClient checks every feed in subscriptions.md for new
 * episodes, using the given HTTP client. This variant exists so tests can
 * inject a client pointed at an httptest.Server.
 *
 * For each feed it sends a conditional GET (If-None-Match / If-Modified-Since
 * from the previous sync's stored ETag/Last-Modified) so an unchanged feed
 * costs one round trip and no re-parsing. New episodes are inserted with
 * status "new" — audio is not downloaded by this call. A feed item with
 * neither a GUID nor an enclosure is skipped, since there is nothing stable
 * to deduplicate it against on a later sync.
 *
 * A failure on one feed (network error, non-200/304 response, unparseable
 * feed) is recorded in SyncResult.Errors and does not stop the remaining
 * feeds from being checked.
 *
 * Parameters:
 *   client (*http.Client) — HTTP client used for every feed request
 *
 * Returns:
 *   SyncResult — per-run summary
 *   error      — non-nil only if the collection is not open or subscriptions.md cannot be read
 *
 * Example:
 *   result, err := col.SyncPodcastsWithClient(&http.Client{Timeout: 30 * time.Second})
 */
func (c *Collection) SyncPodcastsWithClient(client *http.Client) (SyncResult, error) {
	if !c.isOpen {
		return SyncResult{}, fmt.Errorf("collection is not open")
	}
	subs, err := c.LoadSubscriptions()
	if err != nil {
		return SyncResult{}, err
	}

	result := SyncResult{Errors: map[string]string{}}
	for _, sub := range subs {
		result.FeedsChecked++
		n, err := c.syncFeed(client, sub)
		if err != nil {
			result.Errors[sub.FeedURL] = err.Error()
			continue
		}
		result.NewEpisodes += n
	}
	return result, nil
}

// syncFeed fetches and processes a single subscription, returning the number
// of newly-inserted episodes.
func (c *Collection) syncFeed(client *http.Client, sub Subscription) (int, error) {
	etag, lastModified, err := c.feedState(sub.FeedURL)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequest(http.MethodGet, sub.FeedURL, nil)
	if err != nil {
		return 0, fmt.Errorf("building request for %s: %w", sub.FeedURL, err)
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}

	resp, err := client.Do(req)
	if err != nil {
		c.upsertFeed(sub.FeedURL, sub.Label, etag, lastModified, err.Error())
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		if err := c.upsertFeed(sub.FeedURL, sub.Label, etag, lastModified, ""); err != nil {
			return 0, err
		}
		return 0, nil
	}
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("unexpected status %s", resp.Status)
		c.upsertFeed(sub.FeedURL, sub.Label, etag, lastModified, errMsg)
		return 0, fmt.Errorf("%s: %s", sub.FeedURL, errMsg)
	}

	parsed, err := gofeed.NewParser().Parse(resp.Body)
	if err != nil {
		c.upsertFeed(sub.FeedURL, sub.Label, etag, lastModified, err.Error())
		return 0, fmt.Errorf("parsing feed %s: %w", sub.FeedURL, err)
	}

	newETag := resp.Header.Get("ETag")
	newLastModified := resp.Header.Get("Last-Modified")
	if err := c.upsertFeed(sub.FeedURL, sub.Label, newETag, newLastModified, ""); err != nil {
		return 0, err
	}

	count := 0
	for _, item := range parsed.Items {
		inserted, err := c.insertEpisode(sub.FeedURL, sub.Label, item)
		if err != nil {
			return count, err
		}
		if inserted {
			count++
		}
	}
	return count, nil
}

// feedState returns the stored etag/last_modified for a feed, or two empty
// strings if the feed has never been synced before.
func (c *Collection) feedState(feedURL string) (etag, lastModified string, err error) {
	var e, l sql.NullString
	err = c.db.QueryRow(`SELECT etag, last_modified FROM podcast_feeds WHERE feed_url = ?`, feedURL).Scan(&e, &l)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("reading feed state %s: %w", feedURL, err)
	}
	return e.String, l.String, nil
}

// upsertFeed records the outcome of a sync attempt against one feed: its
// current label, the ETag/Last-Modified to send next time, and any error
// (empty on success). nullableString stores an empty value as SQL NULL
// rather than "", so feedState's sql.NullString round-trips cleanly.
func (c *Collection) upsertFeed(feedURL, label, etag, lastModified, lastErr string) error {
	_, err := c.db.Exec(`
		INSERT INTO podcast_feeds (feed_url, show_label, etag, last_modified, last_synced_at, last_error)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, ?)
		ON CONFLICT(feed_url) DO UPDATE SET
			show_label     = excluded.show_label,
			etag           = excluded.etag,
			last_modified  = excluded.last_modified,
			last_synced_at = excluded.last_synced_at,
			last_error     = excluded.last_error
	`, feedURL, label, nullableString(etag), nullableString(lastModified), nullableString(lastErr))
	if err != nil {
		return fmt.Errorf("upserting feed %s: %w", feedURL, err)
	}
	return nil
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// episodeExists reports whether an episode with this feed_url/guid pair is
// already stored, so a repeat sync does not insert a duplicate.
func (c *Collection) episodeExists(feedURL, guid string) (bool, error) {
	var one int
	err := c.db.QueryRow(`SELECT 1 FROM podcast_episodes WHERE feed_url = ? AND guid = ?`, feedURL, guid).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking episode existence: %w", err)
	}
	return true, nil
}

// episodeGUID derives a stable identifier for a feed item: its GUID, or
// failing that the URL of its first enclosure. Returns "" when neither is
// present, meaning the item cannot be safely deduplicated on a later sync.
func episodeGUID(item *gofeed.Item) string {
	if item.GUID != "" {
		return item.GUID
	}
	if len(item.Enclosures) > 0 {
		return item.Enclosures[0].URL
	}
	return ""
}

// insertEpisode inserts a new podcast_episodes row for item if it is not
// already known and has a usable guid. Returns whether a row was inserted.
func (c *Collection) insertEpisode(feedURL, showLabel string, item *gofeed.Item) (bool, error) {
	guid := episodeGUID(item)
	if guid == "" {
		return false, nil
	}
	exists, err := c.episodeExists(feedURL, guid)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}

	var enclosureURL string
	if len(item.Enclosures) > 0 {
		enclosureURL = item.Enclosures[0].URL
	}
	var published string
	if item.PublishedParsed != nil {
		published = item.PublishedParsed.UTC().Format(time.RFC3339)
	}
	var duration string
	if item.ITunesExt != nil {
		duration = itunesDurationToISO8601(item.ITunesExt.Duration)
	}

	id := uuid.NewString()
	_, err = c.db.Exec(`
		INSERT INTO podcast_episodes
			(id, feed_url, show_label, guid, title, published, enclosure_url, duration, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, feedURL, showLabel, guid, item.Title, published, enclosureURL, nullableString(duration), PodcastStatusNew)
	if err != nil {
		return false, fmt.Errorf("inserting episode %q: %w", guid, err)
	}
	return true, nil
}

// itunesDurationToISO8601 converts an iTunes-style duration ("125",
// "5:30", or "1:02:03") into an ISO 8601 duration ("PT2M5S", "PT5M30S",
// "PT1H2M3S"). Returns "" for empty, unparseable, or zero input.
func itunesDurationToISO8601(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, ":")
	var h, m, s int
	var err error
	switch len(parts) {
	case 1:
		s, err = strconv.Atoi(parts[0])
	case 2:
		if m, err = strconv.Atoi(parts[0]); err == nil {
			s, err = strconv.Atoi(parts[1])
		}
	case 3:
		if h, err = strconv.Atoi(parts[0]); err == nil {
			if m, err = strconv.Atoi(parts[1]); err == nil {
				s, err = strconv.Atoi(parts[2])
			}
		}
	default:
		return ""
	}
	if err != nil {
		return ""
	}

	total := h*3600 + m*60 + s
	if total <= 0 {
		return ""
	}
	h, m, s = total/3600, (total%3600)/60, total%60

	var b strings.Builder
	b.WriteString("PT")
	if h > 0 {
		fmt.Fprintf(&b, "%dH", h)
	}
	if m > 0 {
		fmt.Fprintf(&b, "%dM", m)
	}
	if s > 0 || (h == 0 && m == 0) {
		fmt.Fprintf(&b, "%dS", s)
	}
	return b.String()
}

// podcastEpisodeColumns is the column list shared by every query that scans
// into a PodcastEpisode, so the SELECT list and scanPodcastEpisode's Scan
// call can never drift apart.
const podcastEpisodeColumns = `id, feed_url, show_label, guid, title, published, enclosure_url,
	       content_url, duration, status, listened_at, keep, created, updated`

// rowScanner is satisfied by both *sql.Row and *sql.Rows, so
// scanPodcastEpisode works for a single-row lookup and a multi-row list.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanPodcastEpisode scans one row (selected with podcastEpisodeColumns) into
// a PodcastEpisode, resolving the nullable columns (feed_url, content_url,
// duration, listened_at) to "" and keep to a bool.
func scanPodcastEpisode(row rowScanner) (PodcastEpisode, error) {
	var e PodcastEpisode
	var feedURL, contentURL, duration, listenedAt sql.NullString
	var keep int
	if err := row.Scan(&e.ID, &feedURL, &e.ShowLabel, &e.GUID, &e.Title, &e.Published,
		&e.EnclosureURL, &contentURL, &duration, &e.Status, &listenedAt, &keep, &e.Created, &e.Updated); err != nil {
		return PodcastEpisode{}, err
	}
	e.FeedURL = feedURL.String
	e.ContentURL = contentURL.String
	e.Duration = duration.String
	e.ListenedAt = listenedAt.String
	e.Keep = keep != 0
	return e, nil
}

/** ListPodcastEpisodes returns every episode stored for a given feed, newest
 * published first.
 *
 * Parameters:
 *   feedURL (string) — the subscribed feed to list episodes for
 *
 * Returns:
 *   []PodcastEpisode — episodes for that feed; nil if none
 *   error            — non-nil if the collection is not open or on a database failure
 *
 * Example:
 *   episodes, err := col.ListPodcastEpisodes("https://feeds.wnyc.org/radiolab")
 */
func (c *Collection) ListPodcastEpisodes(feedURL string) ([]PodcastEpisode, error) {
	if !c.isOpen {
		return nil, fmt.Errorf("collection is not open")
	}
	rows, err := c.db.Query(`SELECT `+podcastEpisodeColumns+`
		FROM podcast_episodes WHERE feed_url = ? ORDER BY published DESC`, feedURL)
	if err != nil {
		return nil, fmt.Errorf("listing episodes for %s: %w", feedURL, err)
	}
	defer rows.Close()

	var out []PodcastEpisode
	for rows.Next() {
		e, err := scanPodcastEpisode(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning episode: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing episodes for %s: %w", feedURL, err)
	}
	return out, nil
}

/** ListPodcastEpisodesByShow returns every episode stored under a given show
 * label (matching podcast_episodes.show_label, whether the episode came from
 * a feed sync or a manual ingest), newest published first.
 *
 * Parameters:
 *   label (string) — the show label to list episodes for
 *
 * Returns:
 *   []PodcastEpisode — episodes for that show; nil if none
 *   error            — non-nil if the collection is not open or on a database failure
 *
 * Example:
 *   episodes, err := col.ListPodcastEpisodesByShow("Radiolab")
 */
func (c *Collection) ListPodcastEpisodesByShow(label string) ([]PodcastEpisode, error) {
	if !c.isOpen {
		return nil, fmt.Errorf("collection is not open")
	}
	rows, err := c.db.Query(`SELECT `+podcastEpisodeColumns+`
		FROM podcast_episodes WHERE show_label = ? ORDER BY published DESC`, label)
	if err != nil {
		return nil, fmt.Errorf("listing episodes for show %q: %w", label, err)
	}
	defer rows.Close()

	var out []PodcastEpisode
	for rows.Next() {
		e, err := scanPodcastEpisode(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning episode: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing episodes for show %q: %w", label, err)
	}
	return out, nil
}

// getPodcastEpisode looks up a single episode by id, returning an error
// wrapping sql.ErrNoRows when it does not exist.
func (c *Collection) getPodcastEpisode(id string) (PodcastEpisode, error) {
	row := c.db.QueryRow(`SELECT `+podcastEpisodeColumns+`
		FROM podcast_episodes WHERE id = ?`, id)
	e, err := scanPodcastEpisode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return PodcastEpisode{}, fmt.Errorf("podcast episode %s: %w", id, sql.ErrNoRows)
	}
	if err != nil {
		return PodcastEpisode{}, fmt.Errorf("loading podcast episode %s: %w", id, err)
	}
	return e, nil
}

/** PodcastShowSummary describes one subscribed show for the Podcasts browse tab.
 *
 * Parameters:
 *   Label        (string) — display label, from subscriptions.md
 *   FeedURL      (string) — the feed URL
 *   EpisodeCount (int)    — number of episodes stored under this show's label
 *   LastSyncedAt (string) — RFC 3339 timestamp of the last sync attempt; empty if never synced
 *   LastError    (string) — error from the last sync attempt; empty on success
 *
 * Example:
 *   show := audiobox.PodcastShowSummary{Label: "Radiolab", EpisodeCount: 12}
 */
type PodcastShowSummary struct {
	Label        string `json:"label"`
	FeedURL      string `json:"feedURL"`
	EpisodeCount int    `json:"episodeCount"`
	LastSyncedAt string `json:"lastSyncedAt,omitempty"`
	LastError    string `json:"lastError,omitempty"`
}

/** ListPodcastShows returns every subscribed show, alphabetical by label, with
 * its episode count and last sync outcome.
 *
 * Returns:
 *   []PodcastShowSummary — one entry per row in podcast_feeds; nil if none
 *   error                — non-nil if the collection is not open or on a database failure
 *
 * Example:
 *   shows, err := col.ListPodcastShows()
 */
func (c *Collection) ListPodcastShows() ([]PodcastShowSummary, error) {
	if !c.isOpen {
		return nil, fmt.Errorf("collection is not open")
	}
	rows, err := c.db.Query(`
		SELECT f.feed_url, f.show_label, f.last_synced_at, f.last_error,
		       (SELECT COUNT(*) FROM podcast_episodes e WHERE e.show_label = f.show_label) AS episode_count
		FROM podcast_feeds f
		ORDER BY f.show_label COLLATE NOCASE
	`)
	if err != nil {
		return nil, fmt.Errorf("listing podcast shows: %w", err)
	}
	defer rows.Close()

	var out []PodcastShowSummary
	for rows.Next() {
		var s PodcastShowSummary
		var lastSyncedAt, lastError sql.NullString
		if err := rows.Scan(&s.FeedURL, &s.Label, &lastSyncedAt, &lastError, &s.EpisodeCount); err != nil {
			return nil, fmt.Errorf("scanning podcast show: %w", err)
		}
		s.LastSyncedAt = lastSyncedAt.String
		s.LastError = lastError.String
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing podcast shows: %w", err)
	}
	return out, nil
}

// podcastContentTypeExt maps common podcast enclosure content types to a
// file extension, so a downloaded episode gets a sensible filename even when
// the enclosure URL itself carries no extension.
var podcastContentTypeExt = map[string]string{
	"audio/mpeg":   ".mp3",
	"audio/mp3":    ".mp3",
	"audio/mp4":    ".m4a",
	"audio/x-m4a":  ".m4a",
	"audio/aac":    ".aac",
	"audio/ogg":    ".ogg",
	"audio/wav":    ".wav",
	"audio/x-wav":  ".wav",
	"audio/flac":   ".flac",
	"audio/x-flac": ".flac",
}

// extensionForContentType picks a file extension for a downloaded episode:
// by content type first, falling back to the enclosure URL's own extension,
// and finally to ".mp3" (the overwhelmingly common podcast format) when
// neither gives an answer.
func extensionForContentType(contentType, enclosureURL string) string {
	ct := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	if ext, ok := podcastContentTypeExt[ct]; ok {
		return ext
	}
	if ext := filepath.Ext(strings.SplitN(enclosureURL, "?", 2)[0]); ext != "" {
		return ext
	}
	return ".mp3"
}

// slugifyFilename converts a title into a filesystem-safe basename: runs of
// characters other than ASCII letters/digits collapse to a single hyphen,
// and leading/trailing hyphens are trimmed. Returns "episode" for an input
// with no alphanumeric characters at all.
func slugifyFilename(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if out == "" {
		return "episode"
	}
	return out
}

// uniquePodcastFilePath returns dir/baseSlug+ext, or, if that path already
// exists (two episodes sharing a title), dir/baseSlug-<id prefix>+ext.
func uniquePodcastFilePath(dir, baseSlug, ext, episodeID string) string {
	candidate := filepath.Join(dir, baseSlug+ext)
	if _, err := os.Stat(candidate); errors.Is(err, fs.ErrNotExist) {
		return candidate
	}
	suffix := episodeID
	if len(suffix) > 8 {
		suffix = suffix[:8]
	}
	return filepath.Join(dir, baseSlug+"-"+suffix+ext)
}

/** DownloadPodcastEpisode fetches an episode's audio using http.DefaultClient.
 * See DownloadPodcastEpisodeWithClient for details.
 *
 * Parameters:
 *   id (string) — the podcast_episodes id to download
 *
 * Returns:
 *   PodcastEpisode — the episode, updated with its new ContentURL and status
 *   error          — non-nil if the episode does not exist, has no enclosure, or the fetch/write fails
 *
 * Example:
 *   ep, err := col.DownloadPodcastEpisode(id)
 */
func (c *Collection) DownloadPodcastEpisode(id string) (PodcastEpisode, error) {
	return c.DownloadPodcastEpisodeWithClient(http.DefaultClient, id)
}

/** DownloadPodcastEpisodeWithClient fetches an episode's EnclosureURL and
 * saves it under AudioDir/Podcasts/<slugified show label>/, then marks the
 * episode downloaded. The response's Content-Type must start with "audio/";
 * anything else is rejected without writing a file, since sync only records
 * enclosure URLs and never inspects what they actually serve.
 *
 * Already-downloaded episodes (ContentURL already set) are returned as-is
 * without a network call — the operation is idempotent, so retrying a
 * "Download All New" batch after a partial failure is always safe.
 *
 * Parameters:
 *   client (*http.Client) — HTTP client used for the fetch
 *   id     (string)       — the podcast_episodes id to download
 *
 * Returns:
 *   PodcastEpisode — the episode, updated with its new ContentURL and status
 *   error          — non-nil if the episode does not exist, has no enclosure, or the fetch/write fails
 *
 * Example:
 *   ep, err := col.DownloadPodcastEpisodeWithClient(&http.Client{Timeout: 30 * time.Second}, id)
 */
func (c *Collection) DownloadPodcastEpisodeWithClient(client *http.Client, id string) (PodcastEpisode, error) {
	if !c.isOpen {
		return PodcastEpisode{}, fmt.Errorf("collection is not open")
	}
	ep, err := c.getPodcastEpisode(id)
	if err != nil {
		return PodcastEpisode{}, err
	}
	if ep.ContentURL != "" {
		return ep, nil
	}
	if ep.EnclosureURL == "" {
		return PodcastEpisode{}, fmt.Errorf("podcast episode %s has no enclosure to download", id)
	}

	resp, err := client.Get(ep.EnclosureURL)
	if err != nil {
		return PodcastEpisode{}, fmt.Errorf("fetching %s: %w", ep.EnclosureURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return PodcastEpisode{}, fmt.Errorf("fetching %s: unexpected status %s", ep.EnclosureURL, resp.Status)
	}
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(contentType), "audio/") {
		return PodcastEpisode{}, fmt.Errorf("fetching %s: unexpected content type %q, expected audio/*", ep.EnclosureURL, contentType)
	}

	destDir := filepath.Join(c.cfg.AudioDir, "Podcasts", slugifyFilename(ep.ShowLabel))
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return PodcastEpisode{}, fmt.Errorf("creating %s: %w", destDir, err)
	}
	destPath := uniquePodcastFilePath(destDir, slugifyFilename(ep.Title), extensionForContentType(contentType, ep.EnclosureURL), ep.ID)

	f, err := os.Create(destPath)
	if err != nil {
		return PodcastEpisode{}, fmt.Errorf("creating %s: %w", destPath, err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(destPath)
		return PodcastEpisode{}, fmt.Errorf("writing %s: %w", destPath, err)
	}
	if err := f.Close(); err != nil {
		os.Remove(destPath)
		return PodcastEpisode{}, fmt.Errorf("closing %s: %w", destPath, err)
	}

	rel, err := filepath.Rel(c.cfg.AudioDir, destPath)
	if err != nil {
		os.Remove(destPath)
		return PodcastEpisode{}, fmt.Errorf("computing relative path for %s: %w", destPath, err)
	}

	if _, err := c.db.Exec(`
		UPDATE podcast_episodes SET content_url = ?, status = ?, updated = CURRENT_TIMESTAMP WHERE id = ?
	`, rel, PodcastStatusDownloaded, id); err != nil {
		return PodcastEpisode{}, fmt.Errorf("updating podcast episode %s: %w", id, err)
	}

	ep.ContentURL = rel
	ep.Status = PodcastStatusDownloaded
	return ep, nil
}

/** PodcastRetentionDays returns how many days a listened episode is kept
 * before the sweep deletes it. The config field's zero value (unset) means
 * "use the default of 14"; a negative value means "sweep is disabled".
 *
 * Returns:
 *   int — retention window in days, or 0 if the sweep is disabled
 *
 * Example:
 *   days := col.PodcastRetentionDays() // 14 unless configured otherwise
 */
func (c *Collection) PodcastRetentionDays() int {
	switch {
	case c.cfg.PodcastRetentionDays < 0:
		return 0
	case c.cfg.PodcastRetentionDays == 0:
		return 14
	default:
		return c.cfg.PodcastRetentionDays
	}
}

/** MarkPodcastEpisodeListened sets an episode's status to "listened" and
 * records the current time as ListenedAt.
 *
 * Parameters:
 *   id (string) — the podcast_episodes id to mark
 *
 * Returns:
 *   PodcastEpisode — the episode after the update
 *   error          — non-nil if the episode does not exist or on a database failure
 *
 * Example:
 *   ep, err := col.MarkPodcastEpisodeListened(id)
 */
func (c *Collection) MarkPodcastEpisodeListened(id string) (PodcastEpisode, error) {
	if !c.isOpen {
		return PodcastEpisode{}, fmt.Errorf("collection is not open")
	}
	if _, err := c.getPodcastEpisode(id); err != nil {
		return PodcastEpisode{}, err
	}
	if _, err := c.db.Exec(`
		UPDATE podcast_episodes SET status = ?, listened_at = CURRENT_TIMESTAMP, updated = CURRENT_TIMESTAMP WHERE id = ?
	`, PodcastStatusListened, id); err != nil {
		return PodcastEpisode{}, fmt.Errorf("marking podcast episode %s listened: %w", id, err)
	}
	return c.getPodcastEpisode(id)
}

/** MarkPodcastEpisodeUnlistened reverts an episode's status to "downloaded"
 * (or "new" if it was never downloaded) and clears ListenedAt.
 *
 * Parameters:
 *   id (string) — the podcast_episodes id to revert
 *
 * Returns:
 *   PodcastEpisode — the episode after the update
 *   error          — non-nil if the episode does not exist or on a database failure
 *
 * Example:
 *   ep, err := col.MarkPodcastEpisodeUnlistened(id)
 */
func (c *Collection) MarkPodcastEpisodeUnlistened(id string) (PodcastEpisode, error) {
	if !c.isOpen {
		return PodcastEpisode{}, fmt.Errorf("collection is not open")
	}
	ep, err := c.getPodcastEpisode(id)
	if err != nil {
		return PodcastEpisode{}, err
	}
	newStatus := PodcastStatusNew
	if ep.ContentURL != "" {
		newStatus = PodcastStatusDownloaded
	}
	if _, err := c.db.Exec(`
		UPDATE podcast_episodes SET status = ?, listened_at = NULL, updated = CURRENT_TIMESTAMP WHERE id = ?
	`, newStatus, id); err != nil {
		return PodcastEpisode{}, fmt.Errorf("marking podcast episode %s unlistened: %w", id, err)
	}
	return c.getPodcastEpisode(id)
}

/** SetPodcastEpisodeKeep sets or clears an episode's Keep flag, which exempts
 * it from the retention sweep regardless of age.
 *
 * Parameters:
 *   id   (string) — the podcast_episodes id to update
 *   keep (bool)   — true to exempt from the sweep, false to make it sweepable again
 *
 * Returns:
 *   PodcastEpisode — the episode after the update
 *   error          — non-nil if the episode does not exist or on a database failure
 *
 * Example:
 *   ep, err := col.SetPodcastEpisodeKeep(id, true)
 */
func (c *Collection) SetPodcastEpisodeKeep(id string, keep bool) (PodcastEpisode, error) {
	if !c.isOpen {
		return PodcastEpisode{}, fmt.Errorf("collection is not open")
	}
	if _, err := c.getPodcastEpisode(id); err != nil {
		return PodcastEpisode{}, err
	}
	keepInt := 0
	if keep {
		keepInt = 1
	}
	if _, err := c.db.Exec(`
		UPDATE podcast_episodes SET keep = ?, updated = CURRENT_TIMESTAMP WHERE id = ?
	`, keepInt, id); err != nil {
		return PodcastEpisode{}, fmt.Errorf("setting keep on podcast episode %s: %w", id, err)
	}
	return c.getPodcastEpisode(id)
}

/** DeletePodcastEpisode removes an episode's row and, if present, its
 * downloaded audio file. A file that is already missing is not an error —
 * the row is removed either way.
 *
 * Parameters:
 *   id (string) — the podcast_episodes id to delete
 *
 * Returns:
 *   error — non-nil if the episode does not exist, the file can't be removed
 *           for a reason other than already being absent, or on a database failure
 *
 * Example:
 *   err := col.DeletePodcastEpisode(id)
 */
func (c *Collection) DeletePodcastEpisode(id string) error {
	if !c.isOpen {
		return fmt.Errorf("collection is not open")
	}
	ep, err := c.getPodcastEpisode(id)
	if err != nil {
		return err
	}
	if ep.ContentURL != "" {
		path := filepath.Join(c.cfg.AudioDir, ep.ContentURL)
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("deleting file %s: %w", path, err)
		}
	}
	if _, err := c.db.Exec(`DELETE FROM podcast_episodes WHERE id = ?`, id); err != nil {
		return fmt.Errorf("deleting podcast episode %s: %w", id, err)
	}
	return nil
}

/** SweepPodcastEpisodes deletes every episode (row and downloaded file) whose
 * status is "listened", whose Keep flag is false, and whose ListenedAt is
 * older than PodcastRetentionDays. When PodcastRetentionDays reports 0 (the
 * sweep disabled via a negative config value), it does nothing and returns 0.
 *
 * Returns:
 *   int   — number of episodes removed
 *   error — non-nil on a database or filesystem failure; a file already
 *           missing on disk is not an error
 *
 * Example:
 *   n, err := col.SweepPodcastEpisodes()
 */
func (c *Collection) SweepPodcastEpisodes() (int, error) {
	if !c.isOpen {
		return 0, fmt.Errorf("collection is not open")
	}
	retentionDays := c.PodcastRetentionDays()
	if retentionDays <= 0 {
		return 0, nil
	}

	rows, err := c.db.Query(`
		SELECT id, content_url FROM podcast_episodes
		WHERE status = ? AND keep = 0 AND listened_at IS NOT NULL
		  AND listened_at <= datetime('now', ?)
	`, PodcastStatusListened, fmt.Sprintf("-%d days", retentionDays))
	if err != nil {
		return 0, fmt.Errorf("sweep: querying stale episodes: %w", err)
	}
	type staleEpisode struct{ id, contentURL string }
	var stale []staleEpisode
	for rows.Next() {
		var s staleEpisode
		var contentURL sql.NullString
		if err := rows.Scan(&s.id, &contentURL); err != nil {
			rows.Close()
			return 0, fmt.Errorf("sweep: scanning row: %w", err)
		}
		s.contentURL = contentURL.String
		stale = append(stale, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("sweep: iterating rows: %w", err)
	}

	removed := 0
	for _, s := range stale {
		if s.contentURL != "" {
			path := filepath.Join(c.cfg.AudioDir, s.contentURL)
			if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return removed, fmt.Errorf("sweep: removing file %s: %w", path, err)
			}
		}
		if _, err := c.db.Exec(`DELETE FROM podcast_episodes WHERE id = ?`, s.id); err != nil {
			return removed, fmt.Errorf("sweep: deleting episode %s: %w", s.id, err)
		}
		removed++
	}
	return removed, nil
}

// resolveUnderAudioDir resolves a caller-supplied path (relative to
// AudioDir) to an absolute directory path, rejecting anything that would
// resolve outside AudioDir (e.g. "../../etc").
func (c *Collection) resolveUnderAudioDir(rel string) (string, error) {
	audioDir := filepath.Clean(c.cfg.AudioDir)
	joined := filepath.Join(audioDir, rel)
	if joined != audioDir && !strings.HasPrefix(joined, audioDir+string(filepath.Separator)) {
		return "", fmt.Errorf("destination %q escapes AudioDir", rel)
	}
	return joined, nil
}

// moveFile relocates src to dst, using os.Rename when possible (the common
// case: source and destination both live under the same AudioDir) and
// falling back to copy-then-remove when Rename fails, so a migration still
// works if AudioDir happens to span more than one filesystem.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %s: %w", src, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return fmt.Errorf("copying %s to %s: %w", src, dst, err)
	}
	if err := out.Close(); err != nil {
		os.Remove(dst)
		return fmt.Errorf("closing %s: %w", dst, err)
	}
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("removing original %s after copy: %w", src, err)
	}
	return nil
}

/** MigratePodcastEpisodeToLibrary moves a downloaded (or ingested) episode's
 * audio file into destination (a path relative to AudioDir, created if it
 * doesn't exist) and records it as a normal audio_files entry:
 * SchemaType "AudioObject", Name from the episode title, InAlbum from the
 * show label, ByArtist the show label as an Organization, Duration and
 * DatePublished carried over, and a freshly computed checksum. The
 * podcast_episodes row is removed once the library record exists — the file
 * is now an ordinary library track and is no longer subject to the podcast
 * sweep.
 *
 * The library record is created before the file is moved; if the move then
 * fails, the just-created record is rolled back so the episode is left
 * exactly as it was (still a podcast episode, file untouched).
 *
 * Parameters:
 *   id          (string) — the podcast_episodes id to migrate
 *   destination (string) — destination folder, relative to AudioDir
 *
 * Returns:
 *   string — the new audio_files id
 *   error  — non-nil if the episode does not exist, has no downloaded file,
 *            destination escapes AudioDir, or the move/database step fails
 *
 * Example:
 *   audioID, err := col.MigratePodcastEpisodeToLibrary(id, "Classical/Lectures")
 */
func (c *Collection) MigratePodcastEpisodeToLibrary(id, destination string) (string, error) {
	if !c.isOpen {
		return "", fmt.Errorf("collection is not open")
	}
	ep, err := c.getPodcastEpisode(id)
	if err != nil {
		return "", err
	}
	if ep.ContentURL == "" {
		return "", fmt.Errorf("podcast episode %s has no downloaded file to migrate", id)
	}

	oldPath := filepath.Join(c.cfg.AudioDir, ep.ContentURL)
	checksum, err := computeSHA256(oldPath)
	if err != nil {
		return "", err
	}

	destDir, err := c.resolveUnderAudioDir(destination)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("creating %s: %w", destDir, err)
	}

	filename := filepath.Base(oldPath)
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	newPath := uniquePodcastFilePath(destDir, base, ext, ep.ID)

	relContentURL, err := filepath.Rel(c.cfg.AudioDir, newPath)
	if err != nil {
		return "", fmt.Errorf("computing relative path for %s: %w", newPath, err)
	}

	info := AudioInfo{
		SchemaType:        "AudioObject",
		Name:              ep.Title,
		ContentURL:        relContentURL,
		EncodingFormat:    getMIMEType(newPath),
		Duration:          ep.Duration,
		DatePublished:     ep.Published,
		InAlbum:           ep.ShowLabel,
		ByArtist:          []Agent{{Type: "Organization", Name: ep.ShowLabel}},
		Checksum:          checksum,
		ChecksumAlgorithm: "sha256",
	}

	newID, err := c.Create(info)
	if err != nil {
		return "", fmt.Errorf("creating library record for podcast episode %s: %w", id, err)
	}

	if err := moveFile(oldPath, newPath); err != nil {
		if delErr := c.Delete(newID); delErr != nil {
			return "", fmt.Errorf("moving %s to %s: %w (rollback also failed: %v)", oldPath, newPath, err, delErr)
		}
		return "", fmt.Errorf("moving %s to %s: %w", oldPath, newPath, err)
	}

	if _, err := c.db.Exec(`DELETE FROM podcast_episodes WHERE id = ?`, id); err != nil {
		return "", fmt.Errorf("removing podcast episode %s after migration: %w", id, err)
	}
	return newID, nil
}
