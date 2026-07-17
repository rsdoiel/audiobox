package audiobox

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// reLeadingYear matches the first run of 4 digits in a string, used to
// recover a comparable year from DatePublished ("1972", "1972-05-01", etc).
var reLeadingYear = regexp.MustCompile(`\d{4}`)

/** TrackQueryOptions describes the filters accepted by QueryTracks. Every
 * field is optional; an unset field applies no filter.
 *
 * Parameters:
 *   Artist         (string)   — case-insensitive substring match against any
 *                                artist name on the track
 *   YearFrom       (int)      — inclusive lower bound on DatePublished's year; 0 = no bound
 *   YearTo         (int)      — inclusive upper bound on DatePublished's year; 0 = no bound
 *   ExcludeFolders ([]string) — directories to exclude, same semantics as
 *                                GetAlbumEntries/GetArtists/GetTitles
 *
 * Example:
 *   opts := TrackQueryOptions{Artist: "eagles", YearFrom: 1965, YearTo: 1975}
 */
type TrackQueryOptions struct {
	Artist         string
	YearFrom       int
	YearTo         int
	ExcludeFolders []string
}

/** QueryTracks returns every track matching the given criteria, used to
 * build a playlist from criteria (artist name, release year range) rather
 * than by manually browsing and queuing tracks one by one. Folder exclusion
 * here is a one-off filter for this query — distinct from, and independent
 * of, the persistent Folders-tab browse-exclude toggle used elsewhere.
 *
 * Parameters:
 *   opts (TrackQueryOptions) — filters to apply; a zero-value TrackQueryOptions matches everything
 *
 * Returns:
 *   []AudioInfo — matching tracks, sorted by date published, then artist, then album, then track order
 *   error       — non-nil on database failure
 *
 * Example:
 *   tracks, err := col.QueryTracks(TrackQueryOptions{YearFrom: 1965, YearTo: 1975})
 */
func (c *Collection) QueryTracks(opts TrackQueryOptions) ([]AudioInfo, error) {
	if !c.isOpen {
		return nil, fmt.Errorf("collection is not open")
	}
	excl, exclArgs := folderExclusionSQL(opts.ExcludeFolders)
	rows, err := c.db.Query(`
		SELECT id, schema_type, name, description, content_url, encoding_format,
		       duration, date_published, in_language, genre, identifiers, by_artist,
		       in_album, disc_number, track_number, isrc_code, recording_of, checksum, checksum_algorithm,
		       created, updated
		FROM audio_files
		WHERE content_url != '' AND content_url IS NOT NULL`+excl,
		exclArgs...)
	if err != nil {
		return nil, fmt.Errorf("querying tracks: %w", err)
	}
	defer rows.Close()

	wantYear := opts.YearFrom > 0 || opts.YearTo > 0
	artist := strings.ToLower(opts.Artist)

	var out []AudioInfo
	for rows.Next() {
		info, err := scanAudioInfo(rows)
		if err != nil {
			return nil, err
		}
		if wantYear {
			year, ok := parseLeadingYear(info.DatePublished)
			if !ok {
				continue
			}
			if opts.YearFrom > 0 && year < opts.YearFrom {
				continue
			}
			if opts.YearTo > 0 && year > opts.YearTo {
				continue
			}
		}
		if artist != "" && !artistNameContains(info.ByArtist, artist) {
			continue
		}
		out = append(out, info)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating tracks: %w", err)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DatePublished != out[j].DatePublished {
			return out[i].DatePublished < out[j].DatePublished
		}
		ai, aj := extractArtistNames(out[i].ByArtist), extractArtistNames(out[j].ByArtist)
		if ai != aj {
			return ai < aj
		}
		if out[i].InAlbum != out[j].InAlbum {
			return out[i].InAlbum < out[j].InAlbum
		}
		if out[i].DiscNumber != out[j].DiscNumber {
			return out[i].DiscNumber < out[j].DiscNumber
		}
		return out[i].TrackNumber < out[j].TrackNumber
	})
	if out == nil {
		out = []AudioInfo{}
	}
	return out, nil
}

// parseLeadingYear extracts the first 4-digit run from s as a year.
func parseLeadingYear(s string) (int, bool) {
	m := reLeadingYear.FindString(s)
	if m == "" {
		return 0, false
	}
	y, err := strconv.Atoi(m)
	if err != nil {
		return 0, false
	}
	return y, true
}

// artistNameContains reports whether any agent's name contains substr
// (already lowercased by the caller), case-insensitively.
func artistNameContains(agents []Agent, substrLower string) bool {
	for _, a := range agents {
		if strings.Contains(strings.ToLower(a.Name), substrLower) {
			return true
		}
	}
	return false
}
