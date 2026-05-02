// Package audiobox manages audio file metadata in a SQLite3 database,
// aligned with schema.org AudioObject and MusicRecording vocabulary.
package audiobox

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
	"regexp"
	"sort"
	"strconv"
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
 *   DiscNumber         (int)        — disc number within a multi-disc set (0 = unknown/single disc)
 *   TrackNumber        (int)        — track position within the disc (0 = unknown)
 *   IsrcCode           (string)     — schema:isrcCode
 *   RecordingOf        (string)     — schema:recordingOf — composition title if different from Name
 *   Checksum           (string)     — hex-encoded SHA-256 of the file at ingest
 *   ChecksumAlgorithm  (string)     — always "sha256"
 *
 * Example:
 *   info := audiobox.AudioInfo{
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
	DiscNumber        int
	TrackNumber       int
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
 *   col, err := audiobox.LoadCollection("mymusic.yaml")
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
 *   fmt.Println(col.Config().AudioDir)
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
		discNum, _ := meta.Disc()
		info.Name = meta.Title()
		info.InAlbum = meta.Album()
		info.DiscNumber = discNum
		info.TrackNumber = trackNum
		info.Genre = meta.Genre()
		if meta.Year() != 0 {
			info.DatePublished = fmt.Sprintf("%d", meta.Year())
		}
		info.ByArtist = []Agent{{Type: "Person", Name: meta.Artist()}}
		info.Identifiers = Identifiers{}
	}

	// Filename fallback: when tags did not supply disc or track numbers, parse
	// them from the filename using common naming conventions.
	if info.DiscNumber == 0 || info.TrackNumber == 0 {
		stem := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
		d, t := parseDiscTrackFromFilename(stem)
		if info.DiscNumber == 0 && d > 0 {
			info.DiscNumber = d
		}
		if info.TrackNumber == 0 && t > 0 {
			info.TrackNumber = t
		}
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

/** ScanDirectories walks the collection's audioDir, calling ProcessAudioFile for every
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
	logger := log.New(os.Stderr, "audiobox: ", 0)
	return c.ScanDirectoriesWithProcessor(c.ProcessAudioFile, logger)
}

/** ScanDirectoriesWithProcessor walks the collection's audioDir and calls processor for every
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
 *   error — non-nil if audioDir is inaccessible or processor returns a database error
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
	if _, err := os.Stat(c.cfg.AudioDir); err != nil {
		return fmt.Errorf("audio directory %q: %w", c.cfg.AudioDir, err)
	}
	visited := make(map[string]struct{})
	return c.walkDir(c.cfg.AudioDir, visited, processor, logger)
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

// queryToken is a single parsed unit of a search query.
// field is a canonical search_index column name ("name", "in_album", "genre",
// "recording_of", "artist_names") or "" for unscoped (all fields).
// pattern is the literal text or regex pattern; isRegex signals which.
type queryToken struct {
	field   string
	pattern string
	isRegex bool
}

// fieldAliases maps user-facing field names to search_index column names.
var fieldAliases = map[string]string{
	"title":        "name",
	"name":         "name",
	"album":        "in_album",
	"artist":       "artist_names",
	"genre":        "genre",
	"recording":    "recording_of",
	"recording_of": "recording_of",
}

// allSearchFields lists every indexed column in search_index (excludes audio_id UNINDEXED).
var allSearchFields = []string{"name", "in_album", "genre", "recording_of", "artist_names"}

/** parseQuery splits a query string into queryTokens.
 *
 * Supported syntax:
 *   word                — unscoped plain term
 *   "multi word phrase" — unscoped quoted phrase (spaces preserved)
 *   /pattern/           — unscoped regex (RE2 syntax, case-insensitive)
 *   field:word          — plain term scoped to a known field alias
 *   field:"multi word"  — quoted phrase scoped to a field
 *   field:/pattern/     — regex scoped to a known field alias
 *
 * Field aliases: title/name, album, artist, genre, recording/recording_of.
 * A colon is treated as a field scope only when the word before it is a known
 * alias; otherwise the entire token is a plain unscoped term.
 * A /pattern/ is regex only when a matching closing '/' exists.
 * A "phrase" is a quoted phrase only when a matching closing '"' exists.
 *
 * Parameters:
 *   query (string) — raw query string from the user
 *
 * Returns:
 *   []queryToken — parsed tokens; empty slice when query is blank
 *
 * Example:
 *   tokens := parseQuery(`artist:"Glenn Gould" Baroque`)
 *   // [{field:"artist_names", pattern:"Glenn Gould", isRegex:false},
 *   //  {field:"",             pattern:"Baroque",     isRegex:false}]
 */
func parseQuery(query string) []queryToken {
	tokens := make([]queryToken, 0)
	i, n := 0, len(query)
	for i < n {
		// Skip whitespace.
		for i < n && (query[i] == ' ' || query[i] == '\t') {
			i++
		}
		if i >= n {
			break
		}
		tok := queryToken{}

		// Look for a field: prefix — scan to the next ':', ' ', or '\t'.
		j := i
		for j < n && query[j] != ':' && query[j] != ' ' && query[j] != '\t' {
			j++
		}
		if j < n && query[j] == ':' && j > i {
			alias := strings.ToLower(query[i:j])
			if col, ok := fieldAliases[alias]; ok {
				tok.field = col
				i = j + 1 // advance past ':'
			}
		}
		if i >= n {
			break
		}

		switch query[i] {
		case '"':
			// Quoted phrase: read until the next '"'.
			if ci := strings.IndexByte(query[i+1:], '"'); ci >= 0 {
				tok.pattern = query[i+1 : i+1+ci]
				i = i + 1 + ci + 1
			} else {
				// No closing '"' — treat as a plain term including the opening '"'.
				start := i
				for i < n && query[i] != ' ' && query[i] != '\t' {
					i++
				}
				tok.pattern = query[start:i]
			}
		case '/':
			// Regex: read until the next '/'.
			if ci := strings.IndexByte(query[i+1:], '/'); ci >= 0 {
				tok.pattern = query[i+1 : i+1+ci]
				tok.isRegex = true
				i = i + 1 + ci + 1
			} else {
				// No closing '/' — treat as a plain term.
				start := i
				for i < n && query[i] != ' ' && query[i] != '\t' {
					i++
				}
				tok.pattern = query[start:i]
			}
		default:
			// Plain term: read until whitespace.
			start := i
			for i < n && query[i] != ' ' && query[i] != '\t' {
				i++
			}
			tok.pattern = query[start:i]
		}

		if tok.pattern != "" {
			tokens = append(tokens, tok)
		}
	}
	return tokens
}

// hasRegexToken reports whether any token in the slice is a regex.
func hasRegexToken(tokens []queryToken) bool {
	for _, t := range tokens {
		if t.isRegex {
			return true
		}
	}
	return false
}

// buildFTS5Query converts plain (non-regex) queryTokens into an FTS5 MATCH expression.
// Field-scoped tokens produce FTS5 field:term syntax; unscoped tokens are bare phrases.
// All terms are AND-ed (FTS5 default for space-separated expressions).
func buildFTS5Query(tokens []queryToken) string {
	parts := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		escaped := strings.ReplaceAll(tok.pattern, `"`, `""`)
		if tok.field != "" {
			parts = append(parts, tok.field+`:"`+escaped+`"`)
		} else {
			parts = append(parts, `"`+escaped+`"`)
		}
	}
	return strings.Join(parts, " ")
}

