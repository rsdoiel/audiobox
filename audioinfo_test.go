package audioinfo

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testCollectionName = "test_collection"
	testRootDir        = "./testdata"
	testDescription    = "Test Collection"
)

// setupTestCollection creates a fresh collection in a temp directory and returns
// both the Collection and a cleanup function.
func setupTestCollection(t *testing.T) (*Collection, func()) {
	t.Helper()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "testdata"), 0755); err != nil {
		t.Fatalf("creating testdata: %v", err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to temp: %v", err)
	}

	col, err := NewCollection(testCollectionName, testRootDir, testDescription)
	if err != nil {
		os.Chdir(orig)
		t.Fatalf("NewCollection: %v", err)
	}

	cleanup := func() {
		col.Close()
		os.Chdir(orig)
	}
	return col, cleanup
}

func sampleInfo(suffix string) AudioInfo {
	return AudioInfo{
		SchemaType:     "MusicRecording",
		Name:           "Test Song " + suffix,
		InAlbum:        "Test Album " + suffix,
		Genre:          "Classical",
		Description:    "A test recording",
		ContentURL:     filepath.Join(testRootDir, "test_"+suffix+".mp3"),
		EncodingFormat: "audio/mpeg",
		ByArtist:       []Agent{{Type: "Person", Name: "Test Artist " + suffix}},
		Identifiers:    Identifiers{{PropertyID: PropertyDOI, Value: "10.1234/" + suffix}},
	}
}

func TestNewCollection(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	cfg := col.Config()
	if cfg.Name != testCollectionName {
		t.Errorf("cfg.Name = %q, want %q", cfg.Name, testCollectionName)
	}
	if cfg.Description != testDescription {
		t.Errorf("cfg.Description = %q, want %q", cfg.Description, testDescription)
	}
	if !col.isOpen {
		t.Error("collection should be open after NewCollection")
	}
}

func TestLoadCollection(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	col.Close()
	defer cleanup()

	col2, err := LoadCollection(testCollectionName + ".yaml")
	if err != nil {
		t.Fatalf("LoadCollection: %v", err)
	}
	defer col2.Close()

	if col2.Config().Name != testCollectionName {
		t.Errorf("cfg.Name = %q, want %q", col2.Config().Name, testCollectionName)
	}
}

func TestCreateAndRead(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	info := sampleInfo("a")
	id, err := col.Create(info)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == "" {
		t.Fatal("Create returned empty ID")
	}

	got, err := col.Read(id)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.Name != info.Name {
		t.Errorf("Name = %q, want %q", got.Name, info.Name)
	}
	if got.InAlbum != info.InAlbum {
		t.Errorf("InAlbum = %q, want %q", got.InAlbum, info.InAlbum)
	}
	if got.ContentURL != info.ContentURL {
		t.Errorf("ContentURL = %q, want %q", got.ContentURL, info.ContentURL)
	}
	if len(got.ByArtist) != 1 || got.ByArtist[0].Name != info.ByArtist[0].Name {
		t.Errorf("ByArtist = %+v, want %+v", got.ByArtist, info.ByArtist)
	}
	if len(got.Identifiers) != 1 || got.Identifiers[0].Value != info.Identifiers[0].Value {
		t.Errorf("Identifiers = %+v, want %+v", got.Identifiers, info.Identifiers)
	}
}

