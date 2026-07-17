package audiobox

import (
	"path/filepath"
	"testing"
)

func TestQueryTracks_NoFiltersReturnsEverything(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	for i, name := range []string{"a", "b", "c"} {
		info := sampleInfo(name)
		info.ContentURL = filepath.Join(testMusicDir, "qt-nofilter", name+".mp3")
		_ = i
		if _, err := col.Create(info); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	results, err := col.QueryTracks(TrackQueryOptions{})
	if err != nil {
		t.Fatalf("QueryTracks: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
}

func TestQueryTracks_ArtistSubstringCaseInsensitive(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	shimabukuro := sampleInfo("shim")
	shimabukuro.ByArtist = []Agent{{Type: "Person", Name: "Jake Shimabukuro"}}
	shimabukuro.ContentURL = filepath.Join(testMusicDir, "qt-artist", "shim.mp3")
	if _, err := col.Create(shimabukuro); err != nil {
		t.Fatalf("Create: %v", err)
	}

	other := sampleInfo("other")
	other.ByArtist = []Agent{{Type: "Person", Name: "Glenn Gould"}}
	other.ContentURL = filepath.Join(testMusicDir, "qt-artist", "other.mp3")
	if _, err := col.Create(other); err != nil {
		t.Fatalf("Create: %v", err)
	}

	results, err := col.QueryTracks(TrackQueryOptions{Artist: "shimabukuro"})
	if err != nil {
		t.Fatalf("QueryTracks: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].ID != shimabukuro.ID && len(results[0].ByArtist) > 0 && results[0].ByArtist[0].Name != "Jake Shimabukuro" {
		t.Errorf("got wrong track: %+v", results[0])
	}

	// Case-insensitive, partial match.
	resultsUpper, err := col.QueryTracks(TrackQueryOptions{Artist: "SHIMA"})
	if err != nil {
		t.Fatalf("QueryTracks: %v", err)
	}
	if len(resultsUpper) != 1 {
		t.Fatalf("case-insensitive partial match: got %d results, want 1", len(resultsUpper))
	}
}

func TestQueryTracks_YearRangeInclusive(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	years := map[string]string{"y1965": "1965", "y1970": "1970", "y1975": "1975", "y1980": "1980"}
	for suffix, year := range years {
		info := sampleInfo(suffix)
		info.DatePublished = year
		info.ContentURL = filepath.Join(testMusicDir, "qt-year", suffix+".mp3")
		if _, err := col.Create(info); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}
	// A track with no date at all — must be excluded whenever a year filter is active.
	undated := sampleInfo("undated")
	undated.DatePublished = ""
	undated.ContentURL = filepath.Join(testMusicDir, "qt-year", "undated.mp3")
	if _, err := col.Create(undated); err != nil {
		t.Fatalf("Create: %v", err)
	}

	results, err := col.QueryTracks(TrackQueryOptions{YearFrom: 1965, YearTo: 1975})
	if err != nil {
		t.Fatalf("QueryTracks: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3 (1965, 1970, 1975 inclusive)", len(results))
	}
	for _, r := range results {
		if r.DatePublished == "1980" || r.DatePublished == "" {
			t.Errorf("unexpected track in range results: %+v", r)
		}
	}
}

func TestQueryTracks_NoYearFilterIncludesUndatedTracks(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	undated := sampleInfo("undated2")
	undated.DatePublished = ""
	undated.ContentURL = filepath.Join(testMusicDir, "qt-noyearfilter", "undated.mp3")
	if _, err := col.Create(undated); err != nil {
		t.Fatalf("Create: %v", err)
	}

	results, err := col.QueryTracks(TrackQueryOptions{})
	if err != nil {
		t.Fatalf("QueryTracks: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (undated track must not be dropped when no year filter is set)", len(results))
	}
}

func TestQueryTracks_ExcludeFolders(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	included := sampleInfo("incl")
	included.ContentURL = filepath.Join(testMusicDir, "Keep", "track.mp3")
	if _, err := col.Create(included); err != nil {
		t.Fatalf("Create: %v", err)
	}
	excluded := sampleInfo("excl")
	excluded.ContentURL = filepath.Join(testMusicDir, "HolidayMusic", "track.mp3")
	if _, err := col.Create(excluded); err != nil {
		t.Fatalf("Create: %v", err)
	}

	results, err := col.QueryTracks(TrackQueryOptions{
		ExcludeFolders: []string{filepath.Join(testMusicDir, "HolidayMusic")},
	})
	if err != nil {
		t.Fatalf("QueryTracks: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].InAlbum != included.InAlbum {
		t.Errorf("got %+v, want the Keep track", results[0])
	}
}

func TestQueryTracks_CombinedFilters(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	// Matches everything.
	match := sampleInfo("match")
	match.ByArtist = []Agent{{Type: "Person", Name: "The Eagles"}}
	match.DatePublished = "1972"
	match.ContentURL = filepath.Join(testMusicDir, "Keep2", "match.mp3")
	if _, err := col.Create(match); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Wrong artist.
	wrongArtist := sampleInfo("wrongartist")
	wrongArtist.ByArtist = []Agent{{Type: "Person", Name: "Someone Else"}}
	wrongArtist.DatePublished = "1972"
	wrongArtist.ContentURL = filepath.Join(testMusicDir, "Keep2", "wrongartist.mp3")
	if _, err := col.Create(wrongArtist); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Right artist, wrong year.
	wrongYear := sampleInfo("wrongyear")
	wrongYear.ByArtist = []Agent{{Type: "Person", Name: "The Eagles"}}
	wrongYear.DatePublished = "1990"
	wrongYear.ContentURL = filepath.Join(testMusicDir, "Keep2", "wrongyear.mp3")
	if _, err := col.Create(wrongYear); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Right artist, right year, excluded folder.
	excludedFolder := sampleInfo("excludedfolder")
	excludedFolder.ByArtist = []Agent{{Type: "Person", Name: "The Eagles"}}
	excludedFolder.DatePublished = "1972"
	excludedFolder.ContentURL = filepath.Join(testMusicDir, "Skip2", "excludedfolder.mp3")
	if _, err := col.Create(excludedFolder); err != nil {
		t.Fatalf("Create: %v", err)
	}

	results, err := col.QueryTracks(TrackQueryOptions{
		Artist:         "eagles",
		YearFrom:       1965,
		YearTo:         1975,
		ExcludeFolders: []string{filepath.Join(testMusicDir, "Skip2")},
	})
	if err != nil {
		t.Fatalf("QueryTracks: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1: %+v", len(results), results)
	}
	if results[0].InAlbum != match.InAlbum {
		t.Errorf("got %+v, want the match track", results[0])
	}
}
