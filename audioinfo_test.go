package audioinfo

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
)

const (
	testDBName      = "test_audio.db"
	testRootDir     = "./testdata"
	testDescription = "Test Collection"
)

func setupTestDB(t *testing.T) *Collection {
	t.Helper()
	// Remove existing test database if it exists
	if _, err := os.Stat(testDBName); err == nil {
		if err := os.Remove(testDBName); err != nil {
			t.Fatalf("Failed to remove existing test database: %v", err)
		}
	}

	// Create test directory if it doesn't exist
	if err := os.MkdirAll(testRootDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Initialize a new collection
	collection, err := InitializeCollection(testDBName, testRootDir, testDescription)
	if err != nil {
		t.Fatalf("Failed to initialize collection: %v", err)
	}

	return collection
}

func teardownTestDB(t *testing.T, collection *Collection) {
	t.Helper()
	if err := collection.Close(); err != nil {
		t.Errorf("Failed to close collection: %v", err)
	}
	if err := os.Remove(testDBName); err != nil {
		t.Errorf("Failed to remove test database: %v", err)
	}
	if err := os.RemoveAll(testRootDir); err != nil {
		t.Errorf("Failed to remove test directory: %v", err)
	}
}

func TestInitializeCollection(t *testing.T) {
	collection := setupTestDB(t)
	defer teardownTestDB(t, collection)

	if collection.dbName != testDBName {
		t.Errorf("Expected dbName %q, got %q", testDBName, collection.dbName)
	}
	if collection.rootDir != testRootDir {
		t.Errorf("Expected rootDir %q, got %q", testRootDir, collection.rootDir)
	}
	if collection.description != testDescription {
		t.Errorf("Expected description %q, got %q", testDescription, collection.description)
	}
	if !collection.isOpen {
		t.Error("Expected collection to be open")
	}
}

func TestCreateAndRead(t *testing.T) {
	collection := setupTestDB(t)
	defer teardownTestDB(t, collection)

	// Test Create
	info := AudioInfo{
		Title:       "Test Song",
		Artist:      "Test Artist",
		Album:       "Test Album",
		Path:        filepath.Join(testRootDir, "test_song.mp3"),
		MIME:        "audio/mpeg",
		Genre:       "Test Genre",
		Description: "Test Description",
		DOI:         "10.1234/test",
		ExtendedMetadata: map[string]interface{}{
			"test_key": "test_value",
		},
	}

	id, err := collection.Create(info)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if id == "" {
		t.Error("Expected non-empty ID")
	}

	// Test Read
	readInfo, err := collection.Read(id)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if readInfo.Title != info.Title {
		t.Errorf("Expected title %q, got %q", info.Title, readInfo.Title)
	}
	if readInfo.Artist != info.Artist {
		t.Errorf("Expected artist %q, got %q", info.Artist, readInfo.Artist)
	}
	if readInfo.Album != info.Album {
		t.Errorf("Expected album %q, got %q", info.Album, readInfo.Album)
	}
	if readInfo.Path != info.Path {
		t.Errorf("Expected path %q, got %q", info.Path, readInfo.Path)
	}
	if readInfo.MIME != info.MIME {
		t.Errorf("Expected MIME %q, got %q", info.MIME, readInfo.MIME)
	}
	if readInfo.Genre != info.Genre {
		t.Errorf("Expected genre %q, got %q", info.Genre, readInfo.Genre)
	}
	if readInfo.Description != info.Description {
		t.Errorf("Expected description %q, got %q", info.Description, readInfo.Description)
	}
	if readInfo.DOI != info.DOI {
		t.Errorf("Expected DOI %q, got %q", info.DOI, readInfo.DOI)
	}
	if len(readInfo.ExtendedMetadata) != len(info.ExtendedMetadata) {
		t.Errorf("Expected extended metadata length %d, got %d", len(info.ExtendedMetadata), len(readInfo.ExtendedMetadata))
	}
}

func TestUpdate(t *testing.T) {
	collection := setupTestDB(t)
	defer teardownTestDB(t, collection)

	// Create a test entry
	info := AudioInfo{
		Title:       "Original Title",
		Artist:      "Original Artist",
		Album:       "Original Album",
		Path:        filepath.Join(testRootDir, "original.mp3"),
		MIME:        "audio/mpeg",
		Genre:       "Original Genre",
		Description: "Original Description",
	}
	id, err := collection.Create(info)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update the entry
	updatedInfo := info
	updatedInfo.Title = "Updated Title"
	updatedInfo.Artist = "Updated Artist"
	updatedInfo.Album = "Updated Album"
	updatedInfo.Description = "Updated Description"
	updatedInfo.ExtendedMetadata = map[string]interface{}{
		"updated_key": "updated_value",
	}

	err = collection.Update(id, updatedInfo)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify the update
	readInfo, err := collection.Read(id)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if readInfo.Title != updatedInfo.Title {
		t.Errorf("Expected updated title %q, got %q", updatedInfo.Title, readInfo.Title)
	}
	if readInfo.Artist != updatedInfo.Artist {
		t.Errorf("Expected updated artist %q, got %q", updatedInfo.Artist, readInfo.Artist)
	}
	if readInfo.Album != updatedInfo.Album {
		t.Errorf("Expected updated album %q, got %q", updatedInfo.Album, readInfo.Album)
	}
	if readInfo.Description != updatedInfo.Description {
		t.Errorf("Expected updated description %q, got %q", updatedInfo.Description, readInfo.Description)
	}
	if len(readInfo.ExtendedMetadata) != len(updatedInfo.ExtendedMetadata) {
		t.Errorf("Expected updated extended metadata length %d, got %d", len(updatedInfo.ExtendedMetadata), len(readInfo.ExtendedMetadata))
	}
}

func TestDelete(t *testing.T) {
	collection := setupTestDB(t)
	defer teardownTestDB(t, collection)

	// Create a test entry
	info := AudioInfo{
		Title:       "To Be Deleted",
		Artist:      "Delete Artist",
		Album:       "Delete Album",
		Path:        filepath.Join(testRootDir, "delete.mp3"),
		MIME:        "audio/mpeg",
		Genre:       "Delete Genre",
		Description: "Delete Description",
	}
	id, err := collection.Create(info)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Delete the entry
	err = collection.Delete(id)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deletion
	_, err = collection.Read(id)
	if err == nil {
		t.Error("Expected error when reading deleted entry, got nil")
	}
}

func TestGetAlbums(t *testing.T) {
	collection := setupTestDB(t)
	defer teardownTestDB(t, collection)

	// Create test entries with unique paths
	albums := []string{"Album 1", "Album 2", "Album 1"}
	for i, album := range albums {
		info := AudioInfo{
			Title:  album,
			Artist: album,
			Album:  album,
			Path:   filepath.Join(testRootDir, fmt.Sprintf("%s_%d.mp3", album, i)), // Unique path
			MIME:   "audio/mpeg",
		}
		_, err := collection.Create(info)
		if err != nil {
			t.Fatalf("Create failed for album %q: %v", album, err)
		}
	}

	// Get unique albums
	uniqueAlbums, err := collection.GetAlbums()
	if err != nil {
		t.Fatalf("GetAlbums failed: %v", err)
	}
	expectedAlbums := []string{"Album 1", "Album 2"}
	if len(uniqueAlbums) != len(expectedAlbums) {
		t.Fatalf("Expected %d albums, got %d", len(expectedAlbums), len(uniqueAlbums))
	}
	for _, album := range expectedAlbums {
		found := false
		for _, a := range uniqueAlbums {
			if a == album {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected album %q not found in results", album)
		}
	}
}

func TestGetArtists(t *testing.T) {
	collection := setupTestDB(t)
	defer teardownTestDB(t, collection)

	// Create test entries with unique paths
	artists := []string{"Artist 1", "Artist 2", "Artist 1"}
	for i, artist := range artists {
		info := AudioInfo{
			Title:  artist,
			Artist: artist,
			Album:  artist,
			Path:   filepath.Join(testRootDir, fmt.Sprintf("%s_%d.mp3", artist, i)), // Unique path
			MIME:   "audio/mpeg",
		}
		_, err := collection.Create(info)
		if err != nil {
			t.Fatalf("Create failed for artist %q: %v", artist, err)
		}
	}

	// Get unique artists
	uniqueArtists, err := collection.GetArtists()
	if err != nil {
		t.Fatalf("GetArtists failed: %v", err)
	}
	expectedArtists := []string{"Artist 1", "Artist 2"}
	if len(uniqueArtists) != len(expectedArtists) {
		t.Fatalf("Expected %d artists, got %d", len(expectedArtists), len(uniqueArtists))
	}
	for _, artist := range expectedArtists {
		found := false
		for _, a := range uniqueArtists {
			if a == artist {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected artist %q not found in results", artist)
		}
	}
}

func TestGetTitles(t *testing.T) {
	collection := setupTestDB(t)
	defer teardownTestDB(t, collection)

	// Create test entries with unique paths
	titles := []string{"Title 1", "Title 2", "Title 1"}
	for i, title := range titles {
		info := AudioInfo{
			Title:  title,
			Artist: title,
			Album:  title,
			Path:   filepath.Join(testRootDir, fmt.Sprintf("%s_%d.mp3", title, i)), // Unique path
			MIME:   "audio/mpeg",
		}
		_, err := collection.Create(info)
		if err != nil {
			t.Fatalf("Create failed for title %q: %v", title, err)
		}
	}

	// Get unique titles
	uniqueTitles, err := collection.GetTitles()
	if err != nil {
		t.Fatalf("GetTitles failed: %v", err)
	}
	expectedTitles := []string{"Title 1", "Title 2"}
	if len(uniqueTitles) != len(expectedTitles) {
		t.Fatalf("Expected %d titles, got %d", len(expectedTitles), len(uniqueTitles))
	}
	for _, title := range expectedTitles {
		found := false
		for _, t := range uniqueTitles {
			if t == title {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected title %q not found in results", title)
		}
	}
}

func TestSearchAudioFiles(t *testing.T) {
	collection := setupTestDB(t)
	defer teardownTestDB(t, collection)

	// Create test entries
	infos := []AudioInfo{
		{
			Title:  "Search Title 1",
			Artist: "Search Artist 1",
			Album:  "Search Album 1",
			Path:   filepath.Join(testRootDir, "search1.mp3"),
			MIME:   "audio/mpeg",
		},
		{
			Title:  "Search Title 2",
			Artist: "Search Artist 2",
			Album:  "Search Album 2",
			Path:   filepath.Join(testRootDir, "search2.mp3"),
			MIME:   "audio/mpeg",
		},
	}
	for _, info := range infos {
		_, err := collection.Create(info)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// Search for entries
	results, err := collection.SearchAudioFiles("Search Title")
	if err != nil {
		t.Fatalf("SearchAudioFiles failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	results, err = collection.SearchAudioFiles("Search Artist 1")
	if err != nil {
		t.Fatalf("SearchAudioFiles failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Title != "Search Title 1" {
		t.Errorf("Expected title %q, got %q", "Search Title 1", results[0].Title)
	}
}

func TestScanDirectories(t *testing.T) {
	collection := setupTestDB(t)
	defer teardownTestDB(t, collection)

	// Create a test MP3 file
	testFile := filepath.Join(testRootDir, "test.mp3")
	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	file.Close()

	// Define a mock processor function
	mockProcessor := func(filePath string, logger *log.Logger) error {
		if filePath == testFile {
			mime := getMIMEType(filePath)
			info := AudioInfo{
				Title:       "Test Title",
				Artist:      "Test Artist",
				Album:       "Test Album",
				Path:        filePath,
				MIME:        mime,
				Genre:       "Test Genre",
				Description: "",
				DOI:         "",
				ExtendedMetadata: map[string]interface{}{
					"year":         2023,
					"track_number": 1,
				},
			}
			_, err := collection.Create(info)
			if err != nil {
				logger.Printf("Error inserting %s into database: %v\n", filePath, err)
				return err
			}
		}
		return nil
	}

	// Scan directories with the mock processor
	err = collection.ScanDirectoriesWithProcessor(mockProcessor)
	if err != nil {
		t.Fatalf("ScanDirectoriesWithProcessor failed: %v", err)
	}

	// Verify the file was added
	infos, err := collection.SearchAudioFiles("Test Title")
	if err != nil {
		t.Fatalf("SearchAudioFiles failed: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(infos))
	}
	if infos[0].Path != testFile {
		t.Errorf("Expected path %q, got %q", testFile, infos[0].Path)
	}
}
