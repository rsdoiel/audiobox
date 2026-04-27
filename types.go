package audioinfo

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
 *   id := audioinfo.Identifier{
 *     PropertyID: audioinfo.PropertyDOI,
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
 *   ids := audioinfo.Identifiers{
 *     {PropertyID: audioinfo.PropertyDOI,  Value: "10.1234/ex"},
 *     {PropertyID: audioinfo.PropertyISRC, Value: "USRC17607839"},
 *   }
 */
type Identifiers []Identifier

/** Agent represents a schema.org Person or Organization with optional persistent identifiers.
 *
 * Parameters:
 *   Type        (string)      — "Person" or "Organization"
 *   Name        (string)      — the agent's display name
 *   Identifiers (Identifiers) — list of identifiers (ISNI, ORCID, ROR, MusicBrainz, VIAF, etc.)
 *
 * Example:
 *   a := audioinfo.Agent{
 *     Type: "Person",
 *     Name: "Johann Sebastian Bach",
 *     Identifiers: audioinfo.Identifiers{
 *       {PropertyID: audioinfo.PropertyVIAF, Value: "29639567", URL: "https://viaf.org/viaf/29639567"},
 *     },
 *   }
 */
type Agent struct {
	Type        string      `json:"type"                    yaml:"type"`
	Name        string      `json:"name"                    yaml:"name"`
	Identifiers Identifiers `json:"identifiers,omitempty"   yaml:"identifiers,omitempty"`
}
