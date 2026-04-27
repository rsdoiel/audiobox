package audioinfo

import (
	"database/sql"
	"fmt"
	"os"
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
 *   MusicDir    (string) — root directory of the audio file tree to scan
 *   Htdocs      (string) — document root for the web server's static files (optional)
 *   Port        (int)    — port the web server listens on; 0 means default (8010)
 *   CORSOrigin  (string) — Access-Control-Allow-Origin value; "" means "*", "off" disables CORS
 *
 * Example:
 *   cfg := audioinfo.CollectionConfig{
 *     Name:        "mymusic",
 *     Description: "My personal music archive",
 *     Database:    "mymusic.db",
 *     MusicDir:    "/home/alice/Music",
 *     Htdocs:      "/home/alice/music-ui",
 *     Port:        8010,
 *   }
 */
type CollectionConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Database    string `yaml:"database"`
	MusicDir    string `yaml:"musicDir"`
	Htdocs      string `yaml:"htdocs,omitempty"`
	Port        int    `yaml:"port,omitempty"`
	CORSOrigin  string `yaml:"corsOrigin,omitempty"`
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
 *   cfg, err := audioinfo.LoadConfig("mymusic.yaml")
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
 *   err := audioinfo.SaveConfig("mymusic.yaml", cfg)
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

/** NewCollection creates a new audio collection: it writes a YAML config file and initialises
 * the SQLite3 database in the current working directory.
 *
 * The YAML file is named <name>.yaml and the database <name>.db, both placed in the
 * current working directory.  The database path stored in the YAML is relative to the
 * YAML file's location so the pair can be moved together.
 *
 * Parameters:
 *   name        (string) — short collection identifier; used as the basename for both files
 *   musicDir     (string) — root directory of the audio file tree
 *   description (string) — human-readable description of the collection
 *
 * Returns:
 *   *Collection — open, ready-to-use collection
 *   error       — non-nil on any failure; partial files are cleaned up before returning
 *
 * Example:
 *   col, err := audioinfo.NewCollection("mymusic", "/home/alice/Music", "Alice's archive")
 *   if err != nil { log.Fatal(err) }
 *   defer col.Close()
 */
func NewCollection(name, musicDir, description string) (*Collection, error) {
	expanded, err := expandHome(musicDir)
	if err != nil {
		return nil, err
	}
	absRoot, err := filepath.Abs(expanded)
	if err != nil {
		return nil, fmt.Errorf("resolving musicDir %q: %w", musicDir, err)
	}

	dbFile := name + ".db"
	yamlFile := name + ".yaml"

	cfg := CollectionConfig{
		Name:        name,
		Description: description,
		Database:    dbFile,
		MusicDir:     absRoot,
	}

	db, err := openDB(dbFile)
	if err != nil {
		return nil, err
	}
	if err := initSchema(db); err != nil {
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
 *   col, err := audioinfo.LoadCollection("mymusic.yaml")
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

	// Resolve musicDir: expand tilde, then treat relative paths as relative to the YAML file.
	cfg.MusicDir, err = expandHome(cfg.MusicDir)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(cfg.MusicDir) {
		cfg.MusicDir = filepath.Join(filepath.Dir(absYAML), cfg.MusicDir)
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

	return &Collection{db: db, cfg: cfg, cfgPath: absYAML, isOpen: true}, nil
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

// initSchema creates the audio_files table and FTS5 search_index if they do not already exist.
// It is safe to call on an existing database (all statements use IF NOT EXISTS).
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
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("initialising schema: %w", err)
		}
	}
	return nil
}
