package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/rsdoiel/audioinfo"
	"github.com/rsdoiel/termlib"
)

// OutputFormat represents the serialisation format for command output.
type OutputFormat string

const (
	FormatText OutputFormat = "text"
	FormatJSON OutputFormat = "json"
	FormatYAML OutputFormat = "yaml"
	FormatXML  OutputFormat = "xml"
)

// --------------------------------------------------------------------------
// Help text constants (Pandoc man-page style, formatted with FmtHelp)
// --------------------------------------------------------------------------

const helpGeneral = `
{app_name}(1) — audio collection metadata manager

SYNOPSIS

  {app_name} [OPTIONS] ACTION [COLLECTION.yaml] [PARAMETERS]

OPTIONS

  -h, -help
    display this help message

  -l, -license
    display license information

  -v, -version
    display version information

  -fmt FORMAT
    output format for list/search/show: text, json, yaml, xml  (default: text)

ACTIONS

  init [NAME [ROOTDIR [DESCRIPTION]]]
    Initialise a new collection.  Missing arguments are prompted interactively.

  scan COLLECTION.yaml
    Walk the collection's audioDir and ingest every audio file found.

  list COLLECTION.yaml [albums|artists|titles]
    List distinct albums, artists, or titles (default: albums).

  search COLLECTION.yaml QUERY
    Search records by title, album, or artist.

  show COLLECTION.yaml ID
    Display full metadata for the record with the given UUID.

  delete COLLECTION.yaml ID
    Remove the record with the given UUID from the collection.

  server COLLECTION.yaml
    Start a localhost web server for the collection.

  help [ACTION]
    Display detailed help for an action.

EXAMPLES

  {app_name} init mymusic ~/Music "My personal archive"
  {app_name} scan mymusic.yaml
  {app_name} list mymusic.yaml artists
  {app_name} search mymusic.yaml "Bach"
  {app_name} show mymusic.yaml 550e8400-e29b-41d4-a716-446655440000
  {app_name} delete mymusic.yaml 550e8400-e29b-41d4-a716-446655440000

SEE ALSO

  {app_name}(1) man page, https://github.com/rsdoiel/audioinfo
`

const helpInit = `
{app_name} init — initialise a new audio collection

SYNOPSIS

  {app_name} init [NAME [ROOTDIR [DESCRIPTION]]]

DESCRIPTION

  Creates NAME.yaml (the collection configuration) and NAME.db (the SQLite3
  metadata database) in the current working directory.

  ROOTDIR is the root of the audio file tree to scan.  It is stored as an
  absolute path in the YAML.

  If any arguments are omitted {app_name} prompts for the missing values
  interactively.

PARAMETERS

  NAME         short identifier for the collection (no spaces recommended)
  ROOTDIR      root directory of the audio file tree
  DESCRIPTION  human-readable description (quote if it contains spaces)

EXAMPLES

  # Supply all arguments at once
  {app_name} init mymusic ~/Music "My personal music archive"

  # Prompt for all values
  {app_name} init
`

const helpScan = `
{app_name} scan — scan audioDir for audio files

SYNOPSIS

  {app_name} scan COLLECTION.yaml

DESCRIPTION

  Walks the audioDir recorded in COLLECTION.yaml and ingests every recognised
  audio file (.mp3 .flac .ogg .m4a .wma .wav).  For each file {app_name}:

    1. Computes a SHA-256 checksum (fixity).
    2. Reads embedded ID3/Vorbis/MP4 tags.
    3. Inserts a new record, or updates the existing one if the file path
       is already in the database.

  Filesystem and database errors abort the scan.  Tag-decode errors are
  printed as warnings and the scan continues with minimal metadata.

EXAMPLES

  {app_name} scan mymusic.yaml
`

const helpList = `
{app_name} list — list distinct albums, artists, or titles

SYNOPSIS

  {app_name} list COLLECTION.yaml [albums|artists|titles]

DESCRIPTION

  Prints a sorted, deduplicated list of the chosen category.
  Defaults to albums when the category is omitted.

OPTIONS

  -fmt text|json|yaml|xml   output format (default: text)

EXAMPLES

  {app_name} list mymusic.yaml
  {app_name} list mymusic.yaml artists
  {app_name} -fmt json list mymusic.yaml titles
`

const helpSearch = `
{app_name} search — search the collection

SYNOPSIS

  {app_name} search COLLECTION.yaml QUERY

DESCRIPTION

  Performs a case-insensitive substring search across recording name,
  album, and artist fields.  Multiple words are treated as a single query
  string joined with spaces.

OPTIONS

  -fmt text|json|yaml|xml   output format (default: text)

EXAMPLES

  {app_name} search mymusic.yaml "Goldberg Variations"
  {app_name} -fmt json search mymusic.yaml Bach
`

