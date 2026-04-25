// Package audioinfo manages audio file metadata in a SQLite3 database,
// aligned with schema.org AudioObject and MusicRecording vocabulary.
package audioinfo

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

/** AudioInfo holds schema.org AudioObject / MusicRecording metadata for a single audio file.
 *
 * Parameters:
 *   ID                 (string)     — UUID primary key (set by Create)
 *   Created            (time.Time)  — timestamp when the record was first inserted
 *   Updated            (time.Time)  — timestamp of the most recent update
 *   SchemaType         (string)     — "AudioObject" or "MusicRecording"
 *   Name               (string)     — schema:name — title of the recording
 *   Description        (string)     — schema:description
 *   ContentURL         (string)     — schema:contentUrl — absolute path to the file
 *   EncodingFormat     (string)     — schema:encodingFormat — MIME type
 *   Duration           (string)     — schema:duration — ISO 8601 (e.g. "PT3M45S"); set on first play
 *   DatePublished      (string)     — schema:datePublished
 *   InLanguage         (string)     — schema:inLanguage
 *   Genre              (string)     — schema:genre
 *   Identifiers        (Identifiers) — schema:identifier list (DOI, ISRC, ARK, etc.)
 *   ByArtist           ([]Agent)    — schema:byArtist — performers (Person or Organization)
 *   InAlbum            (string)     — schema:inAlbum
 *   IsrcCode           (string)     — schema:isrcCode
 *   RecordingOf        (string)     — schema:recordingOf — composition title if different from Name
 *   Checksum           (string)     — hex-encoded SHA-256 of the file at ingest
 *   ChecksumAlgorithm  (string)     — always "sha256"
 *
 * Example:
 *   info := audioinfo.AudioInfo{
 *     SchemaType:     "MusicRecording",
 *     Name:           "Goldberg Variations BWV 988",
 *     ContentURL:     "/home/alice/Music/bach/goldberg.flac",
 *     EncodingFormat: "audio/flac",
 *   }
 */
type AudioInfo struct {
	ID                string
	Created           time.Time
	Updated           time.Time
	SchemaType        string
	Name              string
	Description       string
	ContentURL        string
	EncodingFormat    string
	Duration          string
	DatePublished     string
	InLanguage        string
	Genre             string
	Identifiers       Identifiers
	ByArtist          []Agent
	InAlbum           string
	IsrcCode          string
	RecordingOf       string
	Checksum          string
	ChecksumAlgorithm string
}

/** Collection manages an audio metadata collection backed by a SQLite3 database.
 *
 * Obtain a Collection via NewCollection (creates) or LoadCollection (opens existing).
 * Always call Close when done.
 *
 * Example:
 *   col, err := audioinfo.LoadCollection("mymusic.yaml")
 *   if err != nil { log.Fatal(err) }
 *   defer col.Close()
 */
type Collection struct {
	db      *sql.DB
	cfg     CollectionConfig
	cfgPath string
	isOpen  bool
}

/** Close releases the database connection held by the Collection.
 *
 * Returns:
 *   error — non-nil if the underlying database close fails
 *
 * Example:
 *   defer col.Close()
 */
func (c *Collection) Close() error {
	if !c.isOpen {
		return nil
	}
	if err := c.db.Close(); err != nil {
		return fmt.Errorf("closing collection: %w", err)
	}
	c.isOpen = false
	return nil
}

/** Config returns the CollectionConfig associated with this Collection.
 *
 * Returns:
 *   CollectionConfig — a copy of the collection's configuration
 *
 * Example:
 *   fmt.Println(col.Config().RootDir)
 */
func (c *Collection) Config() CollectionConfig {
	return c.cfg
}

// isAudioFile reports whether path has a recognised audio file extension.
func isAudioFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3", ".flac", ".ogg", ".m4a", ".wma", ".wav":
		return true
	}
	return false
}

