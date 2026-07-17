package audiobox

import (
	"path/filepath"
	"testing"

	"github.com/rsdoiel/opml"
)

func TestExportPlaylistOPML(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	track1 := sampleInfo("opml-1")
	track1.Name = "Departure Suite"
	track1.InAlbum = "Travels"
	track1.ByArtist = []Agent{{Type: "Person", Name: "Jake Shimabukuro"}}
	track1.ContentURL = filepath.Join(testMusicDir, "Travels", "01-departure.wav")
	id1, err := col.Create(track1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	track2 := sampleInfo("opml-2")
	track2.Name = "Train Ride"
	track2.InAlbum = "Travels"
	track2.ByArtist = []Agent{{Type: "Person", Name: "Jake Shimabukuro"}}
	track2.ContentURL = filepath.Join(testMusicDir, "Travels", "02-train-ride.wav")
	id2, err := col.Create(track2)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	plID, err := col.SavePlaylist("Road Trip", []string{id1, id2})
	if err != nil {
		t.Fatalf("SavePlaylist: %v", err)
	}

	data, err := col.ExportPlaylistOPML(plID)
	if err != nil {
		t.Fatalf("ExportPlaylistOPML: %v", err)
	}

	doc, err := opml.Parse(data)
	if err != nil {
		t.Fatalf("opml.Parse of exported data: %v", err)
	}
	if doc.Head == nil || doc.Head.Title != "Road Trip" {
		t.Errorf("Head.Title = %+v, want %q", doc.Head, "Road Trip")
	}
	if doc.Body == nil || len(doc.Body.Outline) != 2 {
		t.Fatalf("got %d outlines, want 2", len(doc.Body.Outline))
	}

	o1 := doc.Body.Outline[0]
	if o1.Text != "Departure Suite" {
		t.Errorf("outline[0].Text = %q, want %q", o1.Text, "Departure Suite")
	}
	if o1.URL != filepath.ToSlash(track1.ContentURL) {
		t.Errorf("outline[0].URL = %q, want %q", o1.URL, filepath.ToSlash(track1.ContentURL))
	}
	if o1.Category != "Jake Shimabukuro" {
		t.Errorf("outline[0].Category = %q, want %q", o1.Category, "Jake Shimabukuro")
	}
	if o1.Description != "Travels" {
		t.Errorf("outline[0].Description = %q, want %q", o1.Description, "Travels")
	}
	if o1.Type != "audio" {
		t.Errorf("outline[0].Type = %q, want %q", o1.Type, "audio")
	}

	o2 := doc.Body.Outline[1]
	if o2.Text != "Train Ride" {
		t.Errorf("outline[1].Text = %q, want %q (playlist order must be preserved)", o2.Text, "Train Ride")
	}
}

func TestExportPlaylistOPML_NotFound(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	if _, err := col.ExportPlaylistOPML("00000000-0000-0000-0000-000000000000"); err == nil {
		t.Error("expected error for unknown playlist id, got nil")
	}
}

func TestImportPlaylistOPML_MatchesExistingTracksAndSkipsUnmatched(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	track1 := sampleInfo("imp-1")
	track1.ContentURL = filepath.Join(testMusicDir, "Travels", "01-departure.wav")
	id1, err := col.Create(track1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	track2 := sampleInfo("imp-2")
	track2.ContentURL = filepath.Join(testMusicDir, "Travels", "02-train-ride.wav")
	id2, err := col.Create(track2)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	doc := opml.New()
	doc.Head.Title = "My Mix"
	doc.Body.Outline = []*opml.Outline{
		{Text: "Departure", URL: filepath.ToSlash(track1.ContentURL)},
		{Text: "Gone Track", URL: "Travels/no-such-file.wav"}, // not in the collection
		{Text: "Train Ride", URL: filepath.ToSlash(track2.ContentURL)},
	}
	data, err := opml.Marshal(doc)
	if err != nil {
		t.Fatalf("opml.Marshal: %v", err)
	}

	result, err := col.ImportPlaylistOPML(data, "")
	if err != nil {
		t.Fatalf("ImportPlaylistOPML: %v", err)
	}
	if result.Imported != 2 {
		t.Errorf("Imported = %d, want 2", result.Imported)
	}
	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", result.Skipped)
	}
	if result.Name != "My Mix" {
		t.Errorf("Name = %q, want %q (falls back to OPML head title)", result.Name, "My Mix")
	}

	tracks, err := col.LoadPlaylist(result.ID)
	if err != nil {
		t.Fatalf("LoadPlaylist: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("got %d tracks, want 2", len(tracks))
	}
	if tracks[0].ID != id1 || tracks[1].ID != id2 {
		t.Errorf("track order/ids = [%s, %s], want [%s, %s]", tracks[0].ID, tracks[1].ID, id1, id2)
	}
}

func TestImportPlaylistOPML_ExplicitNameOverridesHeadTitle(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	doc := opml.New()
	doc.Head.Title = "Ignored Title"
	data, err := opml.Marshal(doc)
	if err != nil {
		t.Fatalf("opml.Marshal: %v", err)
	}

	result, err := col.ImportPlaylistOPML(data, "Custom Name")
	if err != nil {
		t.Fatalf("ImportPlaylistOPML: %v", err)
	}
	if result.Name != "Custom Name" {
		t.Errorf("Name = %q, want %q", result.Name, "Custom Name")
	}
}

func TestImportPlaylistOPML_FallsBackToDefaultNameWhenNoTitle(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	doc := opml.New()
	data, err := opml.Marshal(doc)
	if err != nil {
		t.Fatalf("opml.Marshal: %v", err)
	}

	result, err := col.ImportPlaylistOPML(data, "")
	if err != nil {
		t.Fatalf("ImportPlaylistOPML: %v", err)
	}
	if result.Name != "Imported Playlist" {
		t.Errorf("Name = %q, want %q", result.Name, "Imported Playlist")
	}
	if result.Imported != 0 || result.Skipped != 0 {
		t.Errorf("Imported/Skipped = %d/%d, want 0/0 for an empty playlist", result.Imported, result.Skipped)
	}
}

func TestImportPlaylistOPML_InvalidXMLReturnsError(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	if _, err := col.ImportPlaylistOPML([]byte("not xml at all"), "x"); err == nil {
		t.Error("expected error for invalid OPML, got nil")
	}
}

func TestPlaylistOPML_ExportImportRoundTrip(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	track1 := sampleInfo("rt-1")
	track1.ContentURL = filepath.Join(testMusicDir, "Travels", "01-departure.wav")
	id1, err := col.Create(track1)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	track2 := sampleInfo("rt-2")
	track2.ContentURL = filepath.Join(testMusicDir, "Travels", "02-train-ride.wav")
	id2, err := col.Create(track2)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	origID, err := col.SavePlaylist("Round Trip", []string{id1, id2})
	if err != nil {
		t.Fatalf("SavePlaylist: %v", err)
	}

	data, err := col.ExportPlaylistOPML(origID)
	if err != nil {
		t.Fatalf("ExportPlaylistOPML: %v", err)
	}

	result, err := col.ImportPlaylistOPML(data, "")
	if err != nil {
		t.Fatalf("ImportPlaylistOPML: %v", err)
	}
	if result.Name != "Round Trip" {
		t.Errorf("Name = %q, want %q", result.Name, "Round Trip")
	}
	if result.Imported != 2 || result.Skipped != 0 {
		t.Errorf("Imported/Skipped = %d/%d, want 2/0", result.Imported, result.Skipped)
	}

	orig, err := col.LoadPlaylist(origID)
	if err != nil {
		t.Fatalf("LoadPlaylist(orig): %v", err)
	}
	reimported, err := col.LoadPlaylist(result.ID)
	if err != nil {
		t.Fatalf("LoadPlaylist(reimported): %v", err)
	}
	if len(orig) != len(reimported) {
		t.Fatalf("track count mismatch: orig=%d reimported=%d", len(orig), len(reimported))
	}
	for i := range orig {
		if orig[i].ID != reimported[i].ID {
			t.Errorf("track[%d] ID mismatch: orig=%s reimported=%s", i, orig[i].ID, reimported[i].ID)
		}
	}
}