// levenshtein returns the edit distance between a and b (unicode-aware, case-insensitive).
func levenshtein(a, b string) int {
	ra, rb := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))
	la, lb := len(ra), len(rb)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			if ra[i-1] == rb[j-1] {
				curr[j] = prev[j-1]
			} else {
				curr[j] = 1 + minInt3(prev[j], curr[j-1], prev[j-1])
			}
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func minInt3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		return c
	}
	return a
}

// fuzzyThreshold returns the max allowed Levenshtein distance for a query term of length n,
// matching OpenSearch AUTO behaviour.
func fuzzyThreshold(n int) int {
	switch {
	case n <= 2:
		return 0
	case n <= 5:
		return 1
	default:
		return 2
	}
}

// collectFieldWords returns each searchable field's words (lowercased, punctuation-stripped)
// keyed by search_index column name.
func collectFieldWords(info AudioInfo) map[string][]string {
	splitWords := func(s string) []string {
		var out []string
		for _, w := range strings.Fields(s) {
			w = strings.ToLower(strings.Trim(w, `.,;:()[]"'`))
			if w != "" {
				out = append(out, w)
			}
		}
		return out
	}
	return map[string][]string{
		"name":         splitWords(info.Name),
		"in_album":     splitWords(info.InAlbum),
		"genre":        splitWords(info.Genre),
		"recording_of": splitWords(info.RecordingOf),
		"artist_names": splitWords(extractArtistNames(info.ByArtist)),
	}
}

