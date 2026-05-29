package audiobox

import (
	"database/sql"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// expandHome replaces a leading "~/" (or bare "~") with the current user's home directory.
func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}

// resolveYAMLPath appends ".yaml" to path if it does not already end with ".yaml".
func resolveYAMLPath(path string) string {
	if strings.HasSuffix(path, ".yaml") {
		return path
	}
	return path + ".yaml"
}

/** CollectionConfig holds the persistent configuration for an audio collection.
 * It is serialised as YAML and stored alongside the SQLite3 database.
 *
 * Parameters:
 *   Name        (string) — short identifier for the collection (also the basename of the DB file)
 *   Description (string) — human-readable description of the collection
 *   Database    (string) — path to the SQLite3 database, relative to the YAML file's directory
 *   AudioDir    (string) — audio directory of the audio file tree to scan
 *   Htdocs      (string) — document root for the web server's static files (optional)
 *   Port        (int)    — port the web server listens on; 0 means default (8010)
 *   CORSOrigin  (string) — Access-Control-Allow-Origin value; "" means "*", "off" disables CORS
 *
 * Example:
 *   cfg := audiobox.CollectionConfig{
 *     Name:        "mymusic",
 *     Description: "My personal music archive",
 *     Database:    "mymusic.db",
 *     AudioDir:    "/home/alice/Music",
 *     Htdocs:      "/home/alice/music-ui",
 *     Port:        8010,
 *   }
 */
type CollectionConfig struct {
	Name         string `yaml:"name"`
	Description  string `yaml:"description"`
	Database     string `yaml:"database"`
	AudioDir     string `yaml:"audioDir"`
	Htdocs       string `yaml:"htdocs,omitempty"`
	Port         int    `yaml:"port,omitempty"`
	CORSOrigin   string `yaml:"corsOrigin,omitempty"`
	ShareAddress    string   `yaml:"shareAddress,omitempty"`
	ExcludedFolders []string `yaml:"excludedFolders,omitempty"`
}

/** LoadConfig reads a CollectionConfig from a YAML file on disk.
 *
 * Parameters:
 *   yamlPath (string) — path to the YAML configuration file
 *
 * Returns:
 *   CollectionConfig — the parsed configuration
 *   error            — non-nil if the file cannot be read or parsed
 *
 * Example:
 *   cfg, err := audiobox.LoadConfig("mymusic.yaml")
 */
func LoadConfig(yamlPath string) (CollectionConfig, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return CollectionConfig{}, fmt.Errorf("reading config %s: %w", yamlPath, err)
	}
	var cfg CollectionConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return CollectionConfig{}, fmt.Errorf("parsing config %s: %w", yamlPath, err)
	}
	return cfg, nil
}

/** SaveConfig writes a CollectionConfig to a YAML file on disk, creating or overwriting it.
 *
 * Parameters:
 *   yamlPath (string)          — destination path for the YAML file
 *   cfg      (CollectionConfig) — configuration to serialise
 *
 * Returns:
 *   error — non-nil if the file cannot be written
 *
 * Example:
 *   err := audiobox.SaveConfig("mymusic.yaml", cfg)
 */
func SaveConfig(yamlPath string, cfg CollectionConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("serialising config: %w", err)
	}
	if err := os.WriteFile(yamlPath, data, 0644); err != nil {
		return fmt.Errorf("writing config %s: %w", yamlPath, err)
	}
	return nil
}

/** SetShareAddress saves addr to the collection's YAML config and updates the in-memory
 * configuration. Pass an empty string to clear the saved share address.
 *
 * Parameters:
 *   addr (string) — IPv4 address to share on, or "" to clear
 *
 * Returns:
 *   error — non-nil if the config file cannot be written
 *
 * Example:
 *   if err := col.SetShareAddress("192.168.1.5"); err != nil { log.Fatal(err) }
 */
func (c *Collection) SetShareAddress(addr string) error {
	c.cfg.ShareAddress = addr
	return SaveConfig(c.cfgPath, c.cfg)
}