const helpShow = `
{app_name} show — display full metadata for a record

SYNOPSIS

  {app_name} show COLLECTION.yaml ID

DESCRIPTION

  Fetches and prints all stored metadata for the record identified by UUID.

OPTIONS

  -fmt text|json|yaml|xml   output format (default: text)

EXAMPLES

  {app_name} show mymusic.yaml 550e8400-e29b-41d4-a716-446655440000
  {app_name} -fmt json show mymusic.yaml 550e8400-e29b-41d4-a716-446655440000
`

const helpDelete = `
{app_name} delete — remove a record from the collection

SYNOPSIS

  {app_name} delete COLLECTION.yaml ID

DESCRIPTION

  Permanently removes the record with the given UUID from the database.
  The audio file on disk is not affected.

EXAMPLES

  {app_name} delete mymusic.yaml 550e8400-e29b-41d4-a716-446655440000
`

const helpServer = `
{app_name} server — start a localhost web server for the collection

SYNOPSIS

  {app_name} server COLLECTION.yaml

DESCRIPTION

  Starts an HTTP server bound to 127.0.0.1 (default port 8010).

  Two types of content are served:

    /api/*   JSON endpoints for querying and managing the collection
    /*       Static files from the htdocs directory configured in the YAML

  The port, htdocs directory, and CORS policy are set in COLLECTION.yaml:

    port (int)           port to listen on (default: 8010)
    htdocs (string)      path to static web content directory
    corsOrigin (string)  Access-Control-Allow-Origin value
                         (default: "*"; set to "off" to disable)

  POST /api/scan starts an asynchronous re-scan of audioDir.
  Use GET /api/scan/status to poll for completion.
  Press Ctrl-C to stop the server.

ENDPOINTS

  GET  /api/list/albums     list distinct album names
  GET  /api/list/artists    list distinct artist names
  GET  /api/list/titles     list distinct recording titles
  GET  /api/search?q=QUERY  search the collection (regex and field-scope supported)
  GET  /api/show/{id}       full metadata for one record
  POST /api/scan            start async re-scan of audioDir
  GET  /api/scan/status     poll async scan progress
  GET  /api/audio/{id}      stream audio file (supports Range requests)
  GET  /api/help            API reference as Markdown

EXAMPLES

  {app_name} server mymusic.yaml
`

var helpTopics = map[string]string{
	"init":   helpInit,
	"scan":   helpScan,
	"list":   helpList,
	"search": helpSearch,
	"show":   helpShow,
	"delete": helpDelete,
	"server": helpServer,
}

// --------------------------------------------------------------------------
// main
// --------------------------------------------------------------------------

