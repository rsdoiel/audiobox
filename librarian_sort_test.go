package audiobox

import (
	"path/filepath"
	"testing"
)

func TestLibrarianSortKey(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"The Dave Matthews Band", "dave matthews band"},
		{"the dave matthews band", "dave matthews band"},
		{"A Perfect Circle", "perfect circle"},
		{"An Cafe", "cafe"},
		{"Eagles", "eagles"},
		{"Theatre of Tragedy", "theatre of tragedy"}, // "The" glued to more letters, not a standalone article
		{"Anaconda", "anaconda"},                     // starts with "An" but not a standalone article
		{"A-ha", "a-ha"},                              // "A" not followed by whitespace — not a standalone article
		{"The", "the"},                                // article with nothing after it — leave as-is
		{"A", "a"},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := librarianSortKey(tc.name)
			if got != tc.want {
				t.Errorf("librarianSortKey(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestGetAlbumEntriesLibrarianSort(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	for _, album := range []string{"The Dave Matthews Band", "Eagles", "Coldplay"} {
		info := sampleInfo(album)
		info.InAlbum = album
		info.ContentURL = filepath.Join(testMusicDir, album, "track.mp3")
		if _, err := col.Create(info); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	albums, err := col.GetAlbumEntries()
	if err != nil {
		t.Fatalf("GetAlbumEntries: %v", err)
	}
	var order []string
	for _, a := range albums {
		order = append(order, a.DisplayName)
	}
	want := []string{"Coldplay", "The Dave Matthews Band", "Eagles"}
	if len(order) != len(want) {
		t.Fatalf("got %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q (full order: %v)", i, order[i], want[i], order)
		}
	}
}

func TestGetArtistsLibrarianSort(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	for i, artist := range []string{"The Dave Matthews Band", "Eagles", "Coldplay"} {
		info := sampleInfo(artist)
		info.ByArtist = []Agent{{Type: "Person", Name: artist}}
		info.ContentURL = filepath.Join(testMusicDir, filepath.Base(artist)+string(rune('0'+i))+".mp3")
		if _, err := col.Create(info); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	artists, err := col.GetArtists()
	if err != nil {
		t.Fatalf("GetArtists: %v", err)
	}
	want := []string{"Coldplay", "The Dave Matthews Band", "Eagles"}
	if len(artists) != len(want) {
		t.Fatalf("got %v, want %v", artists, want)
	}
	for i := range want {
		if artists[i] != want[i] {
			t.Errorf("artists[%d] = %q, want %q (full order: %v)", i, artists[i], want[i], artists)
		}
	}
}

func TestGetTitlesLibrarianSort(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	for i, title := range []string{"The Dave Matthews Band Live", "Eagles Greatest Hits", "Coldplay Anthems"} {
		info := sampleInfo(title)
		info.Name = title
		info.ContentURL = filepath.Join(testMusicDir, filepath.Base(title)+string(rune('0'+i))+".mp3")
		if _, err := col.Create(info); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	titles, err := col.GetTitles()
	if err != nil {
		t.Fatalf("GetTitles: %v", err)
	}
	want := []string{"Coldplay Anthems", "The Dave Matthews Band Live", "Eagles Greatest Hits"}
	if len(titles) != len(want) {
		t.Fatalf("got %v, want %v", titles, want)
	}
	for i := range want {
		if titles[i] != want[i] {
			t.Errorf("titles[%d] = %q, want %q (full order: %v)", i, titles[i], want[i], titles)
		}
	}
}