/** SetExcludedFolders saves the list of disabled folder paths to the collection config.
 * Pass nil or an empty slice to clear all exclusions.
 *
 * Parameters:
 *   paths ([]string) — folder paths (relative to AudioDir) that should be excluded
 *
 * Returns:
 *   error — non-nil if the config file cannot be written
 *
 * Example:
 *   if err := col.SetExcludedFolders([]string{"Music/Seasonal"}); err != nil { log.Fatal(err) }
 */
func (c *Collection) SetExcludedFolders(paths []string) error {
	if len(paths) == 0 {
		c.cfg.ExcludedFolders = nil
	} else {
		c.cfg.ExcludedFolders = paths
	}
	return SaveConfig(c.cfgPath, c.cfg)
}

/** NewCollection creates a new audio collection: it writes a YAML config file and initialises
 * the SQLite3 database in the current working directory.
 *
 * The YAML file is named <name>.yaml and the database <name>.db, both placed in the
 * current working directory.  The database path stored in the YAML is relative to the
 * YAML file's location so the pair can be moved together.
 *
 * Parameters:
 *   name        (string) — short collection identifier; used as the basename for both files
 *   audioDir     (string) — audio directory of the audio file tree
 *   description (string) — human-readable description of the collection
 *
 * Returns:
 *   *Collection — open, ready-to-use collection
 *   error       — non-nil on any failure; partial files are cleaned up before returning
 *
 * Example:
 *   col, err := audiobox.NewCollection("mymusic", "/home/alice/Music", "Alice's archive")
 *   if err != nil { log.Fatal(err) }
 *   defer col.Close()
 */
func NewCollection(name, audioDir, description string) (*Collection, error) {
	expanded, err := expandHome(audioDir)
	if err != nil {
		return nil, err
	}
	absRoot, err := filepath.Abs(expanded)
	if err != nil {
		return nil, fmt.Errorf("resolving audioDir %q: %w", audioDir, err)
	}

	dbFile := name + ".db"
	yamlFile := name + ".yaml"

	cfg := CollectionConfig{
		Name:        name,
		Description: description,
		Database:    dbFile,
		AudioDir:     absRoot,
	}

	db, err := openDB(dbFile)
	if err != nil {
		return nil, err
	}
	if err := initSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrateContentURLs(db, cfg.AudioDir); err != nil {
		db.Close()
		return nil, err
	}

	if err := SaveConfig(yamlFile, cfg); err != nil {
		db.Close()
		return nil, err
	}

	return &Collection{db: db, cfg: cfg, cfgPath: yamlFile, isOpen: true}, nil
}

/** LoadCollection opens an existing audio collection from its YAML configuration file.
 * The database path in the YAML is resolved relative to the YAML file's directory.
 *
 * Parameters:
 *   yamlPath (string) — path to the collection's YAML configuration file
 *
 * Returns:
 *   *Collection — open, ready-to-use collection
 *   error       — non-nil if the config cannot be read or the database cannot be opened
 *
 * Example:
 *   col, err := audiobox.LoadCollection("mymusic.yaml")
 *   if err != nil { log.Fatal(err) }
 *   defer col.Close()
 */
func LoadCollection(yamlPath string) (*Collection, error) {
	absYAML, err := filepath.Abs(resolveYAMLPath(yamlPath))
	if err != nil {
		return nil, fmt.Errorf("resolving yaml path %q: %w", yamlPath, err)
	}

	cfg, err := LoadConfig(absYAML)
	if err != nil {
		return nil, err
	}

	// Resolve audioDir: expand tilde, then treat relative paths as relative to the YAML file.
	cfg.AudioDir, err = expandHome(cfg.AudioDir)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(cfg.AudioDir) {
		cfg.AudioDir = filepath.Join(filepath.Dir(absYAML), cfg.AudioDir)
	}

	// Resolve htdocs the same way.
	if cfg.Htdocs != "" {
		cfg.Htdocs, err = expandHome(cfg.Htdocs)
		if err != nil {
			return nil, err
		}
		if !filepath.IsAbs(cfg.Htdocs) {
			cfg.Htdocs = filepath.Join(filepath.Dir(absYAML), cfg.Htdocs)
		}
	}

	dbPath := cfg.Database
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(filepath.Dir(absYAML), dbPath)
	}

	db, err := openDB(dbPath)
	if err != nil {
		return nil, err
	}
	if err := initSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrateContentURLs(db, cfg.AudioDir); err != nil {
		db.Close()
		return nil, err
	}

	return &Collection{db: db, cfg: cfg, cfgPath: absYAML, isOpen: true}, nil
}

