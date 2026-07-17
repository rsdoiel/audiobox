package audiobox

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

// maxWAVInfoValueBytes bounds how much of a single RIFF INFO sub-chunk value
// (e.g. IART, INAM) is read into memory. Real metadata values are short
// strings; a chunk claiming more than this is either corrupt or hostile and
// is skipped rather than allocated.
const maxWAVInfoValueBytes = 4096

/** WAVInfoTags holds metadata recovered from a WAV file's RIFF "LIST"/"INFO"
 * chunk. dhowden/tag (the library used for ID3/MP4/OGG/FLAC tags elsewhere in
 * this package) has no WAV support at all, so this is used as a fallback
 * metadata source for .wav files before falling through to path/filename-derived
 * defaults.
 */
type WAVInfoTags struct {
	Title  string // INAM
	Album  string // IPRD
	Artist string // IART
	Genre  string // IGNR
	Year   string // ICRD
}

/** IsZero reports whether no RIFF INFO fields were recovered — either the
 * file had no LIST/INFO chunk, or none of its sub-chunks were recognised.
 *
 * Returns:
 *   bool — true when every field is empty
 *
 * Example:
 *   tags, _ := readWAVInfoTags(path)
 *   hasMetadata := !tags.IsZero()
 */
func (w WAVInfoTags) IsZero() bool {
	return w.Title == "" && w.Album == "" && w.Artist == "" && w.Genre == "" && w.Year == ""
}

/** readWAVInfoTags opens filePath and extracts its RIFF "LIST"/"INFO" chunk tags.
 *
 * Parameters:
 *   filePath (string) — path to a .wav file
 *
 * Returns:
 *   WAVInfoTags — recovered tags; zero value when the file has no INFO chunk
 *   error       — non-nil when the file can't be opened or isn't a RIFF/WAVE file
 *
 * Example:
 *   tags, err := readWAVInfoTags("track.wav")
 */
func readWAVInfoTags(filePath string) (WAVInfoTags, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return WAVInfoTags{}, err
	}
	defer f.Close()
	return parseWAVInfoTags(f)
}

/** parseWAVInfoTags reads a RIFF/WAVE stream and extracts IART/INAM/IPRD/IGNR/ICRD
 * values from its "LIST"/"INFO" chunk, if present. Chunks are walked
 * sequentially using their declared sizes; a chunk whose declared size runs
 * past the end of the stream simply ends the walk early (whatever tags were
 * already recovered are returned) rather than erroring, since a truncated or
 * slightly malformed trailing chunk is common in real-world files.
 *
 * Parameters:
 *   r (io.Reader) — a RIFF/WAVE byte stream
 *
 * Returns:
 *   WAVInfoTags — recovered tags; zero value when no INFO chunk is found
 *   error       — non-nil only when the stream isn't a RIFF/WAVE file at all
 *
 * Example:
 *   tags, err := parseWAVInfoTags(file)
 */
func parseWAVInfoTags(r io.Reader) (WAVInfoTags, error) {
	br := bufio.NewReader(r)

	var riffHeader [12]byte
	if _, err := io.ReadFull(br, riffHeader[:]); err != nil {
		return WAVInfoTags{}, fmt.Errorf("reading RIFF header: %w", err)
	}
	if string(riffHeader[0:4]) != "RIFF" || string(riffHeader[8:12]) != "WAVE" {
		return WAVInfoTags{}, fmt.Errorf("not a RIFF/WAVE file")
	}

	var tags WAVInfoTags
	for {
		id, size, ok := readChunkHeader(br)
		if !ok {
			break
		}
		body := io.LimitReader(br, int64(size))
		if id == "LIST" {
			var listType [4]byte
			if _, err := io.ReadFull(body, listType[:]); err == nil && string(listType[:]) == "INFO" {
				parseWAVInfoSubChunks(body, &tags)
			}
		}
		if _, err := io.Copy(io.Discard, body); err != nil {
			break
		}
		if size%2 == 1 {
			if _, err := br.Discard(1); err != nil {
				break
			}
		}
	}
	return tags, nil
}

// readChunkHeader reads one 8-byte RIFF chunk header (4-byte ID + 4-byte
// little-endian size). ok is false at EOF or on a short/truncated header.
func readChunkHeader(br *bufio.Reader) (id string, size uint32, ok bool) {
	var hdr [8]byte
	if _, err := io.ReadFull(br, hdr[:]); err != nil {
		return "", 0, false
	}
	return string(hdr[0:4]), binary.LittleEndian.Uint32(hdr[4:8]), true
}

// parseWAVInfoSubChunks reads the sub-chunks of a LIST/INFO chunk (each a
// 4-byte ID + 4-byte size + data, individually padded to an even length) and
// records the values of recognised IDs into tags.
func parseWAVInfoSubChunks(r io.Reader, tags *WAVInfoTags) {
	br := bufio.NewReader(r)
	for {
		id, size, ok := readChunkHeader(br)
		if !ok {
			return
		}
		if size > maxWAVInfoValueBytes {
			if _, err := io.CopyN(io.Discard, br, int64(size)); err != nil {
				return
			}
		} else {
			data := make([]byte, size)
			if _, err := io.ReadFull(br, data); err != nil {
				return
			}
			value := strings.TrimSpace(strings.TrimRight(string(data), "\x00"))
			switch id {
			case "IART":
				tags.Artist = value
			case "INAM":
				tags.Title = value
			case "IPRD":
				tags.Album = value
			case "IGNR":
				tags.Genre = value
			case "ICRD":
				tags.Year = value
			}
		}
		if size%2 == 1 {
			if _, err := br.Discard(1); err != nil {
				return
			}
		}
	}
}
