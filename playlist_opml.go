package audiobox

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"github.com/rsdoiel/opml"
)

/** ExportPlaylistOPML renders a saved playlist as an OPML 2.0 document. Each
 * track becomes one <outline> element: Text/Title carry the track name, URL
 * carries its content_url (forward-slash normalized) so a matching track can
 * be found again on import, Category carries the artist name(s), and
 * Description carries the album name.
 *
 * Parameters:
 *   id (string) — UUID of the playlist to export
 *
 * Returns:
 *   []byte — UTF-8 OPML XML, including the XML declaration
 *   error  — non-nil when the playlist does not exist or on database failure
 *
 * Example:
 *   data, err := col.ExportPlaylistOPML(id)
 */
func (c *Collection) ExportPlaylistOPML(id string) ([]byte, error) {
	name, err := c.playlistName(id)
	if err != nil {
		return nil, err
	}
	tracks, err := c.LoadPlaylist(id)
	if err != nil {
		return nil, err
	}

	doc := opml.New()
	doc.Head.Title = name
	doc.Head.Created = time.Now().UTC().Format(time.RFC1123Z)
	doc.Body.Outline = make([]*opml.Outline, 0, len(tracks))
	for _, t := range tracks {
		title := t.Name
		if title == "" {
			title = "(untitled)"
		}
		doc.Body.Outline = append(doc.Body.Outline, &opml.Outline{
			Text:        title,
			Type:        "audio",
			URL:         filepath.ToSlash(t.ContentURL),
			Category:    extractArtistNames(t.ByArtist),
			Description: t.InAlbum,
		})
	}

	body, err := opml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal OPML: %w", err)
	}
	return append([]byte(`<?xml version="1.0" encoding="UTF-8"?>`+"\n"), body...), nil
}

// playlistName looks up a playlist's display name by id, returning an error
// wrapping sql.ErrNoRows when it does not exist.
func (c *Collection) playlistName(id string) (string, error) {
	if !c.isOpen {
		return "", fmt.Errorf("collection is not open")
	}
	var name string
	err := c.db.QueryRow(`SELECT name FROM playlists WHERE id = ?`, id).Scan(&name)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("playlist %s: %w", id, sql.ErrNoRows)
	}
	if err != nil {
		return "", fmt.Errorf("playlist name: %w", err)
	}
	return name, nil
}

/** PlaylistImportResult summarizes the outcome of importing an OPML playlist.
 *
 * Parameters:
 *   ID       (string) — UUID of the newly created playlist
 *   Name     (string) — name the playlist was saved under
 *   Imported (int)    — outline entries successfully matched to a track in this collection
 *   Skipped  (int)    — outline entries with no url, or whose url matched no track
 *
 * Example:
 *   result, err := col.ImportPlaylistOPML(data, "")
 *   fmt.Printf("%d imported, %d skipped\n", result.Imported, result.Skipped)
 */
type PlaylistImportResult struct {
	ID       string
	Name     string
	Imported int
	Skipped  int
}

/** ImportPlaylistOPML parses an OPML document and creates a new playlist from
 * it. Each top-level <outline> is resolved back to a track by matching its
 * URL attribute against a track's content_url (both forward-slash
 * normalized); outlines with no URL, or whose URL matches no track in this
 * collection (e.g. the file was moved, or the playlist came from a
 * different machine), are skipped and counted rather than causing the
 * import to fail.
 *
 * Parameters:
 *   data ([]byte) — OPML XML
 *   name (string) — playlist name; when empty, the OPML document's head title is
 *                    used, falling back to "Imported Playlist"
 *
 * Returns:
 *   PlaylistImportResult — the created playlist's id/name and match counts
 *   error                — non-nil when the OPML can't be parsed or on database failure
 *
 * Example:
 *   result, err := col.ImportPlaylistOPML(data, "")
 */
func (c *Collection) ImportPlaylistOPML(data []byte, name string) (PlaylistImportResult, error) {
	if !c.isOpen {
		return PlaylistImportResult{}, fmt.Errorf("collection is not open")
	}
	doc, err := opml.Parse(data)
	if err != nil {
		return PlaylistImportResult{}, fmt.Errorf("parsing OPML: %w", err)
	}
	if name == "" && doc.Head != nil {
		name = doc.Head.Title
	}
	if name == "" {
		name = "Imported Playlist"
	}

	var trackIDs []string
	skipped := 0
	if doc.Body != nil {
		for _, ol := range doc.Body.Outline {
			if ol == nil {
				continue
			}
			trackID, err := c.findTrackByContentURL(ol.URL)
			if err != nil {
				skipped++
				continue
			}
			trackIDs = append(trackIDs, trackID)
		}
	}

	id, err := c.SavePlaylist(name, trackIDs)
	if err != nil {
		return PlaylistImportResult{}, err
	}
	return PlaylistImportResult{ID: id, Name: name, Imported: len(trackIDs), Skipped: skipped}, nil
}

// findTrackByContentURL looks up a track's id by content_url, comparing both
// sides with backslashes normalized to forward slashes so a playlist
// exported on one platform can still be matched on another.
func (c *Collection) findTrackByContentURL(url string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("empty url")
	}
	normalized := filepath.ToSlash(url)
	var id string
	err := c.db.QueryRow(
		`SELECT id FROM audio_files WHERE replace(content_url, '\', '/') = ?`, normalized,
	).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}