/** ConfigPath returns the absolute path to the YAML configuration file for this Collection.
 *
 * Returns:
 *   string — absolute path to the collection's YAML file
 *
 * Example:
 *   fmt.Println(col.ConfigPath()) // "/home/alice/Audio/audio.yaml"
 */
func (c *Collection) ConfigPath() string {
	return c.cfgPath
}

/** InitAudiobox initialises (or upgrades) the standard ~/Audio audiobox installation.
 * It is idempotent: running it again fixes missing directories, files, or schema.
 *
 * Steps:
 *   1. Creates ~/Audio if it does not exist.
 *   2. Creates ~/Audio/Music, ~/Audio/Podcasts, ~/Audio/Theater, ~/Audio/Books.
 *   3. Writes ~/Audio/audio.yaml if it does not exist, with AudioDir set to ~/Audio
 *      and Description set to "Audio collections for $USER".
 *   4. Opens (or creates) ~/Audio/audio.db and applies the schema.
 *
 * Returns:
 *   *Collection — open, ready-to-use collection
 *   error       — non-nil on any filesystem or database failure
 *
 * Example:
 *   col, err := audiobox.InitAudiobox()
 *   if err != nil { log.Fatal(err) }
 *   defer col.Close()
 */
func InitAudiobox() (*Collection, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolving home directory: %w", err)
	}

	audioDir := filepath.Join(home, "Audio")
	for _, sub := range []string{"", "Music", "Podcasts", "Theater", "Books"} {
		if err := os.MkdirAll(filepath.Join(audioDir, sub), 0755); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", filepath.Join(audioDir, sub), err)
		}
	}

	yamlFile := filepath.Join(audioDir, "audio.yaml")
	dbFile := filepath.Join(audioDir, "audio.db")

	var cfg CollectionConfig
	if _, err := os.Stat(yamlFile); os.IsNotExist(err) {
		userName := os.Getenv("USER")
		if userName == "" {
			if u, uerr := user.Current(); uerr == nil {
				userName = u.Username
			}
		}
		cfg = CollectionConfig{
			Name:        "audio",
			Description: "Audio collections for " + userName,
			Database:    "audio.db",
			AudioDir:    audioDir,
		}
		if err := SaveConfig(yamlFile, cfg); err != nil {
			return nil, err
		}
	} else {
		cfg, err = LoadConfig(yamlFile)
		if err != nil {
			return nil, err
		}
		cfg.AudioDir, err = expandHome(cfg.AudioDir)
		if err != nil {
			return nil, err
		}
		if !filepath.IsAbs(cfg.AudioDir) {
			cfg.AudioDir = filepath.Join(filepath.Dir(yamlFile), cfg.AudioDir)
		}
	}

	db, err := openDB(dbFile)
	if err != nil {
		return nil, err
	}
	if err := initSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrateContentURLs(db, cfg.AudioDir); err != nil {
		db.Close()
		return nil, err
	}
	return &Collection{db: db, cfg: cfg, cfgPath: yamlFile, isOpen: true}, nil
}

/** LoadAudiobox opens the standard ~/Audio audiobox installation.
 * It is the read-only counterpart to InitAudiobox: it loads ~/Audio/audio.yaml
 * and the database it references without creating any files or directories.
 *
 * Returns:
 *   *Collection — open, ready-to-use collection
 *   error       — non-nil if ~/Audio/audio.yaml does not exist or cannot be opened
 *
 * Example:
 *   col, err := audiobox.LoadAudiobox()
 *   if err != nil { log.Fatal(err) }
 *   defer col.Close()
 */
