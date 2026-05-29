package audiobox

const (
	/** PropertyDOI is the propertyID for Digital Object Identifiers.
	 *
	 * Example:
	 *   id := Identifier{PropertyID: PropertyDOI, Value: "10.1234/example", URL: "https://doi.org/10.1234/example"}
	 */
	PropertyDOI = "doi"

	/** PropertyISRC is the propertyID for International Standard Recording Codes.
	 *
	 * Example:
	 *   id := Identifier{PropertyID: PropertyISRC, Value: "USRC17607839"}
	 */
	PropertyISRC = "isrc"

	/** PropertyISWC is the propertyID for International Standard Musical Work Codes.
	 *
	 * Example:
	 *   id := Identifier{PropertyID: PropertyISWC, Value: "T-034.524.680-1"}
	 */
	PropertyISWC = "iswc"

	/** PropertyISNI is the propertyID for International Standard Name Identifiers.
	 *
	 * Example:
	 *   id := Identifier{PropertyID: PropertyISNI, Value: "0000000121032683"}
	 */
	PropertyISNI = "isni"

	/** PropertyORCID is the propertyID for Open Researcher and Contributor IDs.
	 *
	 * Example:
	 *   id := Identifier{PropertyID: PropertyORCID, Value: "0000-0003-0900-6903", URL: "https://orcid.org/0000-0003-0900-6903"}
	 */
	PropertyORCID = "orcid"

	/** PropertyROR is the propertyID for Research Organization Registry identifiers.
	 *
	 * Example:
	 *   id := Identifier{PropertyID: PropertyROR, Value: "04aj4c181", URL: "https://ror.org/04aj4c181"}
	 */
	PropertyROR = "ror"

	/** PropertyMBID is the propertyID for MusicBrainz identifiers.
	 *
	 * Example:
	 *   id := Identifier{PropertyID: PropertyMBID, Value: "f27ec8db-af05-4f36-916e-3d57f91ecf7e"}
	 */
	PropertyMBID = "musicbrainz"

	/** PropertyVIAF is the propertyID for Virtual International Authority File identifiers.
	 *
	 * Example:
	 *   id := Identifier{PropertyID: PropertyVIAF, Value: "29639567", URL: "https://viaf.org/viaf/29639567"}
	 */
	PropertyVIAF = "viaf"

	/** PropertyARK is the propertyID for Archival Resource Keys.
	 *
	 * Example:
	 *   id := Identifier{PropertyID: PropertyARK, Value: "ark:/13030/tf5p30086k"}
	 */
	PropertyARK = "ark"

	/** PropertyHandle is the propertyID for Handle System persistent identifiers.
	 *
	 * Example:
	 *   id := Identifier{PropertyID: PropertyHandle, Value: "2027/uc1.b4118084"}
	 */
	PropertyHandle = "handle"
)

/** Identifier holds a single schema.org PropertyValue used as a persistent identifier.
 *
 * Parameters:
 *   PropertyID (string) — the identifier scheme; use the Property* constants
 *   Value      (string) — the identifier value
 *   URL        (string) — optional resolvable URL form of the identifier
 *   Name       (string) — optional human-readable label
 *
 * Example:
 *   id := audiobox.Identifier{
 *     PropertyID: audiobox.PropertyDOI,
 *     Value:      "10.1234/example",
 *     URL:        "https://doi.org/10.1234/example",
 *   }
 */
type Identifier struct {
	PropertyID string `json:"propertyID"       yaml:"propertyID"`
	Value      string `json:"value"            yaml:"value"`
	URL        string `json:"url,omitempty"    yaml:"url,omitempty"`
	Name       string `json:"name,omitempty"   yaml:"name,omitempty"`
}

/** Identifiers is an ordered list of Identifier values associated with a resource or agent.
 *
 * Example:
 *   ids := audiobox.Identifiers{
 *     {PropertyID: audiobox.PropertyDOI,  Value: "10.1234/ex"},
 *     {PropertyID: audiobox.PropertyISRC, Value: "USRC17607839"},
 *   }
 */
type Identifiers []Identifier

/** Album represents a distinct album release in the collection.
 * Two albums that share the same name but live in different directories
 * (e.g. a US release and a UK release) are treated as separate entries.
 *
 * Parameters:
 *   Name        (string) — the in_album metadata value stored on the tracks
 *   DisplayName (string) — the label shown in lists; includes a folder qualifier
 *                          when multiple releases share the same Name
 *   Dir         (string) — absolute path to the directory containing the album's tracks
 *
 * Example:
 *   alb := audiobox.Album{
 *     Name:        "801 Live",
 *     DisplayName: "801 Live [801-Live-UK]",
 *     Dir:         "/home/alice/Music/801/801-Live-UK",
 *   }
 */
type Album struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Dir         string `json:"dir"`
}

/** FolderEntry represents a directory that contains one or more audio files.
 * Path is relative to the collection's AudioDir. Name is the deslugified last
 * path component, suitable for display. TrackCount is the number of audio files
 * directly or indirectly inside the folder.
 *
 * Example:
 *   FolderEntry{
 *     Path:       "Jazz/Miles-Davis/Kind-Of-Blue",
 *     Name:       "Kind Of Blue",
 *     TrackCount: 9,
 *   }
 */
type FolderEntry struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	TrackCount int    `json:"trackCount"`
}

/** Agent represents a schema.org Person or Organization with optional persistent identifiers.
 *
 * Parameters:
 *   Type        (string)      — "Person" or "Organization"
 *   Name        (string)      — the agent's display name
 *   Identifiers (Identifiers) — list of identifiers (ISNI, ORCID, ROR, MusicBrainz, VIAF, etc.)
 *
 * Example:
 *   a := audiobox.Agent{
 *     Type: "Person",
 *     Name: "Johann Sebastian Bach",
 *     Identifiers: audiobox.Identifiers{
 *       {PropertyID: audiobox.PropertyVIAF, Value: "29639567", URL: "https://viaf.org/viaf/29639567"},
 *     },
 *   }
 */
type Agent struct {
	Type        string      `json:"type"                    yaml:"type"`
	Name        string      `json:"name"                    yaml:"name"`
	Identifiers Identifiers `json:"identifiers,omitempty"   yaml:"identifiers,omitempty"`
}

/** PlaylistInfo describes a saved playlist stored in the collection database.
 *
 * Parameters:
 *   ID         (string) — UUID v4 identifier
 *   Name       (string) — human-readable playlist name
 *   TrackCount (int)    — number of tracks in the playlist
 *   Created    (string) — ISO 8601 creation timestamp
 *
 * Example:
 *   pl := audiobox.PlaylistInfo{ID: "…", Name: "Evening Drive", TrackCount: 12}
 */
type PlaylistInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	TrackCount int    `json:"trackCount"`
	Created    string `json:"created"`
}
