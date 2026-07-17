package audiobox

import (
	"bytes"
	"encoding/binary"
	"log"
	"os"
	"path/filepath"
	"testing"
)

// makeChunk builds a single RIFF chunk: a 4-byte ID, a 4-byte little-endian
// size, the raw data, and a trailing zero pad byte when the data length is odd.
func makeChunk(id string, data []byte) []byte {
	var buf bytes.Buffer
	buf.WriteString(id)
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(data))); err != nil {
		panic(err)
	}
	buf.Write(data)
	if len(data)%2 == 1 {
		buf.WriteByte(0)
	}
	return buf.Bytes()
}

// buildWAV assembles a minimal RIFF/WAVE byte stream from an ordered list of
// already-built top-level chunks (see makeChunk).
func buildWAV(chunks ...[]byte) []byte {
	var body bytes.Buffer
	for _, c := range chunks {
		body.Write(c)
	}
	var buf bytes.Buffer
	buf.WriteString("RIFF")
	if err := binary.Write(&buf, binary.LittleEndian, uint32(4+body.Len())); err != nil {
		panic(err)
	}
	buf.WriteString("WAVE")
	buf.Write(body.Bytes())
	return buf.Bytes()
}

func TestParseWAVInfoTags_ExtractsInfoChunk(t *testing.T) {
	infoBody := append([]byte("INFO"),
		append(makeChunk("IART", []byte("Jake Shimabukuro")),
			append(makeChunk("INAM", []byte("Departure Suite")),
				append(makeChunk("IPRD", []byte("Travels")),
					append(makeChunk("IGNR", []byte("Ukulele")),
						makeChunk("ICRD", []byte("2017"))...)...)...)...)...)

	data := buildWAV(
		makeChunk("fmt ", make([]byte, 16)),
		makeChunk("LIST", infoBody),
		makeChunk("data", []byte{0, 0, 0, 0}),
	)

	tags, err := parseWAVInfoTags(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parseWAVInfoTags: %v", err)
	}
	if tags.Artist != "Jake Shimabukuro" {
		t.Errorf("Artist = %q, want %q", tags.Artist, "Jake Shimabukuro")
	}
	if tags.Title != "Departure Suite" {
		t.Errorf("Title = %q, want %q", tags.Title, "Departure Suite")
	}
	if tags.Album != "Travels" {
		t.Errorf("Album = %q, want %q", tags.Album, "Travels")
	}
	if tags.Genre != "Ukulele" {
		t.Errorf("Genre = %q, want %q", tags.Genre, "Ukulele")
	}
	if tags.Year != "2017" {
		t.Errorf("Year = %q, want %q", tags.Year, "2017")
	}
	if tags.IsZero() {
		t.Error("IsZero() = true, want false")
	}
}

func TestParseWAVInfoTags_NoInfoChunk(t *testing.T) {
	data := buildWAV(
		makeChunk("fmt ", make([]byte, 16)),
		makeChunk("data", []byte{1, 2, 3, 4}),
	)

	tags, err := parseWAVInfoTags(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parseWAVInfoTags: %v", err)
	}
	if !tags.IsZero() {
		t.Errorf("expected zero-value tags, got %+v", tags)
	}
}

func TestParseWAVInfoTags_NotRIFF(t *testing.T) {
	_, err := parseWAVInfoTags(bytes.NewReader([]byte("this is not a wav file at all")))
	if err == nil {
		t.Error("expected error for non-RIFF data, got nil")
	}
}

func TestParseWAVInfoTags_OddLengthValuePaddingDoesNotCorruptNextChunk(t *testing.T) {
	// "Bach" is 4 bytes (even); use an odd-length value to force a pad byte
	// between IART and the next sub-chunk, and make sure INAM still parses.
	infoBody := append([]byte("INFO"),
		append(makeChunk("IART", []byte("Bach7")), // 5 bytes — odd, needs pad
			makeChunk("INAM", []byte("Fugue"))...)...)

	data := buildWAV(
		makeChunk("LIST", infoBody),
	)

	tags, err := parseWAVInfoTags(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parseWAVInfoTags: %v", err)
	}
	if tags.Artist != "Bach7" {
		t.Errorf("Artist = %q, want %q", tags.Artist, "Bach7")
	}
	if tags.Title != "Fugue" {
		t.Errorf("Title = %q, want %q (padding after an odd-length value must be consumed)", tags.Title, "Fugue")
	}
}

func TestParseWAVInfoTags_TruncatedChunkDoesNotPanic(t *testing.T) {
	// A LIST chunk that claims a size larger than the bytes actually present.
	var buf bytes.Buffer
	buf.WriteString("LIST")
	if err := binary.Write(&buf, binary.LittleEndian, uint32(1_000_000)); err != nil {
		t.Fatal(err)
	}
	buf.WriteString("INFO")
	buf.Write(makeChunk("IART", []byte("Truncated")))

	data := buildWAV(buf.Bytes())

	// Must not panic and must not error — a truncated trailing chunk is
	// simply the end of whatever tags could be recovered.
	tags, err := parseWAVInfoTags(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parseWAVInfoTags: %v", err)
	}
	if tags.Artist != "Truncated" {
		t.Errorf("Artist = %q, want %q (whatever was readable before truncation)", tags.Artist, "Truncated")
	}
}