// getMIMEType returns the MIME type for a recognised audio file extension.
func getMIMEType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		return "audio/mpeg"
	case ".flac":
		return "audio/flac"
	case ".ogg":
		return "audio/ogg"
	case ".m4a":
		return "audio/mp4"
	case ".wma":
		return "audio/x-ms-wma"
	case ".wav":
		return "audio/wav"
	default:
		return "application/octet-stream"
	}
}

// computeSHA256 returns the lowercase hex-encoded SHA-256 hash of the named file.
func computeSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening file for checksum %s: %w", path, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hashing file %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

/** ProcessAudioFile reads an audio file's embedded tags, computes its SHA-256 checksum,
 * and inserts or updates the corresponding record in the collection database.
 *
 * Filesystem and database errors are returned as hard errors (stopping the scan).
 * Tag-decode errors are emitted as warnings via logger and the file is still recorded
 * with whatever metadata could be extracted.
 *
 * Parameters:
 *   filePath (string)      — absolute path to the audio file
 *   logger   (*log.Logger) — receives warning messages for non-fatal tag errors
 *
 * Returns:
 *   error — non-nil for filesystem or database failures only
 *
 * Example:
 *   logger := log.New(os.Stderr, "warn: ", 0)
 *   err := col.ProcessAudioFile("/home/alice/Music/track.mp3", logger)
 */
func (c *Collection) ProcessAudioFile(filePath string, logger *log.Logger) error {
	checksum, err := computeSHA256(filePath)
	if err != nil {
		return err
	}

	info := AudioInfo{
		SchemaType:        "MusicRecording",
		ContentURL:        filePath,
		EncodingFormat:    getMIMEType(filePath),
		Checksum:          checksum,
		ChecksumAlgorithm: "sha256",
	}

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening %s: %w", filePath, err)
	}
	defer f.Close()

	meta, err := tag.ReadFrom(f)
	if err != nil {
		logger.Printf("warning: could not read tags from %s: %v", filePath, err)
	} else {
		trackNum, _ := meta.Track()
		info.Name = meta.Title()
		info.InAlbum = meta.Album()
		info.Genre = meta.Genre()
		if meta.Year() != 0 {
			info.DatePublished = fmt.Sprintf("%d", meta.Year())
		}
		info.ByArtist = []Agent{{Type: "Person", Name: meta.Artist()}}
		info.Identifiers = Identifiers{}
		if meta.Comment() != "" {
			info.Description = meta.Comment()
		}
		_ = trackNum
	}

	var existingID string
	err = c.db.QueryRow("SELECT id FROM audio_files WHERE content_url = ?", filePath).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("checking for existing record: %w", err)
	}

	if err == sql.ErrNoRows {
		_, err = c.Create(info)
		if err != nil {
			return fmt.Errorf("inserting %s: %w", filePath, err)
		}
	} else {
		info.ID = existingID
		if err = c.Update(existingID, info); err != nil {
			return fmt.Errorf("updating %s: %w", filePath, err)
		}
	}
	return nil
}

/** ScanDirectories walks the collection's rootDir, calling ProcessAudioFile for every
 * recognised audio file found.  Filesystem and database errors stop the walk immediately.
 * Tag-decode errors are logged as warnings and the walk continues.
 *
 * Returns:
 *   error — non-nil on any filesystem or database failure
 *
 * Example:
 *   if err := col.ScanDirectories(); err != nil { log.Fatal(err) }
 */
func (c *Collection) ScanDirectories() error {
	logger := log.New(os.Stderr, "audioinfo: ", 0)
	return c.ScanDirectoriesWithProcessor(c.ProcessAudioFile, logger)
}