func main() {
	showHelp := flag.Bool("help", false, "display help")
	flag.BoolVar(showHelp, "h", false, "display help")
	showLicense := flag.Bool("license", false, "display license")
	flag.BoolVar(showLicense, "l", false, "display license")
	showVersion := flag.Bool("version", false, "display version")
	flag.BoolVar(showVersion, "v", false, "display version")
	fmtStr := flag.String("fmt", string(FormatText), "output format: text, json, yaml, xml")
	flag.Parse()

	appName := os.Args[0]

	if *showHelp {
		fmt.Println(audioinfo.FmtHelp(helpGeneral, appName, audioinfo.Version, audioinfo.ReleaseDate, audioinfo.ReleaseHash))
		os.Exit(0)
	}
	if *showLicense {
		fmt.Print(audioinfo.LicenseText)
		os.Exit(0)
	}
	if *showVersion {
		fmt.Println(audioinfo.FmtHelp("{app_name} {version} (released {release_date}, commit {release_hash})",
			appName, audioinfo.Version, audioinfo.ReleaseDate, audioinfo.ReleaseHash))
		os.Exit(0)
	}

	if flag.NArg() < 1 {
		fmt.Println(audioinfo.FmtHelp(helpGeneral, appName, audioinfo.Version, audioinfo.ReleaseDate, audioinfo.ReleaseHash))
		os.Exit(0)
	}

	format := parseFormat(*fmtStr)
	action := strings.ToLower(flag.Arg(0))
	args := flag.Args()[1:]

	switch action {
	case "init":
		handleInit(args)
	case "help":
		handleHelp(appName, args)
	case "scan", "list", "search", "show", "delete", "server", "player":
		if len(args) < 1 {
			fmt.Fprintf(os.Stderr, "error: %s requires a COLLECTION.yaml argument\n", action)
			os.Exit(1)
		}
		col, err := audioinfo.LoadCollection(args[0])
		if err != nil {
			log.Fatalf("error opening collection: %v", err)
		}
		defer col.Close()
		rest := args[1:]
		switch action {
		case "scan":
			handleScan(col)
		case "list":
			handleList(col, rest, format)
		case "search":
			handleSearch(col, rest, format)
		case "show":
			handleShow(col, rest, format)
		case "delete":
			handleDelete(col, rest)
		case "server":
			handleServer(col)
		case "player":
			handlePlayer(col)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown action %q\n", action)
		fmt.Println(audioinfo.FmtHelp(helpGeneral, appName, audioinfo.Version, audioinfo.ReleaseDate, audioinfo.ReleaseHash))
		os.Exit(1)
	}
}

// --------------------------------------------------------------------------
// Action handlers
// --------------------------------------------------------------------------

func handleHelp(appName string, args []string) {
	if len(args) == 0 {
		fmt.Println(audioinfo.FmtHelp(helpGeneral, appName, audioinfo.Version, audioinfo.ReleaseDate, audioinfo.ReleaseHash))
		return
	}
	topic := strings.ToLower(args[0])
	text, ok := helpTopics[topic]
	if !ok {
		fmt.Fprintf(os.Stderr, "no help available for %q\n", topic)
		os.Exit(1)
	}
	fmt.Println(audioinfo.FmtHelp(text, appName, audioinfo.Version, audioinfo.ReleaseDate, audioinfo.ReleaseHash))
}

func handleInit(args []string) {
	name, audioDir, description := "", "", ""
	if len(args) >= 1 {
		name = args[0]
	}
	if len(args) >= 2 {
		audioDir = args[1]
	}
	if len(args) >= 3 {
		description = strings.Join(args[2:], " ")
	}

	// Prompt for any missing values interactively.
	if name == "" || audioDir == "" {
		le := termlib.NewLineEditor(os.Stdin, os.Stdout)
		var err error
		if name == "" {
			name, err = le.Prompt("Collection name: ")
			if err != nil {
				log.Fatalf("init aborted: %v", err)
			}
			name = strings.TrimSpace(name)
		}
		if audioDir == "" {
			audioDir, err = le.Prompt("Audio directory: ")
			if err != nil {
				log.Fatalf("init aborted: %v", err)
			}
			audioDir = strings.TrimSpace(audioDir)
		}
		if description == "" {
			description, err = le.Prompt("Description (optional): ")
			if err != nil && err != termlib.ErrInterrupted {
				log.Fatalf("init aborted: %v", err)
			}
			description = strings.TrimSpace(description)
		}
	}

	if name == "" || audioDir == "" {
		fmt.Fprintln(os.Stderr, "error: name and root directory are required")
		os.Exit(1)
	}

	col, err := audioinfo.NewCollection(name, audioDir, description)
	if err != nil {
		log.Fatalf("error initialising collection: %v", err)
	}
	col.Close()

	cfg := col.Config()
	fmt.Printf("Initialised collection %q\n  config:   %s.yaml\n  database: %s\n  audioDir:  %s\n",
		cfg.Name, name, cfg.Database, cfg.AudioDir)
}

func handleScan(col *audioinfo.Collection) {
	fmt.Print("Scanning…\n")
	if err := col.ScanDirectories(); err != nil {
		log.Fatalf("scan failed: %v", err)
	}
	fmt.Print("Scan complete.\n")
}

func handleList(col *audioinfo.Collection, args []string, format OutputFormat) {
	category := "albums"
	if len(args) > 0 {
		category = strings.ToLower(args[0])
	}

	var items []string
	var err error
	switch category {
	case "albums":
		items, err = col.GetAlbums()
	case "artists":
		items, err = col.GetArtists()
	case "titles":
		items, err = col.GetTitles()
	default:
		fmt.Fprintf(os.Stderr, "unknown list category %q — use albums, artists, or titles\n", category)
		os.Exit(1)
	}
	if err != nil {
		log.Fatalf("error listing %s: %v", category, err)
	}

	label := strings.ToUpper(category[:1]) + category[1:]
	switch format {
	case FormatText:
		fmt.Printf("%s:\n", label)
		for _, item := range items {
			fmt.Println(" ", item)
		}
	case FormatJSON:
		data, _ := json.MarshalIndent(map[string]interface{}{category: items}, "", "  ")
		fmt.Println(string(data))
	case FormatYAML:
		data, _ := yaml.Marshal(map[string]interface{}{category: items})
		fmt.Print(string(data))
	case FormatXML:
		type List struct {
			XMLName xml.Name `xml:"list"`
			Items   []string `xml:"item"`
		}
		data, _ := xml.MarshalIndent(List{Items: items}, "", "  ")
		fmt.Println(xml.Header + string(data))
	}
}

func handleSearch(col *audioinfo.Collection, args []string, format OutputFormat) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "error: search requires a QUERY argument")
		os.Exit(1)
	}
	query := strings.Join(args, " ")
	results, err := col.SearchAudioFiles(query)
	if err != nil {
		log.Fatalf("search failed: %v", err)
	}
	if len(results) == 0 {
		fmt.Println("No results found.")
		return
	}
	printAudioInfoList(results, format)
}