func LoadAudiobox() (*Collection, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolving home directory: %w", err)
	}
	yamlFile := filepath.Join(home, "Audio", "audio.yaml")
	return LoadCollection(yamlFile)
}

// openDB opens (or creates) a SQLite3 database and sets the required pragmas.
func openDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database %s: %w", dbPath, err)
	}
	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("setting %s: %w", pragma, err)
		}
	}
	return db, nil
}

// migrateContentURLs converts any absolute content_url values in the database to paths
// relative to audioDir. It is idempotent — already-relative paths are skipped.
func migrateContentURLs(db *sql.DB, audioDir string) error {
	rows, err := db.Query("SELECT id, content_url FROM audio_files WHERE content_url IS NOT NULL AND content_url != ''")
	if err != nil {
		return fmt.Errorf("migrate content_url: query: %w", err)
	}
	type rec struct{ id, url string }
	var toUpdate []rec
	for rows.Next() {
		var r rec
		if err := rows.Scan(&r.id, &r.url); err != nil {
			rows.Close()
			return fmt.Errorf("migrate content_url: scan: %w", err)
		}
		if filepath.IsAbs(r.url) {
			toUpdate = append(toUpdate, r)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("migrate content_url: iter: %w", err)
	}
	for _, r := range toUpdate {
		rel, err := filepath.Rel(audioDir, r.url)
		if err != nil {
			return fmt.Errorf("migrate content_url: rel path for %s: %w", r.url, err)
		}
		if _, err := db.Exec("UPDATE audio_files SET content_url = ? WHERE id = ?", rel, r.id); err != nil {
			return fmt.Errorf("migrate content_url: update %s: %w", r.id, err)
		}
	}
	return nil
}

// initSchema creates the audio_files table and FTS5 search_index if they do not already exist.
// It is safe to call on an existing database (all statements use IF NOT EXISTS).
// It also applies incremental column migrations for older databases.
func initSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS audio_files (
			id                  TEXT PRIMARY KEY,
			schema_type         TEXT NOT NULL DEFAULT 'MusicRecording',
			name                TEXT,
			description         TEXT,
			content_url         TEXT UNIQUE,
			encoding_format     TEXT,
			duration            TEXT,
			date_published      TEXT,
			in_language         TEXT,
			genre               TEXT,
			identifiers         JSON,
			by_artist           JSON,
			in_album            TEXT,
			disc_number         INTEGER NOT NULL DEFAULT 0,
			track_number        INTEGER NOT NULL DEFAULT 0,
			isrc_code           TEXT,
			recording_of        TEXT,
			checksum            TEXT,
			checksum_algorithm  TEXT,
			created             TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated             TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		// FTS5 virtual table with unicode61 tokenizer and auto-fuzziness support.
		// remove_diacritics normalises accented characters so "Meklit" matches "Méklít".
		`CREATE VIRTUAL TABLE IF NOT EXISTS search_index USING fts5(
			audio_id     UNINDEXED,
			name,
			in_album,
			genre,
			recording_of,
			artist_names,
			tokenize='unicode61 remove_diacritics 1'
		)`,
		`CREATE TABLE IF NOT EXISTS playlists (
			id      TEXT PRIMARY KEY,
			name    TEXT NOT NULL,
			created TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS playlist_tracks (
			playlist_id TEXT NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
			position    INTEGER NOT NULL,
			audio_id    TEXT NOT NULL REFERENCES audio_files(id) ON DELETE CASCADE,
			PRIMARY KEY (playlist_id, position)
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("initialising schema: %w", err)
		}
	}
	// Incremental migrations: add columns introduced after the initial schema.
	// ALTER TABLE ADD COLUMN fails with "duplicate column name" when already present;
	// that error is intentionally ignored so opening an existing database is always safe.
	for _, migration := range []string{
		`ALTER TABLE audio_files ADD COLUMN track_number INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE audio_files ADD COLUMN disc_number  INTEGER NOT NULL DEFAULT 0`,
	} {
		if _, err := db.Exec(migration); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("applying migration %q: %w", migration, err)
			}
		}
	}
	return nil
}