// collectFieldText returns the full (unspilt) text of each searchable field,
// keyed by search_index column name.  Used by regexSearch.
func collectFieldText(info AudioInfo) map[string]string {
	return map[string]string{
		"name":         info.Name,
		"in_album":     info.InAlbum,
		"genre":        info.Genre,
		"recording_of": info.RecordingOf,
		"artist_names": extractArtistNames(info.ByArtist),
	}
}

// fuzzySearch scans all audio_files and returns those where every token fuzzy-matches
// at least one word in the token's target field(s) (AND semantics).
// Edit-distance thresholds match OpenSearch AUTO: 0 for ≤2 chars, 1 for 3–5, 2 for 6+.
func (c *Collection) fuzzySearch(tokens []queryToken) ([]AudioInfo, error) {
	rows, err := c.db.Query(`
		SELECT id, schema_type, name, description, content_url, encoding_format,
		       duration, date_published, in_language, genre, identifiers, by_artist,
		       in_album, disc_number, track_number, isrc_code, recording_of, checksum, checksum_algorithm,
		       created, updated
		FROM audio_files`)
	if err != nil {
		return nil, fmt.Errorf("fuzzy scan: %w", err)
	}
	defer rows.Close()

	var results []AudioInfo
	for rows.Next() {
		info, err := scanAudioInfo(rows)
		if err != nil {
			return nil, err
		}
		fieldWords := collectFieldWords(info)
		allMatch := true
		for _, tok := range tokens {
			thresh := fuzzyThreshold(len([]rune(tok.pattern)))
			var candidates []string
			if tok.field != "" {
				candidates = fieldWords[tok.field]
			} else {
				for _, f := range allSearchFields {
					candidates = append(candidates, fieldWords[f]...)
				}
			}
			matched := false
			for _, w := range candidates {
				if levenshtein(tok.pattern, w) <= thresh {
					matched = true
					break
				}
			}
			if !matched {
				allMatch = false
				break
			}
		}
		if allMatch {
			results = append(results, info)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fuzzy scan rows: %w", err)
	}
	if results == nil {
		results = []AudioInfo{}
	}
	return results, nil
}

// regexSearch applies compiled regex (and literal substring for plain tokens) against all
// records.  Plain tokens in a regex query are treated as case-insensitive substrings.
// Returns an error immediately if any regex pattern fails to compile.
func (c *Collection) regexSearch(tokens []queryToken) ([]AudioInfo, error) {
	compiled := make([]*regexp.Regexp, len(tokens))
	for i, tok := range tokens {
		pat := tok.pattern
		if !tok.isRegex {
			pat = regexp.QuoteMeta(pat)
		}
		re, err := regexp.Compile("(?i)" + pat)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %w", tok.pattern, err)
		}
		compiled[i] = re
	}

	rows, err := c.db.Query(`
		SELECT id, schema_type, name, description, content_url, encoding_format,
		       duration, date_published, in_language, genre, identifiers, by_artist,
		       in_album, disc_number, track_number, isrc_code, recording_of, checksum, checksum_algorithm,
		       created, updated
		FROM audio_files`)
	if err != nil {
		return nil, fmt.Errorf("regex scan: %w", err)
	}
	defer rows.Close()

	var results []AudioInfo
	for rows.Next() {
		info, err := scanAudioInfo(rows)
		if err != nil {
			return nil, err
		}
		fieldText := collectFieldText(info)
		allMatch := true
		for i, tok := range tokens {
			var fields []string
			if tok.field != "" {
				fields = []string{tok.field}
			} else {
				fields = allSearchFields
			}
			matched := false
			for _, f := range fields {
				if compiled[i].MatchString(fieldText[f]) {
					matched = true
					break
				}
			}
			if !matched {
				allMatch = false
				break
			}
		}
		if allMatch {
			results = append(results, info)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("regex scan rows: %w", err)
	}
	if results == nil {
		results = []AudioInfo{}
	}
	return results, nil
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
 *   id, err := col.Create(audiobox.AudioInfo{Name: "My Track", ContentURL: "/music/track.mp3"})
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
			in_album, disc_number, track_number, isrc_code, recording_of, checksum, checksum_algorithm,
			created, updated
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		info.ID, info.SchemaType, info.Name, info.Description, info.ContentURL,
		info.EncodingFormat, info.Duration, info.DatePublished, info.InLanguage,
		info.Genre, idsJSON, artistsJSON, info.InAlbum, info.DiscNumber, info.TrackNumber, info.IsrcCode,
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
		       in_album, disc_number, track_number, isrc_code, recording_of, checksum, checksum_algorithm,
		       created, updated
		FROM audio_files WHERE id = ?`, id).Scan(
		&info.ID, &info.SchemaType, &info.Name, &info.Description, &info.ContentURL,
		&info.EncodingFormat, &info.Duration, &info.DatePublished, &info.InLanguage,
		&info.Genre, &idsJSON, &artistsJSON, &info.InAlbum, &info.DiscNumber, &info.TrackNumber, &info.IsrcCode,
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
			by_artist=?, in_album=?, disc_number=?, track_number=?, isrc_code=?, recording_of=?,
			checksum=?, checksum_algorithm=?, updated=?
		WHERE id=?`,
		info.SchemaType, info.Name, info.Description, info.ContentURL, info.EncodingFormat,
		info.Duration, info.DatePublished, info.InLanguage, info.Genre,
		idsJSON, artistsJSON, info.InAlbum, info.DiscNumber, info.TrackNumber, info.IsrcCode, info.RecordingOf,
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

/** Sweep removes database records whose ContentURL no longer exists on disk.
 * It queries all content_url values in the collection, checks each path with
 * os.Stat, and deletes the record (and its search-index entry) for every file
 * that is missing.  The number of removed records is returned.
 *
 * Returns:
 *   removed (int)   — count of records deleted
 *   error           — non-nil on database failure
 *
 * Example:
 *   n, err := col.Sweep()
 *   fmt.Printf("removed %d stale records\n", n)
 */
func (c *Collection) Sweep() (int, error) {
	if !c.isOpen {
		return 0, fmt.Errorf("collection is not open")
	}
	rows, err := c.db.Query("SELECT id, content_url FROM audio_files WHERE content_url IS NOT NULL AND content_url != ''")
	if err != nil {
		return 0, fmt.Errorf("sweep: querying records: %w", err)
	}
	type record struct{ id, path string }
	var stale []record
	for rows.Next() {
		var r record
		if err := rows.Scan(&r.id, &r.path); err != nil {
			rows.Close()
			return 0, fmt.Errorf("sweep: scanning row: %w", err)
		}
		if _, err := os.Stat(r.path); os.IsNotExist(err) {
			stale = append(stale, r)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("sweep: iterating rows: %w", err)
	}
	for _, r := range stale {
		if err := c.Delete(r.id); err != nil {
			return 0, fmt.Errorf("sweep: deleting %s (%s): %w", r.id, r.path, err)
		}
	}
	return len(stale), nil
}

/** Count returns the total number of records in the collection.
 *
 * Returns:
 *   int  — number of rows in audio_files
 *   error — non-nil on database failure
 *
 * Example:
 *   n, err := col.Count()
 */
func (c *Collection) Count() (int, error) {
	if !c.isOpen {
		return 0, fmt.Errorf("collection is not open")
	}
	var n int
	if err := c.db.QueryRow("SELECT COUNT(*) FROM audio_files").Scan(&n); err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}
	return n, nil
}

// deslugify converts a directory-name slug into a human-readable album title.
// Hyphens and underscores are treated as space placeholders; other characters
// (including parentheses, apostrophes, digits) are preserved as-is.
// "801-Live-(American-Release)" → "801 Live (American Release)"
func deslugify(s string) string {
	s = strings.NewReplacer("-", " ", "_", " ").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

// reDiscParens matches "(Disc N) TT …"
// reDiscSlug  matches "Disc-N-TT…" (slugified multi-disc)
// reDiscTrack matches "N-TT " or "NN-TT-" (numeric disc-track with hyphen)
// reTrackOnly matches "TT " or "TT-" (track number at start, disc implied = 1)
var (
	reDiscParens = regexp.MustCompile(`^\(Disc\s+(\d+)\)\s+(\d+)`)
	reDiscSlug   = regexp.MustCompile(`(?i)^Disc-(\d+)-(\d+)`)
	reDiscTrack  = regexp.MustCompile(`^(\d+)-(\d{2})[\s\-]`)
	reTrackOnly  = regexp.MustCompile(`^(\d+)[\s\-]`)
)

// parseDiscTrackFromFilename extracts disc and track numbers from a filename stem
// (no extension) using common music library naming conventions.
// Returns (0, 0) when no recognisable pattern is found.
func parseDiscTrackFromFilename(stem string) (disc, track int) {
	if m := reDiscParens.FindStringSubmatch(stem); m != nil {
		disc, _ = strconv.Atoi(m[1])
		track, _ = strconv.Atoi(m[2])
		return
	}
	if m := reDiscSlug.FindStringSubmatch(stem); m != nil {
		disc, _ = strconv.Atoi(m[1])
		track, _ = strconv.Atoi(m[2])
		return
	}
	if m := reDiscTrack.FindStringSubmatch(stem); m != nil {
		disc, _ = strconv.Atoi(m[1])
		track, _ = strconv.Atoi(m[2])
		return
	}
	if m := reTrackOnly.FindStringSubmatch(stem); m != nil {
		disc = 1
		track, _ = strconv.Atoi(m[1])
		if track == 0 {
			// "00 …" prefix means unlisted/bonus; leave disc=1, track=0
			disc = 0
		}
		return
	}
	return 0, 0
}

/** GetAlbumEntries returns a sorted list of album entries derived from the
 * directory structure of the collection. Each unique directory that contains
 * audio files becomes one Album entry; the album name is produced by deslugifying
 * the directory's base name (replacing hyphens/underscores with spaces).
 *
 * This approach means the directory layout is the authoritative source for album
 * identity, which correctly handles releases whose embedded in_album tags are
 * incomplete or missing.  "801-Live-(American-Release)" becomes
 * "801 Live (American Release)" regardless of what the MP3 tags say.
 *
 * When two different directories deslugify to the same name, both DisplayNames
 * are qualified with their parent directory basename.
 *
 * Returns:
 *   []Album — sorted by DisplayName
 *   error   — non-nil on database failure
 *
 * Example:
 *   albums, err := col.GetAlbumEntries()
 *   for _, a := range albums {
 *     fmt.Println(a.DisplayName) // e.g. "801 Live (American Release)"
 *   }
 */
func (c *Collection) GetAlbumEntries() ([]Album, error) {
	if !c.isOpen {
		return nil, fmt.Errorf("collection is not open")
	}
	rows, err := c.db.Query(`
		SELECT DISTINCT content_url
		FROM audio_files
		WHERE content_url != '' AND content_url IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("querying album entries: %w", err)
	}
	defer rows.Close()

	dirSet := make(map[string]struct{})
	for rows.Next() {
		var contentURL string
		if err := rows.Scan(&contentURL); err != nil {
			return nil, fmt.Errorf("scanning content_url: %w", err)
		}
		dirSet[filepath.Dir(contentURL)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating content_urls: %w", err)
	}

	type raw struct{ dir, name string }
	raws := make([]raw, 0, len(dirSet))
	nameCounts := make(map[string]int)
	for dir := range dirSet {
		name := deslugify(filepath.Base(dir))
		raws = append(raws, raw{dir: dir, name: name})
		nameCounts[name]++
	}

	var albums []Album
	for _, r := range raws {
		displayName := r.name
		if nameCounts[r.name] > 1 {
			// Two directories produce the same deslugified name; qualify with parent.
			displayName = r.name + " [" + deslugify(filepath.Base(filepath.Dir(r.dir))) + "]"
		}
		albums = append(albums, Album{Name: r.name, DisplayName: displayName, Dir: r.dir})
	}
	sort.Slice(albums, func(i, j int) bool {
		return albums[i].DisplayName < albums[j].DisplayName
	})
	return albums, nil
}

// escapeLIKE escapes the SQLite LIKE special characters (\, %, _) in s
// so the result can safely be used as a literal prefix in a LIKE pattern.
func escapeLIKE(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

/** GetTracksByAlbum returns all audio tracks stored in the given album's directory,
 * sorted by disc number then track number then name. The directory is the sole
 * filter — the in_album tag is not consulted — so releases whose tags are
 * incomplete (e.g. "Live" instead of "801 Live") are handled correctly.
 *
 * Parameters:
 *   album (Album) — an entry obtained from GetAlbumEntries
 *
 * Returns:
 *   []AudioInfo — matching tracks in play order; empty slice when none found
 *   error       — non-nil on database failure
 *
 * Example:
 *   albums, _ := col.GetAlbumEntries()
 *   tracks, err := col.GetTracksByAlbum(albums[0])
 */
func (c *Collection) GetTracksByAlbum(album Album) ([]AudioInfo, error) {
	if !c.isOpen {
		return nil, fmt.Errorf("collection is not open")
	}
	pattern := escapeLIKE(album.Dir) + string(filepath.Separator) + "%"
	rows, err := c.db.Query(`
		SELECT id, schema_type, name, description, content_url, encoding_format,
		       duration, date_published, in_language, genre, identifiers, by_artist,
		       in_album, disc_number, track_number, isrc_code, recording_of, checksum, checksum_algorithm,
		       created, updated
		FROM audio_files
		WHERE content_url LIKE ? ESCAPE '\'
		ORDER BY disc_number, track_number, name`,
		pattern)
	if err != nil {
		return nil, fmt.Errorf("querying tracks for album %q: %w", album.DisplayName, err)
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
		return nil, fmt.Errorf("iterating tracks for album %q: %w", album.Name, err)
	}
	if results == nil {
		results = []AudioInfo{}
	}
	return results, nil
}

/** GetAlbums returns a sorted, deduplicated list of album display names in the collection.
 * Albums sharing the same name but stored in different directories appear as separate
 * entries with a folder qualifier appended (e.g. "801 Live [801-Live-UK]").
 * Prefer GetAlbumEntries when you need structured album data.
 *
 * Returns:
 *   []string — album display names in ascending order
 *   error    — non-nil on database failure
 *
 * Example:
 *   albums, err := col.GetAlbums()
 */
func (c *Collection) GetAlbums() ([]string, error) {
	entries, err := c.GetAlbumEntries()
	if err != nil {
		return nil, err
	}
	names := make([]string, len(entries))
	for i, a := range entries {
		names[i] = a.DisplayName
	}
	return names, nil
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
	sort.Strings(result)
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

/** SearchAudioFiles searches the collection using a flexible query syntax.
 *
 * Query tokens are whitespace-separated; every token must match (AND semantics).
 * Three token forms are supported:
 *
 *   word            plain term — FTS5 exact match, then Levenshtein fuzzy fallback
 *   /pattern/       RE2 regex — case-insensitive, matched as substring across all fields
 *   field:word      plain term scoped to a specific field
 *   field:/pattern/ regex scoped to a specific field
 *
 * Field aliases: title/name, album, artist, genre, recording/recording_of.
 *
 * For plain queries the search runs FTS5 first (fast, relevance-ranked) and falls back
 * to a full Levenshtein scan when FTS5 finds nothing.  Fuzzy thresholds mirror
 * OpenSearch AUTO: ≤2 chars exact, 3–5 chars edit-distance 1, 6+ edit-distance 2.
 *
 * Any token containing a /regex/ causes the entire query to use regexSearch (full scan).
 * An invalid regex pattern returns an error.
 *
 * Parameters:
 *   query (string) — search query, e.g. "Bach", "artist:Gould", "artist:/Glenn Gould/"
 *
 * Returns:
 *   []AudioInfo — matching records (empty slice when nothing matches)
 *   error       — non-nil on database failure or invalid regex
 *
 * Example:
 *   results, err := col.SearchAudioFiles("Melkit")                 // fuzzy — finds "Meklit"
 *   results, err := col.SearchAudioFiles("artist:Gould")           // field-scoped plain
 *   results, err := col.SearchAudioFiles("artist:/Glenn Gould/")   // field-scoped regex
 *   results, err := col.SearchAudioFiles("/Bach/ genre:Baroque")   // mixed
 */
func (c *Collection) SearchAudioFiles(query string) ([]AudioInfo, error) {
	if !c.isOpen {
		return nil, fmt.Errorf("collection is not open")
	}
	tokens := parseQuery(query)
	if len(tokens) == 0 {
		return []AudioInfo{}, nil
	}

	// Any regex token routes the whole query through regexSearch.
	if hasRegexToken(tokens) {
		return c.regexSearch(tokens)
	}

	// Plain tokens: FTS5 fast path with Levenshtein fuzzy fallback.
	ftsQuery := buildFTS5Query(tokens)
	rows, err := c.db.Query(`
		SELECT a.id, a.schema_type, a.name, a.description, a.content_url, a.encoding_format,
		       a.duration, a.date_published, a.in_language, a.genre, a.identifiers, a.by_artist,
		       a.in_album, a.disc_number, a.track_number, a.isrc_code, a.recording_of, a.checksum, a.checksum_algorithm,
		       a.created, a.updated
		FROM search_index
		JOIN audio_files a ON search_index.audio_id = a.id
		WHERE search_index MATCH ?
		ORDER BY rank`,
		ftsQuery,
	)
	if err != nil {
		// FTS5 may reject malformed query strings; fall through to fuzzy.
		return c.fuzzySearch(tokens)
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
	if len(results) == 0 {
		// FTS5 found nothing — try Levenshtein fuzzy scan as fallback.
		return c.fuzzySearch(tokens)
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
		&info.Genre, &idsJSON, &artistsJSON, &info.InAlbum, &info.DiscNumber, &info.TrackNumber, &info.IsrcCode,
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
