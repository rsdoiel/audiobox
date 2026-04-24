package audioinfo

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/dhowden/tag"
	"github.com/google/uuid"
)

// AudioInfo holds metadata for an audio file.
type AudioInfo struct {
	ID              string
	Title           string
	Artist          string
	Album           string
	Path            string
	MIME            string
	Genre           string
	Description     string
	Created         time.Time
	Updated         time.Time
	PublicationDate string
	DOI             string
	ExtendedMetadata map[string]interface{}
}

// Collection manages a collection of audio files in a SQLite database.
type Collection struct {
	db          *sql.DB
	dbName      string
	rootDir     string
	description string
	isOpen      bool
}

// InitializeCollection initializes a new Collection and its database.
func InitializeCollection(dbName, rootDir, description string) (*Collection, error) {
	db, err := sql.Open("sqlite3", dbName+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS audio_files (
			id TEXT PRIMARY KEY,
			title TEXT,
			artist TEXT,
			album TEXT,
			path TEXT UNIQUE,
			mime_type TEXT,
			genre TEXT,
			description TEXT,
			created TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			publication_date TEXT,
			doi TEXT,
			extended_metadata JSON
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("error creating table: %v", err)
	}

	collection := &Collection{
		db:          db,
		dbName:      dbName,
		rootDir:     rootDir,
		description: description,
		isOpen:      true,
	}

	return collection, nil
}

// OpenCollection opens an existing Collection.
func OpenCollection(dbName string) (*Collection, error) {
	db, err := sql.Open("sqlite3", dbName+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	collection := &Collection{
		db:     db,
		dbName: dbName,
		isOpen: true,
	}

	return collection, nil
}

// Close closes the Collection and its database.
func (c *Collection) Close() error {
	if !c.isOpen {
		return nil
	}
	err := c.db.Close()
	if err != nil {
		return fmt.Errorf("error closing database: %v", err)
	}
	c.isOpen = false
	return nil
}

// isAudioFile checks if a file is an audio file based on its extension.
func isAudioFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3", ".flac", ".ogg", ".m4a", ".wma", ".wav":
		return true
	default:
		return false
	}
}

// getMIMEType returns the MIME type for a given file extension.
func getMIMEType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
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
		return "audio/unknown"
	}
}

// ProcessAudioFile processes an audio file and adds or updates it in the collection.
func (c *Collection) ProcessAudioFile(filePath string, logger *log.Logger) error {
	file, err := os.Open(filePath)
	if err != nil {
		logger.Printf("Error opening file %s: %v\n", filePath, err)
		return err
	}
	defer file.Close()

	meta, err := tag.ReadFrom(file)
	if err != nil {
		logger.Printf("Error reading audio tags for %s: %v\n", filePath, err)
		return err
	}

	mime := getMIMEType(filePath)
	trackNumber, _ := meta.Track()

	info := AudioInfo{
		Title:       meta.Title(),
		Artist:      meta.Artist(),
		Album:       meta.Album(),
		Path:        filePath,
		MIME:        mime,
		Genre:       meta.Genre(),
		Description: "",
		DOI:         "",
		ExtendedMetadata: map[string]interface{}{
			"year":         meta.Year(),
			"track_number": trackNumber,
		},
	}

	// Check if the file already exists in the database
	var existingID string
	err = c.db.QueryRow("SELECT id FROM audio_files WHERE path = ?", filePath).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error checking for existing file: %v", err)
	}

	if err == sql.ErrNoRows {
		// File does not exist in the database, create a new entry
		_, err = c.Create(info)
		if err != nil {
			return fmt.Errorf("error inserting %s into database: %v", filePath, err)
		}
	} else {
		// File exists in the database, update the existing entry
		info.ID = existingID
		err = c.Update(existingID, info)
		if err != nil {
			return fmt.Errorf("error updating %s in database: %v", filePath, err)
		}
	}

	return nil
}

// ScanDirectories scans the root directory for audio files and adds them to the collection.
func (c *Collection) ScanDirectories() error {
	return c.ScanDirectoriesWithProcessor(c.ProcessAudioFile)
}

// ScanDirectoriesWithProcessor allows specifying a custom processor function for testing.
func (c *Collection) ScanDirectoriesWithProcessor(processor func(filePath string, logger *log.Logger) error) error {
	if !c.isOpen {
		return fmt.Errorf("collection is not open")
	}

	logger := log.New(io.Discard, "", 0)

	err := filepath.Walk(c.rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logger.Printf("Error accessing path %s: %v\n", path, err)
			return nil
		}
		if !info.IsDir() && isAudioFile(path) {
			err := processor(path, logger)
			if err != nil {
				logger.Printf("Error processing file %s: %v\n", path, err)
			}
		}
		return nil
	})

	return err
}