func TestParseWAVInfoTags_HugeDeclaredSubChunkSizeIsSkippedNotAllocated(t *testing.T) {
	// A sub-chunk inside INFO that claims an implausibly large size must be
	// skipped safely rather than triggering a multi-gigabyte allocation.
	var sub bytes.Buffer
	sub.WriteString("IART")
	if err := binary.Write(&sub, binary.LittleEndian, uint32(1<<31)); err != nil {
		t.Fatal(err)
	}
	// No actual data follows — parser must not try to read 2GB here.

	infoBody := append([]byte("INFO"), sub.Bytes()...)
	data := buildWAV(makeChunk("LIST", infoBody))

	tags, err := parseWAVInfoTags(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parseWAVInfoTags: %v", err)
	}
	if tags.Artist != "" {
		t.Errorf("Artist = %q, want empty (oversized sub-chunk must be skipped, not misparsed)", tags.Artist)
	}
}

// TestProcessAudioFileWAVRIFFInfoTags reproduces the "Jake Shimabukuro"
// artist-search bug: a .wav album stored one directory level deep
// (Albums/<Album>/track.wav, no separate Artist folder) previously fell back
// to artist "Unknown" because dhowden/tag can't read WAV files at all and the
// path alone can't recover an artist name. A real .wav file carrying RIFF
// LIST/INFO tags must now populate the artist (and album/title) correctly.
func TestProcessAudioFileWAVRIFFInfoTags(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	albumDir := filepath.Join(testMusicDir, "Travels")
	if err := os.MkdirAll(albumDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	defer os.RemoveAll(albumDir)

	infoBody := append([]byte("INFO"),
		append(makeChunk("IART", []byte("Jake Shimabukuro")),
			append(makeChunk("INAM", []byte("Departure Suite - part I")),
				makeChunk("IPRD", []byte("Travels"))...)...)...)
	wavBytes := buildWAV(
		makeChunk("fmt ", make([]byte, 16)),
		makeChunk("LIST", infoBody),
		makeChunk("data", []byte{0, 0, 0, 0}),
	)

	trackPath := filepath.Join(albumDir, "01-departure-suite.wav")
	if err := os.WriteFile(trackPath, wavBytes, 0644); err != nil {
		t.Fatalf("write wav: %v", err)
	}

	logger := log.New(os.Stderr, "test: ", 0)
	if err := col.ProcessAudioFile(trackPath, logger); err != nil {
		t.Fatalf("ProcessAudioFile: %v", err)
	}

	results, err := col.SearchAudioFiles(`artist:"Jake Shimabukuro"`)
	if err != nil {
		t.Fatalf("SearchAudioFiles: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf(`SearchAudioFiles(artist:"Jake Shimabukuro") got %d results, want 1`, len(results))
	}
	info := results[0]
	if info.Name != "Departure Suite - part I" {
		t.Errorf("Name = %q, want %q", info.Name, "Departure Suite - part I")
	}
	if info.InAlbum != "Travels" {
		t.Errorf("InAlbum = %q, want %q", info.InAlbum, "Travels")
	}
	for _, a := range info.ByArtist {
		if a.Name == "Unknown" {
			t.Error("ByArtist fell back to \"Unknown\" instead of using the WAV RIFF INFO artist tag")
		}
	}
}

// TestProcessAudioFileWAVWithoutTagsStillFallsBack is a regression guard: a
// .wav file with no RIFF INFO chunk at all must still fall back to
// path-derived metadata exactly as before, with no panic or error.
func TestProcessAudioFileWAVWithoutTagsStillFallsBack(t *testing.T) {
	col, cleanup := setupTestCollection(t)
	defer cleanup()

	albumDir := filepath.Join(testMusicDir, "Peace-Love-Ukulele")
	if err := os.MkdirAll(albumDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	defer os.RemoveAll(albumDir)

	wavBytes := buildWAV(
		makeChunk("fmt ", make([]byte, 16)),
		makeChunk("data", []byte{0, 0, 0, 0}),
	)
	trackPath := filepath.Join(albumDir, "01-track.wav")
	if err := os.WriteFile(trackPath, wavBytes, 0644); err != nil {
		t.Fatalf("write wav: %v", err)
	}

	logger := log.New(os.Stderr, "test: ", 0)
	if err := col.ProcessAudioFile(trackPath, logger); err != nil {
		t.Fatalf("ProcessAudioFile: %v", err)
	}

	results, err := col.SearchAudioFiles("Peace-Love-Ukulele")
	if err != nil {
		t.Fatalf("SearchAudioFiles: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("SearchAudioFiles got %d results, want 1", len(results))
	}
	if !hasArtistName(results[0].ByArtist) {
		t.Error("ByArtist should have a fallback name even without RIFF tags")
	}
}