func handleShow(col *audioinfo.Collection, args []string, format OutputFormat) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "error: show requires an ID argument")
		os.Exit(1)
	}
	info, err := col.Read(args[0])
	if err != nil {
		log.Fatalf("error reading record: %v", err)
	}
	printAudioInfoList([]audioinfo.AudioInfo{info}, format)
}

func handleServer(col *audioinfo.Collection) {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	if err := col.Serve(logger); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func handleDelete(col *audioinfo.Collection, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "error: delete requires an ID argument")
		os.Exit(1)
	}
	if err := col.Delete(args[0]); err != nil {
		log.Fatalf("error deleting record: %v", err)
	}
	fmt.Printf("Deleted %s\n", args[0])
}

func handlePlayer(col *audioinfo.Collection) {
	if err := audioinfo.RunPlayer(col); err != nil {
		log.Fatalf("player error: %v", err)
	}
}

// --------------------------------------------------------------------------
// Output helpers
// --------------------------------------------------------------------------

func parseFormat(s string) OutputFormat {
	switch strings.ToLower(s) {
	case "json":
		return FormatJSON
	case "yaml":
		return FormatYAML
	case "xml":
		return FormatXML
	default:
		return FormatText
	}
}

// xmlAudioInfo is a flat XML-serialisable projection of AudioInfo.
type xmlAudioInfo struct {
	XMLName        xml.Name `xml:"audioInfo"`
	ID             string   `xml:"id"`
	SchemaType     string   `xml:"schemaType"`
	Name           string   `xml:"name"`
	Description    string   `xml:"description"`
	ContentURL     string   `xml:"contentUrl"`
	EncodingFormat string   `xml:"encodingFormat"`
	Duration       string   `xml:"duration"`
	DatePublished  string   `xml:"datePublished"`
	InLanguage     string   `xml:"inLanguage"`
	Genre          string   `xml:"genre"`
	InAlbum        string   `xml:"inAlbum"`
	IsrcCode       string   `xml:"isrcCode"`
	RecordingOf    string   `xml:"recordingOf"`
	Checksum       string   `xml:"checksum"`
	Algorithm      string   `xml:"checksumAlgorithm"`
}

func toXMLInfo(a audioinfo.AudioInfo) xmlAudioInfo {
	return xmlAudioInfo{
		ID: a.ID, SchemaType: a.SchemaType, Name: a.Name,
		Description: a.Description, ContentURL: a.ContentURL,
		EncodingFormat: a.EncodingFormat, Duration: a.Duration,
		DatePublished: a.DatePublished, InLanguage: a.InLanguage,
		Genre: a.Genre, InAlbum: a.InAlbum, IsrcCode: a.IsrcCode,
		RecordingOf: a.RecordingOf, Checksum: a.Checksum,
		Algorithm: a.ChecksumAlgorithm,
	}
}

func printAudioInfoList(items []audioinfo.AudioInfo, format OutputFormat) {
	switch format {
	case FormatText:
		for _, a := range items {
			fmt.Printf("ID:             %s\n", a.ID)
			fmt.Printf("Name:           %s\n", a.Name)
			fmt.Printf("InAlbum:        %s\n", a.InAlbum)
			fmt.Printf("EncodingFormat: %s\n", a.EncodingFormat)
			fmt.Printf("ContentURL:     %s\n", a.ContentURL)
			fmt.Printf("Checksum:       %s (%s)\n", a.Checksum, a.ChecksumAlgorithm)
			if len(a.ByArtist) > 0 {
				names := make([]string, 0, len(a.ByArtist))
				for _, ag := range a.ByArtist {
					names = append(names, ag.Name)
				}
				fmt.Printf("ByArtist:       %s\n", strings.Join(names, "; "))
			}
			fmt.Println("---")
		}
	case FormatJSON:
		data, _ := json.MarshalIndent(items, "", "  ")
		fmt.Println(string(data))
	case FormatYAML:
		data, _ := yaml.Marshal(items)
		fmt.Print(string(data))
	case FormatXML:
		type Results struct {
			XMLName xml.Name       `xml:"results"`
			Count   int            `xml:"count,attr"`
			Items   []xmlAudioInfo `xml:"audioInfo"`
		}
		xmlItems := make([]xmlAudioInfo, len(items))
		for i, a := range items {
			xmlItems[i] = toXMLInfo(a)
		}
		data, _ := xml.MarshalIndent(Results{Count: len(items), Items: xmlItems}, "", "  ")
		fmt.Println(xml.Header + string(data))
	}
}