// Create inserts a new AudioInfo into the collection and returns the ID and error.
func (c *Collection) Create(info AudioInfo) (string, error) {
	if !c.isOpen {
		return "", fmt.Errorf("collection is not open")
	}

	info.ID = uuid.New().String()
	info.Created = time.Now()
	info.Updated = time.Now()
	extendedMetadataJSON, err := json.Marshal(info.ExtendedMetadata)
	if err != nil {
		return "", fmt.Errorf("error marshaling extended metadata: %v", err)
	}

	_, err = c.db.Exec(`
		INSERT INTO audio_files (
			id, title, artist, album, path, mime_type, genre, description,
			created, updated, publication_date, doi, extended_metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		info.ID, info.Title, info.Artist, info.Album, info.Path, info.MIME, info.Genre, info.Description,
		info.Created, info.Updated, info.PublicationDate, info.DOI, extendedMetadataJSON,
	)
	if err != nil {
		return "", fmt.Errorf("error inserting audio info: %v", err)
	}
	return info.ID, nil
}

// Read fetches an AudioInfo by ID and returns it with an error.
func (c *Collection) Read(id string) (AudioInfo, error) {
	if !c.isOpen {
		return AudioInfo{}, fmt.Errorf("collection is not open")
	}

	var info AudioInfo
	var extendedMetadataJSON []byte
	err := c.db.QueryRow(`
		SELECT
			id, title, artist, album, path, mime_type, genre, description,
			created, updated, publication_date, doi, extended_metadata
		FROM audio_files WHERE id = ?
	`, id).Scan(
		&info.ID, &info.Title, &info.Artist, &info.Album, &info.Path,
		&info.MIME, &info.Genre, &info.Description, &info.Created, &info.Updated,
		&info.PublicationDate, &info.DOI, &extendedMetadataJSON,
	)
	if err != nil {
		return info, fmt.Errorf("error fetching audio info: %v", err)
	}
	if err := json.Unmarshal(extendedMetadataJSON, &info.ExtendedMetadata); err != nil {
		return info, fmt.Errorf("error unmarshaling extended metadata: %v", err)
	}
	return info, nil
}

// Update updates an existing AudioInfo by ID and returns an error.
func (c *Collection) Update(id string, info AudioInfo) error {
	if !c.isOpen {
		return fmt.Errorf("collection is not open")
	}

	info.Updated = time.Now()
	extendedMetadataJSON, err := json.Marshal(info.ExtendedMetadata)
	if err != nil {
		return fmt.Errorf("error marshaling extended metadata: %v", err)
	}

	_, err = c.db.Exec(`
		UPDATE audio_files SET
			title = ?,
			artist = ?,
			album = ?,
			path = ?,
			mime_type = ?,
			genre = ?,
			description = ?,
			updated = ?,
			publication_date = ?,
			doi = ?,
			extended_metadata = ?
		WHERE id = ?
	`,
		info.Title, info.Artist, info.Album, info.Path, info.MIME, info.Genre, info.Description,
		info.Updated, info.PublicationDate, info.DOI, extendedMetadataJSON, id,
	)
	if err != nil {
		return fmt.Errorf("error updating audio info: %v", err)
	}
	return nil
}

// Delete removes an AudioInfo by ID and returns an error.
func (c *Collection) Delete(id string) error {
	if !c.isOpen {
		return fmt.Errorf("collection is not open")
	}

	_, err := c.db.Exec("DELETE FROM audio_files WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("error deleting audio info: %v", err)
	}
	return nil
}

// GetAlbums returns a list of unique albums in the collection.
func (c *Collection) GetAlbums() ([]string, error) {
	if !c.isOpen {
		return nil, fmt.Errorf("collection is not open")
	}

	rows, err := c.db.Query("SELECT DISTINCT album FROM audio_files WHERE album != '' ORDER BY album")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []string
	for rows.Next() {
		var album string
		if err := rows.Scan(&album); err != nil {
			return nil, err
		}
		albums = append(albums, album)
	}
	return albums, nil
}

// GetArtists returns a list of unique artists in the collection.
func (c *Collection) GetArtists() ([]string, error) {
	if !c.isOpen {
		return nil, fmt.Errorf("collection is not open")
	}

	rows, err := c.db.Query("SELECT DISTINCT artist FROM audio_files WHERE artist != '' ORDER BY artist")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artists []string
	for rows.Next() {
		var artist string
		if err := rows.Scan(&artist); err != nil {
			return nil, err
		}
		artists = append(artists, artist)
	}
	return artists, nil
}

// GetTitles returns a list of unique titles in the collection.
func (c *Collection) GetTitles() ([]string, error) {
	if !c.isOpen {
		return nil, fmt.Errorf("collection is not open")
	}

	rows, err := c.db.Query("SELECT DISTINCT title FROM audio_files WHERE title != '' ORDER BY title")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var titles []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, err
		}
		titles = append(titles, title)
	}
	return titles, nil
}

// SearchAudioFiles performs a full-text search on the audio_files table.
func (c *Collection) SearchAudioFiles(query string) ([]AudioInfo, error) {
	if !c.isOpen {
		return nil, fmt.Errorf("collection is not open")
	}

	q := fmt.Sprintf(
		"SELECT id, title, artist, album, path, mime_type, genre, description, created, updated, publication_date, doi, extended_metadata FROM audio_files WHERE title LIKE '%%%s%%' OR artist LIKE '%%%s%%' OR album LIKE '%%%s%%'",
		query, query, query,
	)
	rows, err := c.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []AudioInfo
	for rows.Next() {
		var info AudioInfo
		var extendedMetadataJSON []byte
		if err := rows.Scan(
			&info.ID, &info.Title, &info.Artist, &info.Album, &info.Path,
			&info.MIME, &info.Genre, &info.Description, &info.Created, &info.Updated,
			&info.PublicationDate, &info.DOI, &extendedMetadataJSON,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(extendedMetadataJSON, &info.ExtendedMetadata); err != nil {
			return nil, err
		}
		results = append(results, info)
	}
	return results, nil
}