func TestUpdate(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	id, err := col.Create(sampleInfo("b"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, _ := col.Read(id)
	updated.Name = "Updated Title"
	updated.InAlbum = "Updated Album"
	updated.ByArtist = []Agent{{Type: "Organization", Name: "New Ensemble"}}

	if err := col.Update(id, updated); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := col.Read(id)
	if err != nil {
		t.Fatalf("Read after Update: %v", err)
	}
	if got.Name != "Updated Title" {
		t.Errorf("Name = %q, want %q", got.Name, "Updated Title")
	}
	if got.ByArtist[0].Type != "Organization" {
		t.Errorf("ByArtist[0].Type = %q, want Organization", got.ByArtist[0].Type)
	}
}

func TestDelete(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	id, err := col.Create(sampleInfo("c"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := col.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := col.Read(id); err == nil {
		t.Error("Read after Delete should return error")
	}
}

func TestGetAlbums(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	for i, album := range []string{"Album A", "Album B", "Album A"} {
		info := sampleInfo(fmt.Sprintf("alb%d", i))
		info.InAlbum = album
		info.ContentURL = filepath.Join(testRootDir, fmt.Sprintf("alb%d.mp3", i))
		if _, err := col.Create(info); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	albums, err := col.GetAlbums()
	if err != nil {
		t.Fatalf("GetAlbums: %v", err)
	}
	if len(albums) != 2 {
		t.Errorf("len(albums) = %d, want 2", len(albums))
	}
}

func TestGetArtists(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	for i, artist := range []string{"Artist X", "Artist Y", "Artist X"} {
		info := sampleInfo(fmt.Sprintf("art%d", i))
		info.ByArtist = []Agent{{Type: "Person", Name: artist}}
		info.ContentURL = filepath.Join(testRootDir, fmt.Sprintf("art%d.mp3", i))
		if _, err := col.Create(info); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	artists, err := col.GetArtists()
	if err != nil {
		t.Fatalf("GetArtists: %v", err)
	}
	if len(artists) != 2 {
		t.Errorf("len(artists) = %d, want 2", len(artists))
	}
}

func TestGetTitles(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	for i, title := range []string{"Title 1", "Title 2", "Title 1"} {
		info := sampleInfo(fmt.Sprintf("ttl%d", i))
		info.Name = title
		info.ContentURL = filepath.Join(testRootDir, fmt.Sprintf("ttl%d.mp3", i))
		if _, err := col.Create(info); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	titles, err := col.GetTitles()
	if err != nil {
		t.Fatalf("GetTitles: %v", err)
	}
	if len(titles) != 2 {
		t.Errorf("len(titles) = %d, want 2", len(titles))
	}
}

func TestSearchAudioFiles(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	infos := []AudioInfo{
		{SchemaType: "MusicRecording", Name: "Goldberg Variations", InAlbum: "Bach: Keyboard Works",
			Genre: "Baroque", ContentURL: testRootDir + "/goldberg.mp3", EncodingFormat: "audio/mpeg",
			ByArtist: []Agent{{Type: "Person", Name: "Glenn Gould"}}},
		{SchemaType: "MusicRecording", Name: "Well-Tempered Clavier", InAlbum: "Bach: Keyboard Works",
			Genre: "Baroque", ContentURL: testRootDir + "/wtc.mp3", EncodingFormat: "audio/mpeg",
			ByArtist: []Agent{{Type: "Person", Name: "Glenn Gould"}}},
		{SchemaType: "MusicRecording", Name: "Moonlight Sonata", InAlbum: "Beethoven: Piano Sonatas",
			Genre: "Classical", ContentURL: testRootDir + "/moonlight.mp3", EncodingFormat: "audio/mpeg",
			ByArtist: []Agent{{Type: "Person", Name: "Daniel Barenboim"}}},
		{SchemaType: "MusicRecording", Name: "Infinity", InAlbum: "Infinity Recordings Vol. 1",
			Genre: "Folk", ContentURL: testRootDir + "/infinity.mp3", EncodingFormat: "audio/mpeg",
			ByArtist: []Agent{{Type: "Organization", Name: "Folkways Ensemble"}}},
	}
	for _, info := range infos {
		if _, err := col.Create(info); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	cases := []struct {
		query string
		want  int
		label string
	}{
		{"Goldberg", 1, "title match"},
		{"Bach", 2, "album substring match"},
		{"Gould", 2, "artist name via json_each"},
		{"Barenboim", 1, "artist name single"},
		{"Baroque", 2, "genre match"},
		{"infinity", 1, "title+album case-insensitive"},
		{"Glenn Keyboard", 2, "multi-term: artist + album word"},
		{"Moonlight Beethoven", 1, "multi-term: title + album"},
		{"Bach Barenboim", 0, "multi-term: no record spans both"},
		{"", 0, "empty query"},
		{"' OR '1'='1", 0, "SQL injection attempt"},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			results, err := col.SearchAudioFiles(tc.query)
			if err != nil {
				t.Fatalf("SearchAudioFiles(%q): %v", tc.query, err)
			}
			if len(results) != tc.want {
				t.Errorf("got %d results, want %d", len(results), tc.want)
			}
		})
	}
}

func TestScanDirectoriesWithProcessor(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	testFile := filepath.Join(testRootDir, "scan_test.mp3")
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("creating test file: %v", err)
	}
	f.Close()

	logger := log.New(os.Stderr, "test: ", 0)
	mockProcessor := func(filePath string, l *log.Logger) error {
		_, err := col.Create(AudioInfo{
			SchemaType:     "MusicRecording",
			Name:           "Scanned Track",
			ContentURL:     filePath,
			EncodingFormat: getMIMEType(filePath),
		})
		return err
	}

	if err := col.ScanDirectoriesWithProcessor(mockProcessor, logger); err != nil {
		t.Fatalf("ScanDirectoriesWithProcessor: %v", err)
	}

	results, err := col.SearchAudioFiles("Scanned Track")
	if err != nil {
		t.Fatalf("SearchAudioFiles after scan: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("len(results) = %d, want 1", len(results))
	}
}

func TestToJSONLD(t *testing.T) {
	info := AudioInfo{
		ID:             "test-uuid",
		SchemaType:     "MusicRecording",
		Name:           "Goldberg Variations BWV 988",
		InAlbum:        "Bach: Keyboard Works",
		EncodingFormat: "audio/flac",
		ContentURL:     "/music/goldberg.flac",
		ByArtist:       []Agent{{Type: "Person", Name: "Glenn Gould", Identifiers: Identifiers{{PropertyID: PropertyMBID, Value: "some-mbid"}}}},
		Identifiers:    Identifiers{{PropertyID: PropertyISRC, Value: "USRC17607839"}},
	}

	data, err := info.ToJSONLD()
	if err != nil {
		t.Fatalf("ToJSONLD: %v", err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON-LD output: %v", err)
	}
	if doc["@context"] != "https://schema.org" {
		t.Errorf("@context = %v, want https://schema.org", doc["@context"])
	}
	if doc["@type"] != "MusicRecording" {
		t.Errorf("@type = %v, want MusicRecording", doc["@type"])
	}
	if doc["name"] != info.Name {
		t.Errorf("name = %v, want %q", doc["name"], info.Name)
	}

	// Verify byArtist uses @type
	byArtist, ok := doc["byArtist"].([]interface{})
	if !ok || len(byArtist) == 0 {
		t.Fatal("byArtist missing or empty in JSON-LD")
	}
	artistMap := byArtist[0].(map[string]interface{})
	if artistMap["@type"] != "Person" {
		t.Errorf("byArtist[0].@type = %v, want Person", artistMap["@type"])
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"propertyID"`) {
		t.Error("JSON-LD identifiers missing propertyID field")
	}
}

func TestComputeSHA256(t *testing.T) {
	f, err := os.CreateTemp("", "sha256test*.bin")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	f.WriteString("hello audioinfo")
	f.Close()
	defer os.Remove(f.Name())

	sum, err := computeSHA256(f.Name())
	if err != nil {
		t.Fatalf("computeSHA256: %v", err)
	}
	if len(sum) != 64 {
		t.Errorf("checksum length = %d, want 64 hex chars", len(sum))
	}

	// Same file → same checksum
	sum2, _ := computeSHA256(f.Name())
	if sum != sum2 {
		t.Error("computeSHA256 is not deterministic")
	}
}
