package audiobox

import (
	"regexp"
	"sort"
	"strings"
)

// reLeadingArticle matches a leading "The", "A", or "An" that is a standalone
// word (followed by whitespace and at least one more character) — so "The
// Beatles" matches but "Theatre of Tragedy" and "Anaconda" do not.
var reLeadingArticle = regexp.MustCompile(`(?i)^(the|an|a)\s+(\S.*)$`)

/** librarianSortKey returns the key used to order name the way a library
 * catalogue would: a leading standalone "The", "A", or "An" is moved out of
 * the way so e.g. "The Dave Matthews Band" sorts under "D" rather than "T".
 * The name itself is never altered — this is a comparison key only.
 *
 * Parameters:
 *   name (string) — an album, artist, or title
 *
 * Returns:
 *   string — lowercased name with any leading standalone article removed
 *
 * Example:
 *   librarianSortKey("The Dave Matthews Band") // "dave matthews band"
 *   librarianSortKey("Anaconda")               // "anaconda" (not a standalone article)
 */
func librarianSortKey(name string) string {
	if m := reLeadingArticle.FindStringSubmatch(name); m != nil {
		return strings.ToLower(m[2])
	}
	return strings.ToLower(name)
}

// sortStringsLibrarian sorts items in place by librarianSortKey, breaking
// ties with the original string so the order stays deterministic.
func sortStringsLibrarian(items []string) {
	sort.SliceStable(items, func(i, j int) bool {
		ki, kj := librarianSortKey(items[i]), librarianSortKey(items[j])
		if ki != kj {
			return ki < kj
		}
		return items[i] < items[j]
	})
}
