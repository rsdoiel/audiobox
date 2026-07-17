package audiobox

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
	testMusicDir        = "./testdata"
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

	col, err := NewCollection(testCollectionName, testMusicDir, testDescription)
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
		ContentURL:     filepath.Join(testMusicDir, "test_"+suffix+".mp3"),
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

	// Tracks in two different subdirectories → two album entries regardless of tag.
	for i, subdir := range []string{"album-a", "album-b"} {
		info := sampleInfo(fmt.Sprintf("alb%d", i))
		info.InAlbum = subdir // tags match dir names for simplicity
		info.ContentURL = filepath.Join(testMusicDir, subdir, "track.mp3")
		if _, err := col.Create(info); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	// Second track in album-a — still only one entry for that dir.
	extra := sampleInfo("alb2")
	extra.InAlbum = "album-a"
	extra.ContentURL = filepath.Join(testMusicDir, "album-a", "track2.mp3")
	if _, err := col.Create(extra); err != nil {
		t.Fatalf("Create extra: %v", err)
	}

	albums, err := col.GetAlbums()
	if err != nil {
		t.Fatalf("GetAlbums: %v", err)
	}
	if len(albums) != 2 {
		t.Errorf("len(albums) = %d, want 2", len(albums))
	}
}

func TestGetAlbumEntriesSameNameDifferentDirs(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	// Simulate two "801 Live" releases: different dirs, different (or same) in_album tags.
	// Directory names deslugify to distinct readable names.
	dirs := []string{"801-Live", "801-Live-(American-Release)"}
	for i, subdir := range dirs {
		info := sampleInfo(fmt.Sprintf("live%d", i))
		info.InAlbum = "801 Live" // both have same (incomplete) tag
		info.ContentURL = filepath.Join(testMusicDir, subdir, "track.mp3")
		if _, err := col.Create(info); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	// A third album in its own dir.
	solo := sampleInfo("third")
	solo.InAlbum = "Solo Album"
	solo.ContentURL = filepath.Join(testMusicDir, "Solo-Album", "track.mp3")
	if _, err := col.Create(solo); err != nil {
		t.Fatalf("Create solo: %v", err)
	}

	entries, err := col.GetAlbumEntries()
	if err != nil {
		t.Fatalf("GetAlbumEntries: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("len(entries) = %d, want 3 (2 '801 Live' releases + 1 unique)", len(entries))
	}

	// Verify deslugified names.
	names := map[string]bool{}
	for _, e := range entries {
		names[e.DisplayName] = true
	}
	for _, want := range []string{"801 Live", "801 Live (American Release)", "Solo Album"} {
		if !names[want] {
			t.Errorf("missing expected album %q in entries", want)
		}
	}
}

func TestGetTracksByAlbum(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	for i, subdir := range []string{"801-Live", "801-Live-(American-Release)"} {
		info := sampleInfo(fmt.Sprintf("live%d", i))
		info.InAlbum = "801 Live"
		info.ContentURL = filepath.Join(testMusicDir, subdir, "track.mp3")
		if _, err := col.Create(info); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	entries, err := col.GetAlbumEntries()
	if err != nil {
		t.Fatalf("GetAlbumEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 album entries, got %d", len(entries))
	}

	for _, entry := range entries {
		tracks, err := col.GetTracksByAlbum(entry)
		if err != nil {
			t.Fatalf("GetTracksByAlbum(%q): %v", entry.DisplayName, err)
		}
		if len(tracks) != 1 {
			t.Errorf("album %q: want 1 track, got %d", entry.DisplayName, len(tracks))
		}
		for _, tr := range tracks {
			if !strings.HasPrefix(tr.ContentURL, entry.Dir) {
				t.Errorf("track %q: ContentURL %q not under Dir %q", tr.Name, tr.ContentURL, entry.Dir)
			}
		}
	}
}

// TestGetTracksByAlbumDir reproduces the "Milton-plus-Esperanza" /
// "Travels with Jack" bugs: a directory-based lookup must return exactly the
// tracks under that directory, independent of whatever the in_album tag
// says (which may be missing, or may differ from the directory name), and
// must not bleed into a sibling directory whose name happens to share a
// prefix.
func TestGetTracksByAlbumDir(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	// "Travels" has no album tag at all (as real .wav files without ID3
	// tags would produce) — directory lookup must still find it by path.
	untagged := sampleInfo("travels-1")
	untagged.InAlbum = ""
	untagged.ContentURL = filepath.Join(testMusicDir, "Travels", "01-departure.wav")
	untaggedID, err := col.Create(untagged)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// "Travels with Jack" is a sibling directory whose name starts with the
	// same prefix as "Travels" — must never appear in a "Travels" lookup.
	sibling := sampleInfo("travels-with-jack-1")
	sibling.InAlbum = "Travels with Jack"
	sibling.ContentURL = filepath.Join(testMusicDir, "Travels with Jack", "01-departure.wav")
	if _, err := col.Create(sibling); err != nil {
		t.Fatalf("Create: %v", err)
	}

	dir := filepath.Join(testMusicDir, "Travels")
	tracks, err := col.GetTracksByAlbumDir(dir)
	if err != nil {
		t.Fatalf("GetTracksByAlbumDir(%q): %v", dir, err)
	}
	if len(tracks) != 1 {
		t.Fatalf("want 1 track under %q, got %d: %+v", dir, len(tracks), tracks)
	}
	if tracks[0].ID != untaggedID {
		t.Errorf("got track %+v, want the untagged Travels track", tracks[0])
	}
}

func TestGetArtists(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	for i, artist := range []string{"Artist X", "Artist Y", "Artist X"} {
		info := sampleInfo(fmt.Sprintf("art%d", i))
		info.ByArtist = []Agent{{Type: "Person", Name: artist}}
		info.ContentURL = filepath.Join(testMusicDir, fmt.Sprintf("art%d.mp3", i))
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
	if len(artists) == 2 && artists[0] > artists[1] {
		t.Errorf("GetArtists not sorted: %q > %q", artists[0], artists[1])
	}
}

func TestGetTitles(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	for i, title := range []string{"Title 1", "Title 2", "Title 1"} {
		info := sampleInfo(fmt.Sprintf("ttl%d", i))
		info.Name = title
		info.ContentURL = filepath.Join(testMusicDir, fmt.Sprintf("ttl%d.mp3", i))
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
			Genre: "Baroque", ContentURL: testMusicDir + "/goldberg.mp3", EncodingFormat: "audio/mpeg",
			ByArtist: []Agent{{Type: "Person", Name: "Glenn Gould"}}},
		{SchemaType: "MusicRecording", Name: "Well-Tempered Clavier", InAlbum: "Bach: Keyboard Works",
			Genre: "Baroque", ContentURL: testMusicDir + "/wtc.mp3", EncodingFormat: "audio/mpeg",
			ByArtist: []Agent{{Type: "Person", Name: "Glenn Gould"}}},
		{SchemaType: "MusicRecording", Name: "Moonlight Sonata", InAlbum: "Beethoven: Piano Sonatas",
			Genre: "Classical", ContentURL: testMusicDir + "/moonlight.mp3", EncodingFormat: "audio/mpeg",
			ByArtist: []Agent{{Type: "Person", Name: "Daniel Barenboim"}}},
		{SchemaType: "MusicRecording", Name: "Infinity", InAlbum: "Infinity Recordings Vol. 1",
			Genre: "Folk", ContentURL: testMusicDir + "/infinity.mp3", EncodingFormat: "audio/mpeg",
			ByArtist: []Agent{{Type: "Organization", Name: "Folkways Ensemble"}}},
		{SchemaType: "MusicRecording", Name: "Ambassel", InAlbum: "On A Day Like This",
			Genre: "World", ContentURL: testMusicDir + "/ambassel.mp3", EncodingFormat: "audio/mpeg",
			ByArtist: []Agent{{Type: "Person", Name: "Meklit"}}},
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
		{"Melkit", 1, "fuzzy: typo Melkit matches artist Meklit"},
		// Field-scoped plain terms
		{"artist:Gould", 2, "field scope: artist plain"},
		{"artist:Barenboim", 1, "field scope: artist single"},
		{"genre:Baroque", 2, "field scope: genre"},
		{"artist:Gould genre:Baroque", 2, "field scope: artist + genre AND"},
		{"title:Goldberg", 1, "field scope: title alias"},
		{"album:Bach", 2, "field scope: album alias"},
		// Regex queries
		{"/Goldberg.*/", 1, "unscoped regex: title prefix"},
		{"artist:/Gould.*/", 2, "field regex: artist"},
		{"album:/Bach.*/", 2, "field regex: album prefix"},
		{"/Goldberg/ genre:Baroque", 1, "mixed: unscoped regex + field plain"},
		{"/[Gg]ould/", 2, "regex: character class"},
		// Quoted-phrase browse queries (as generated by buildBrowseQuery in the TypeScript player).
		{`album:"Bach: Keyboard Works"`, 2, "quoted phrase: multi-word album"},
		{`artist:"Glenn Gould"`, 2, "quoted phrase: multi-word artist"},
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

	// Invalid regex must return an error, not empty results.
	t.Run("invalid regex returns error", func(t *testing.T) {
		_, err := col.SearchAudioFiles("/[unclosed/")
		if err == nil {
			t.Error("expected error for invalid regex, got nil")
		}
	})
}

// TestSearchAudioFilesNoFuzzyForFieldScopedQuery reproduces the "Travel" /
// "Travels with Jack" album-mixing bug: a field-scoped query with no exact
// FTS match must return zero results rather than silently falling back to
// Levenshtein fuzzy matching, which conflates similarly-spelled albums.
func TestSearchAudioFilesNoFuzzyForFieldScopedQuery(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	infos := []AudioInfo{
		{SchemaType: "MusicRecording", Name: "Departure Suite", InAlbum: "Travels with Jack",
			ContentURL: testMusicDir + "/travels-with-jack-01.mp3", EncodingFormat: "audio/mpeg",
			ByArtist: []Agent{{Type: "Organization", Name: "ZBS"}}},
		{SchemaType: "MusicRecording", Name: "143 (Kelly's Song)", InAlbum: "Peace Love Ukulele",
			ContentURL: testMusicDir + "/peace-love-ukulele-01.mp3", EncodingFormat: "audio/mpeg",
			ByArtist: []Agent{{Type: "Person", Name: "Jake Shimabukuro"}}},
	}
	for _, info := range infos {
		if _, err := col.Create(info); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	// No track is tagged in_album exactly "Travel" — FTS finds nothing, and
	// since the whole query is field-scoped, fuzzy fallback must not kick in
	// and match "Travels with Jack" (edit distance 1).
	results, err := col.SearchAudioFiles(`album:"Travel"`)
	if err != nil {
		t.Fatalf(`SearchAudioFiles(album:"Travel"): %v`, err)
	}
	if len(results) != 0 {
		t.Errorf(`album:"Travel" got %d results, want 0 (fuzzy fallback must not run for field-scoped queries); got %+v`, len(results), results)
	}

	// A free-text (unscoped) query for the same misspelling should still get
	// fuzzy tolerance, since the user isn't selecting a known catalog entry.
	fuzzy, err := col.SearchAudioFiles("Travel")
	if err != nil {
		t.Fatalf("SearchAudioFiles(Travel): %v", err)
	}
	if len(fuzzy) == 0 {
		t.Errorf("unscoped \"Travel\" got 0 results, want fuzzy match against \"Travels with Jack\"")
	}
}

func TestParseQuery(t *testing.T) {
	cases := []struct {
		input  string
		tokens []queryToken
	}{
		{"", nil},
		{"Bach", []queryToken{{pattern: "Bach"}}},
		{"/Bach.*/", []queryToken{{pattern: "Bach.*", isRegex: true}}},
		{"artist:Gould", []queryToken{{field: "artist_names", pattern: "Gould"}}},
		{"artist:/Gould.*/", []queryToken{{field: "artist_names", pattern: "Gould.*", isRegex: true}}},
		{"title:Goldberg", []queryToken{{field: "name", pattern: "Goldberg"}}},
		{"album:Bach", []queryToken{{field: "in_album", pattern: "Bach"}}},
		{"genre:Baroque", []queryToken{{field: "genre", pattern: "Baroque"}}},
		// Unknown field alias — treated as plain unscoped term.
		{"Bach:Keyboard", []queryToken{{pattern: "Bach:Keyboard"}}},
		// Slash at start but no closing slash — plain term.
		{"/unclosed", []queryToken{{pattern: "/unclosed"}}},
		// Multi-token
		{"artist:Gould genre:Baroque", []queryToken{
			{field: "artist_names", pattern: "Gould"},
			{field: "genre", pattern: "Baroque"},
		}},
		// Quoted phrases — multi-word values.
		{`album:"Bach: Keyboard Works"`, []queryToken{
			{field: "in_album", pattern: "Bach: Keyboard Works"},
		}},
		{`artist:"Glenn Gould"`, []queryToken{
			{field: "artist_names", pattern: "Glenn Gould"},
		}},
		{`"Goldberg Variations"`, []queryToken{
			{pattern: "Goldberg Variations"},
		}},
		// Unclosed quote — treated as plain term including the opening '"'.
		{`album:"unclosed`, []queryToken{
			{field: "in_album", pattern: `"unclosed`},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := parseQuery(tc.input)
			if len(got) != len(tc.tokens) {
				t.Fatalf("len=%d, want %d: %+v", len(got), len(tc.tokens), got)
			}
			for i, want := range tc.tokens {
				if got[i] != want {
					t.Errorf("token[%d] = %+v, want %+v", i, got[i], want)
				}
			}
		})
	}
}

func TestParseDiscTrackFromFilename(t *testing.T) {
	cases := []struct {
		stem      string
		wantDisc  int
		wantTrack int
	}{
		// (Disc N) TT - Title
		{"(Disc 2) 01 - Lagrima", 2, 1},
		{"(Disc 2) 12 - Third Uncle", 2, 12},
		// Disc-N-TT-Title (slugified)
		{"Disc-1-01-Fleuve_Saint-Laurent", 1, 1},
		{"Disc-2-06-The_Other_City", 2, 6},
		// NN-TT- Title (disc-track hyphenated, 2-digit disc)
		{"01-01- Future Legend (1999 Remastered Version)", 1, 1},
		{"01-12- Whew", 1, 12},
		// N-TT Title (single-digit disc, 2-digit track)
		{"1-01 Pharaoh's Dance", 1, 1},
		{"1-09 Africa", 1, 9},
		// TT - Title (track only, disc=1)
		{"01 - Lagrima", 1, 1},
		{"12 - Lagrima - Reprise", 1, 12},
		// TT Title (track only, no dash)
		{"01 Lagrima", 1, 1},
		{"10 Third Uncle", 1, 10},
		// 00 prefix → unlisted, no position
		{"00 Barbecutie", 0, 0},
		// No numeric prefix → no match
		{"Chapter-1-Dreams-of-Bali", 0, 0},
		{"COOK01083_01", 0, 0},
		{"esperanza spalding - Emily's D+Evolution - 01 Good Lava", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.stem, func(t *testing.T) {
			d, tr := parseDiscTrackFromFilename(tc.stem)
			if d != tc.wantDisc || tr != tc.wantTrack {
				t.Errorf("parseDiscTrackFromFilename(%q) = (%d,%d), want (%d,%d)",
					tc.stem, d, tr, tc.wantDisc, tc.wantTrack)
			}
		})
	}
}

func TestScanDirectoriesWithProcessor(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	testFile := filepath.Join(testMusicDir, "scan_test.mp3")
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
	f.WriteString("hello audiobox")
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

func TestTrackTitleFromStem(t *testing.T) {
	cases := []struct {
		stem string
		want string
	}{
		// Numeric prefix stripped, remainder deslugified
		{"01 Song Title", "Song Title"},
		{"01-Song-Title", "Song Title"},
		{"1-02 Song Title", "Song Title"},
		{"(Disc 2) 03 Third Track", "Third Track"},
		// No numeric prefix — full stem deslugified
		{"Track_1", "Track 1"},
		{"Song_Name", "Song Name"},
		{"My-Favorite-Song", "My Favorite Song"},
		// Numeric prefix with no remainder — full stem fallback
		{"01", "01"},
	}
	for _, tc := range cases {
		t.Run(tc.stem, func(t *testing.T) {
			got := trackTitleFromStem(tc.stem)
			if got != tc.want {
				t.Errorf("trackTitleFromStem(%q) = %q, want %q", tc.stem, got, tc.want)
			}
		})
	}
}

func TestAlbumFromPath(t *testing.T) {
	audioDir := "/music"
	cases := []struct {
		filePath string
		want     string
	}{
		// File directly in audioDir — no album
		{"/music/track.mp3", ""},
		// One level: album is the directory
		{"/music/Ulithian Songs/track.mp3", "Ulithian Songs"},
		// Generic leaf skipped — parent is the album
		{"/music/Ulithian Songs/Tracks/track.mp3", "Ulithian Songs"},
		// Artist/Album layout — album is innermost non-generic
		{"/music/Artist/Album/track.mp3", "Album"},
		{"/music/Artist/Album/Tracks/track.mp3", "Album"},
		// All-generic path — outermost used as fallback
		{"/music/CD1/Tracks/track.mp3", "CD1"},
		// Slugified names are deslugified
		{"/music/My-Artist/Best-Of/Tracks/track.mp3", "Best Of"},
	}
	for _, tc := range cases {
		t.Run(tc.filePath, func(t *testing.T) {
			got := albumFromPath(audioDir, tc.filePath)
			if got != tc.want {
				t.Errorf("albumFromPath(%q) = %q, want %q", tc.filePath, got, tc.want)
			}
		})
	}
}

func TestArtistFromPath(t *testing.T) {
	audioDir := "/music"
	cases := []struct {
		filePath string
		want     string
	}{
		// No artist inferable (0 or 1 directory level)
		{"/music/track.mp3", ""},
		{"/music/Album/track.mp3", ""},
		// Outermost != album → artist
		{"/music/Artist/Album/track.mp3", "Artist"},
		{"/music/Artist/Album/Tracks/track.mp3", "Artist"},
		// Outermost == album (e.g. "Ulithian Songs/Tracks/") → no artist
		{"/music/Ulithian Songs/Tracks/track.mp3", ""},
		// Generic root container(s) (e.g. "Music/Albums/<Album>/track") are not
		// an artist name — regression test: this used to wrongly return "Music".
		{"/music/Music/Albums/Travels/track.wav", ""},
		{"/music/Music/Albums/Peace-Love-Ukulele/track.wav", ""},
		{"/music/Audio/Album/track.mp3", ""},
		// A real artist folder nested under a generic root is still found.
		{"/music/Music/Jake Shimabukuro/Travels/track.wav", "Jake Shimabukuro"},
	}
	for _, tc := range cases {
		t.Run(tc.filePath, func(t *testing.T) {
			got := artistFromPath(audioDir, tc.filePath)
			if got != tc.want {
				t.Errorf("artistFromPath(%q) = %q, want %q", tc.filePath, got, tc.want)
			}
		})
	}
}

func TestProcessAudioFileFallbacks(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	// Create a directory structure mimicking "Ulithian Songs/Tracks/Track_1.mp3".
	trackDir := filepath.Join(testMusicDir, "Ulithian Songs", "Tracks")
	if err := os.MkdirAll(trackDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	trackPath := filepath.Join(trackDir, "Track_1.mp3")
	f, err := os.Create(trackPath)
	if err != nil {
		t.Fatalf("create track: %v", err)
	}
	f.Close()
	defer os.RemoveAll(filepath.Join(testMusicDir, "Ulithian Songs"))

	logger := log.New(os.Stderr, "test: ", 0)
	if err := col.ProcessAudioFile(trackPath, logger); err != nil {
		t.Fatalf("ProcessAudioFile: %v", err)
	}

	results, err := col.SearchAudioFiles("Ulithian")
	if err != nil {
		t.Fatalf("SearchAudioFiles: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	info := results[0]

	if info.Name == "" {
		t.Error("Name should not be empty after fallback")
	}
	if info.InAlbum != "Ulithian Songs" {
		t.Errorf("InAlbum = %q, want %q", info.InAlbum, "Ulithian Songs")
	}
	if !hasArtistName(info.ByArtist) {
		t.Error("ByArtist should have a name after fallback")
	}
}

func TestGetAlbumEntriesGenericSubdir(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	// Files in "Album/Tracks/" — album name should be "Album", not "Tracks".
	info := sampleInfo("gen")
	info.InAlbum = ""
	info.ContentURL = filepath.Join(testMusicDir, "Album", "Tracks", "track.mp3")
	if _, err := col.Create(info); err != nil {
		t.Fatalf("Create: %v", err)
	}

	entries, err := col.GetAlbumEntries()
	if err != nil {
		t.Fatalf("GetAlbumEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Name != "Album" {
		t.Errorf("Name = %q, want %q", entries[0].Name, "Album")
	}
	if entries[0].DisplayName != "Album" {
		t.Errorf("DisplayName = %q, want %q", entries[0].DisplayName, "Album")
	}
}