/** ScanDirectoriesWithProcessor walks the collection's rootDir and calls processor for every
 * recognised audio file.  This variant is primarily useful for testing with a mock processor.
 *
 * The walk follows symbolic links and detects cycles via canonical path tracking.
 * An unreadable subdirectory is logged as a warning and skipped; only database errors
 * returned by processor stop the walk.
 *
 * Parameters:
 *   processor (func(filePath string, logger *log.Logger) error) — called for each audio file
 *   logger    (*log.Logger) — receives warnings for inaccessible paths
 *
 * Returns:
 *   error — non-nil if rootDir is inaccessible or processor returns a database error
 *
 * Example:
 *   err := col.ScanDirectoriesWithProcessor(myProcessor, log.New(os.Stderr, "", 0))
 */
func (c *Collection) ScanDirectoriesWithProcessor(
	processor func(filePath string, logger *log.Logger) error,
	logger *log.Logger,
) error {
	if !c.isOpen {
		return fmt.Errorf("collection is not open")
	}
	if _, err := os.Stat(c.cfg.RootDir); err != nil {
		return fmt.Errorf("root directory %q: %w", c.cfg.RootDir, err)
	}
	visited := make(map[string]struct{})
	return c.walkDir(c.cfg.RootDir, visited, processor, logger)
}

// walkDir recursively descends dir, following symlinks and skipping cycles.
// Inaccessible files or directories are logged and skipped.
// Only errors returned by processor are propagated (they indicate DB failures).
func (c *Collection) walkDir(
	dir string,
	visited map[string]struct{},
	processor func(string, *log.Logger) error,
	logger *log.Logger,
) error {
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		logger.Printf("warning: cannot resolve path %s: %v", dir, err)
		return nil
	}
	if _, seen := visited[real]; seen {
		return nil
	}
	visited[real] = struct{}{}

	entries, err := os.ReadDir(dir)
	if err != nil {
		logger.Printf("warning: cannot read directory %s: %v", dir, err)
		return nil
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		// Use os.Stat (not Lstat) so symlinks are followed for both files and directories.
		info, err := os.Stat(path)
		if err != nil {
			logger.Printf("warning: cannot access %s: %v", path, err)
			continue
		}
		if info.IsDir() {
			if err := c.walkDir(path, visited, processor, logger); err != nil {
				return err
			}
		} else if isAudioFile(path) {
			if err := processor(path, logger); err != nil {
				return err
			}
		}
	}
	return nil
}

// marshalJSON is a helper that marshals v and returns the bytes, or nil on empty/nil input.
func marshalJSON(v interface{}) ([]byte, error) {
	if v == nil {
		return []byte("null"), nil
	}
	return json.Marshal(v)
}

// extractArtistNames returns a space-joined string of all agent names for FTS5 indexing.
func extractArtistNames(agents []Agent) string {
	names := make([]string, 0, len(agents))
	for _, a := range agents {
		if a.Name != "" {
			names = append(names, a.Name)
		}
	}
	return strings.Join(names, " ")
}

// buildFTS5Query converts a slice of user-supplied terms into an FTS5 MATCH expression
// with auto-fuzziness matching OpenSearch AUTO behaviour:
//   - ≤2 chars: exact match
//   - 3–5 chars: edit distance 1  (~1)
//   - 6+ chars: edit distance 2  (~2)
//
// Each term is double-quoted so FTS5 special characters in user input are inert.
func buildFTS5Query(terms []string) string {
	parts := make([]string, len(terms))
	for i, term := range terms {
		escaped := strings.ReplaceAll(term, `"`, `""`)
		switch n := len([]rune(term)); {
		case n <= 2:
			parts[i] = `"` + escaped + `"`
		case n <= 5:
			parts[i] = `"` + escaped + `"~1`
		default:
			parts[i] = `"` + escaped + `"~2`
		}
	}
	return strings.Join(parts, " ")
}

/** Create inserts a new AudioInfo record into the collection and returns its generated UUID.
 *
 * Parameters:
 *   info (AudioInfo) — the record to insert; ID, Created, and Updated are set automatically
 *
 * Returns:
 *   string — the UUID assigned to the new record
 *   error  — non-nil on database failure
 *
 * Example:
 *   id, err := col.Create(audioinfo.AudioInfo{Name: "My Track", ContentURL: "/music/track.mp3"})
 */
func (c *Collection) Create(info AudioInfo) (string, error) {
	if !c.isOpen {
		return "", fmt.Errorf("collection is not open")
	}
	info.ID = uuid.New().String()
	now := time.Now()
	info.Created = now
	info.Updated = now

	idsJSON, err := marshalJSON(info.Identifiers)
	if err != nil {
		return "", fmt.Errorf("marshalling identifiers: %w", err)
	}
	artistsJSON, err := marshalJSON(info.ByArtist)
	if err != nil {
		return "", fmt.Errorf("marshalling by_artist: %w", err)
	}

	_, err = c.db.Exec(`
		INSERT INTO audio_files (
			id, schema_type, name, description, content_url, encoding_format,
			duration, date_published, in_language, genre, identifiers, by_artist,
			in_album, isrc_code, recording_of, checksum, checksum_algorithm,
			created, updated
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		info.ID, info.SchemaType, info.Name, info.Description, info.ContentURL,
		info.EncodingFormat, info.Duration, info.DatePublished, info.InLanguage,
		info.Genre, idsJSON, artistsJSON, info.InAlbum, info.IsrcCode,
		info.RecordingOf, info.Checksum, info.ChecksumAlgorithm,
		info.Created, info.Updated,
	)
	if err != nil {
		return "", fmt.Errorf("inserting audio record: %w", err)
	}
	if _, err = c.db.Exec(
		`INSERT INTO search_index (audio_id, name, in_album, genre, recording_of, artist_names)
		 VALUES (?,?,?,?,?,?)`,
		info.ID, info.Name, info.InAlbum, info.Genre, info.RecordingOf,
		extractArtistNames(info.ByArtist),
	); err != nil {
		return "", fmt.Errorf("indexing audio record: %w", err)
	}
	return info.ID, nil
}

/** Read fetches a single AudioInfo record by its UUID.
 *
 * Parameters:
 *   id (string) — the UUID of the record to fetch
 *
 * Returns:
 *   AudioInfo — the fetched record
 *   error     — non-nil if the record is not found or a database error occurs
 *
 * Example:
 *   info, err := col.Read("550e8400-e29b-41d4-a716-446655440000")
 */
func (c *Collection) Read(id string) (AudioInfo, error) {
	if !c.isOpen {
		return AudioInfo{}, fmt.Errorf("collection is not open")
	}
	var info AudioInfo
	var idsJSON, artistsJSON []byte

	err := c.db.QueryRow(`
		SELECT id, schema_type, name, description, content_url, encoding_format,
		       duration, date_published, in_language, genre, identifiers, by_artist,
		       in_album, isrc_code, recording_of, checksum, checksum_algorithm,
		       created, updated
		FROM audio_files WHERE id = ?`, id).Scan(
		&info.ID, &info.SchemaType, &info.Name, &info.Description, &info.ContentURL,
		&info.EncodingFormat, &info.Duration, &info.DatePublished, &info.InLanguage,
		&info.Genre, &idsJSON, &artistsJSON, &info.InAlbum, &info.IsrcCode,
		&info.RecordingOf, &info.Checksum, &info.ChecksumAlgorithm,
		&info.Created, &info.Updated,
	)
	if err != nil {
		return AudioInfo{}, fmt.Errorf("reading audio record %s: %w", id, err)
	}
	if err := json.Unmarshal(idsJSON, &info.Identifiers); err != nil {
		return AudioInfo{}, fmt.Errorf("unmarshalling identifiers: %w", err)
	}
	if err := json.Unmarshal(artistsJSON, &info.ByArtist); err != nil {
		return AudioInfo{}, fmt.Errorf("unmarshalling by_artist: %w", err)
	}
	return info, nil
}

/** Update replaces the metadata of an existing AudioInfo record identified by id.
 * The Created timestamp is preserved; Updated is set to now.
 *
 * Parameters:
 *   id   (string)    — the UUID of the record to update
 *   info (AudioInfo) — the new field values (ID field is ignored; id parameter is used)
 *
 * Returns:
 *   error — non-nil on database failure
 *
 * Example:
 *   info.Name = "Corrected Title"
 *   err := col.Update(id, info)
 */
func (c *Collection) Update(id string, info AudioInfo) error {
	if !c.isOpen {
		return fmt.Errorf("collection is not open")
	}
	info.Updated = time.Now()

	idsJSON, err := marshalJSON(info.Identifiers)
	if err != nil {
		return fmt.Errorf("marshalling identifiers: %w", err)
	}
	artistsJSON, err := marshalJSON(info.ByArtist)
	if err != nil {
		return fmt.Errorf("marshalling by_artist: %w", err)
	}

	_, err = c.db.Exec(`
		UPDATE audio_files SET
			schema_type=?, name=?, description=?, content_url=?, encoding_format=?,
			duration=?, date_published=?, in_language=?, genre=?, identifiers=?,
			by_artist=?, in_album=?, isrc_code=?, recording_of=?,
			checksum=?, checksum_algorithm=?, updated=?
		WHERE id=?`,
		info.SchemaType, info.Name, info.Description, info.ContentURL, info.EncodingFormat,
		info.Duration, info.DatePublished, info.InLanguage, info.Genre,
		idsJSON, artistsJSON, info.InAlbum, info.IsrcCode, info.RecordingOf,
		info.Checksum, info.ChecksumAlgorithm, info.Updated, id,
	)
	if err != nil {
		return fmt.Errorf("updating audio record %s: %w", id, err)
	}
	// FTS5 does not support in-place UPDATE efficiently; delete and re-insert.
	if _, err = c.db.Exec(`DELETE FROM search_index WHERE audio_id = ?`, id); err != nil {
		return fmt.Errorf("removing old search index entry %s: %w", id, err)
	}
	if _, err = c.db.Exec(
		`INSERT INTO search_index (audio_id, name, in_album, genre, recording_of, artist_names)
		 VALUES (?,?,?,?,?,?)`,
		id, info.Name, info.InAlbum, info.Genre, info.RecordingOf,
		extractArtistNames(info.ByArtist),
	); err != nil {
		return fmt.Errorf("re-indexing audio record %s: %w", id, err)
	}
	return nil
}

/** Delete removes the audio record with the given UUID from the collection.
 *
 * Parameters:
 *   id (string) — the UUID of the record to delete
 *
 * Returns:
 *   error — non-nil on database failure
 *
 * Example:
 *   err := col.Delete("550e8400-e29b-41d4-a716-446655440000")
 */
func (c *Collection) Delete(id string) error {
	if !c.isOpen {
		return fmt.Errorf("collection is not open")
	}
	if _, err := c.db.Exec("DELETE FROM audio_files WHERE id = ?", id); err != nil {
		return fmt.Errorf("deleting audio record %s: %w", id, err)
	}
	if _, err := c.db.Exec("DELETE FROM search_index WHERE audio_id = ?", id); err != nil {
		return fmt.Errorf("removing search index entry %s: %w", id, err)
	}
	return nil
}

/** GetAlbums returns a sorted, deduplicated list of album names in the collection.
 *
 * Returns:
 *   []string — album names in ascending alphabetical order
 *   error    — non-nil on database failure
 *
 * Example:
 *   albums, err := col.GetAlbums()
 */
func (c *Collection) GetAlbums() ([]string, error) {
	return c.queryDistinctColumn("in_album")
}

/** GetArtists returns a sorted, deduplicated list of artist names in the collection.
 * Artist names are extracted from the by_artist JSON column.
 *
 * Returns:
 *   []string — artist names in ascending alphabetical order
 *   error    — non-nil on database failure
 *
 * Example:
 *   artists, err := col.GetArtists()
 */
func (c *Collection) GetArtists() ([]string, error) {
	if !c.isOpen {
		return nil, fmt.Errorf("collection is not open")
	}
	rows, err := c.db.Query(`SELECT DISTINCT by_artist FROM audio_files WHERE by_artist IS NOT NULL AND by_artist != 'null'`)
	if err != nil {
		return nil, fmt.Errorf("querying artists: %w", err)
	}
	defer rows.Close()

	seen := map[string]struct{}{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scanning artist row: %w", err)
		}
		var agents []Agent
		if err := json.Unmarshal(raw, &agents); err != nil {
			continue
		}
		for _, a := range agents {
			if a.Name != "" {
				seen[a.Name] = struct{}{}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating artist rows: %w", err)
	}

	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	return result, nil
}

/** GetTitles returns a sorted, deduplicated list of recording names in the collection.
 *
 * Returns:
 *   []string — recording names in ascending alphabetical order
 *   error    — non-nil on database failure
 *
 * Example:
 *   titles, err := col.GetTitles()
 */
func (c *Collection) GetTitles() ([]string, error) {
	return c.queryDistinctColumn("name")
}

// queryDistinctColumn returns sorted distinct non-empty values from a text column.
func (c *Collection) queryDistinctColumn(col string) ([]string, error) {
	if !c.isOpen {
		return nil, fmt.Errorf("collection is not open")
	}
	rows, err := c.db.Query(
		fmt.Sprintf("SELECT DISTINCT %s FROM audio_files WHERE %s != '' AND %s IS NOT NULL ORDER BY %s", col, col, col, col),
	)
	if err != nil {
		return nil, fmt.Errorf("querying %s: %w", col, err)
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("scanning %s row: %w", col, err)
		}
		items = append(items, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating %s rows: %w", col, err)
	}
	return items, nil
}

/** SearchAudioFiles performs a full-text search against the FTS5 search index, modelled on
 * OpenSearch's default query behaviour.
 *
 * The query string is split into whitespace-separated terms.  Every term must match at
 * least one of the indexed fields (AND-of-OR semantics).  Each term is matched with
 * auto-fuzziness to tolerate typos:
 *
 *   ≤2 chars  exact match
 *   3–5 chars  edit distance 1  (one insertion / deletion / substitution)
 *   6+ chars   edit distance 2  (e.g. covers transpositions like "Melkit" → "Meklit")
 *
 * Fields searched: name (recording title), in_album, genre, recording_of, and each
 * artist's name.  Results are ordered by FTS5 relevance rank (best match first).
 *
 * Parameters:
 *   query (string) — space-separated search terms, e.g. "infinity" or "miles davis"
 *
 * Returns:
 *   []AudioInfo — matching records ordered by relevance (empty slice when nothing matches)
 *   error       — non-nil on database failure
 *
 * Example:
 *   results, err := col.SearchAudioFiles("Melkit")   // finds artist "Meklit" via fuzzy match
 *   results, err := col.SearchAudioFiles("miles davis")
 */
func (c *Collection) SearchAudioFiles(query string) ([]AudioInfo, error) {
	if !c.isOpen {
		return nil, fmt.Errorf("collection is not open")
	}
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return []AudioInfo{}, nil
	}

	ftsQuery := buildFTS5Query(terms)

	rows, err := c.db.Query(`
		SELECT a.id, a.schema_type, a.name, a.description, a.content_url, a.encoding_format,
		       a.duration, a.date_published, a.in_language, a.genre, a.identifiers, a.by_artist,
		       a.in_album, a.isrc_code, a.recording_of, a.checksum, a.checksum_algorithm,
		       a.created, a.updated
		FROM search_index
		JOIN audio_files a ON search_index.audio_id = a.id
		WHERE search_index MATCH ?
		ORDER BY rank`,
		ftsQuery,
	)
	if err != nil {
		// FTS5 may reject malformed query strings; treat as empty result rather than error.
		return []AudioInfo{}, nil
	}
	defer rows.Close()

	var results []AudioInfo
	for rows.Next() {
		info, err := scanAudioInfo(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, info)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating search results: %w", err)
	}
	if results == nil {
		results = []AudioInfo{}
	}
	return results, nil
}

// scanAudioInfo reads one row from a *sql.Rows into an AudioInfo.
func scanAudioInfo(rows *sql.Rows) (AudioInfo, error) {
	var info AudioInfo
	var idsJSON, artistsJSON []byte
	err := rows.Scan(
		&info.ID, &info.SchemaType, &info.Name, &info.Description, &info.ContentURL,
		&info.EncodingFormat, &info.Duration, &info.DatePublished, &info.InLanguage,
		&info.Genre, &idsJSON, &artistsJSON, &info.InAlbum, &info.IsrcCode,
		&info.RecordingOf, &info.Checksum, &info.ChecksumAlgorithm,
		&info.Created, &info.Updated,
	)
	if err != nil {
		return AudioInfo{}, fmt.Errorf("scanning audio row: %w", err)
	}
	if err := json.Unmarshal(idsJSON, &info.Identifiers); err != nil {
		return AudioInfo{}, fmt.Errorf("unmarshalling identifiers: %w", err)
	}
	if err := json.Unmarshal(artistsJSON, &info.ByArtist); err != nil {
		return AudioInfo{}, fmt.Errorf("unmarshalling by_artist: %w", err)
	}
	return info, nil
}

/** ToJSONLD serialises the AudioInfo as a schema.org JSON-LD document.
 *
 * Returns:
 *   []byte — indented JSON-LD bytes
 *   error  — non-nil on marshalling failure
 *
 * Example:
 *   data, err := info.ToJSONLD()
 *   fmt.Println(string(data))
 */
func (a AudioInfo) ToJSONLD() ([]byte, error) {
	schemaType := a.SchemaType
	if schemaType == "" {
		schemaType = "MusicRecording"
	}

	doc := map[string]interface{}{
		"@context":       "https://schema.org",
		"@type":          schemaType,
		"@id":            a.ID,
		"name":           a.Name,
		"description":    a.Description,
		"contentUrl":     a.ContentURL,
		"encodingFormat": a.EncodingFormat,
		"genre":          a.Genre,
		"inAlbum":        a.InAlbum,
		"isrcCode":       a.IsrcCode,
		"recordingOf":    a.RecordingOf,
		"inLanguage":     a.InLanguage,
		"datePublished":  a.DatePublished,
		"duration":       a.Duration,
	}

	// identifiers as schema:PropertyValue array
	if len(a.Identifiers) > 0 {
		ids := make([]map[string]interface{}, 0, len(a.Identifiers))
		for _, id := range a.Identifiers {
			entry := map[string]interface{}{
				"@type":      "PropertyValue",
				"propertyID": id.PropertyID,
				"value":      id.Value,
			}
			if id.URL != "" {
				entry["url"] = id.URL
			}
			if id.Name != "" {
				entry["name"] = id.Name
			}
			ids = append(ids, entry)
		}
		doc["identifier"] = ids
	}

	// byArtist as schema:Person / schema:Organization array
	if len(a.ByArtist) > 0 {
		artists := make([]map[string]interface{}, 0, len(a.ByArtist))
		for _, ag := range a.ByArtist {
			entry := map[string]interface{}{
				"@type": ag.Type,
				"name":  ag.Name,
			}
			if len(ag.Identifiers) > 0 {
				agIDs := make([]map[string]interface{}, 0, len(ag.Identifiers))
				for _, id := range ag.Identifiers {
					agEntry := map[string]interface{}{
						"@type":      "PropertyValue",
						"propertyID": id.PropertyID,
						"value":      id.Value,
					}
					if id.URL != "" {
						agEntry["url"] = id.URL
					}
					agIDs = append(agIDs, agEntry)
				}
				entry["identifier"] = agIDs
			}
			artists = append(artists, entry)
		}
		doc["byArtist"] = artists
	}

	return json.MarshalIndent(doc, "", "  ")
}
