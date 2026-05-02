package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/rsdoiel/audiobox"
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

  {app_name} [OPTIONS] [ACTION [PARAMETERS]]

  Running {app_name} with no arguments starts the web service and opens the
  collection in the default browser.

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

  (default)
    Start the web service and open the collection in the default browser.
    Equivalent to running "{app_name} server".

  init
    Initialise (or upgrade) the standard ~/Audio audiobox installation.

  scan
    Walk ~/Audio and ingest every audio file found.

  list [albums|artists|titles]
    List distinct albums, artists, or titles (default: albums).

  search QUERY
    Search records by title, album, or artist.

  show ID
    Display full metadata for the record with the given UUID.

  delete ID
    Remove the record with the given UUID from the collection.

  server
    Start a localhost web server and open the default browser.

  sweep
    Remove database records whose audio files are no longer present on disk.

  player
    Start the terminal (TUI) player.

  help [ACTION]
    Display detailed help for an action.

EXAMPLES

  {app_name}
  {app_name} init
  {app_name} scan
  {app_name} sweep
  {app_name} list artists
  {app_name} search "Bach"
  {app_name} show 550e8400-e29b-41d4-a716-446655440000
  {app_name} delete 550e8400-e29b-41d4-a716-446655440000
  {app_name} player

SEE ALSO

  {app_name}(1) man page, https://github.com/rsdoiel/audiobox
`

const helpInit = `
{app_name} init — initialise or upgrade the standard audiobox installation

SYNOPSIS

  {app_name} init

DESCRIPTION

  Creates and populates ~/Audio with the standard audiobox layout.
  Running init again is safe — it only creates what is missing.

  Files and directories created:

    ~/Audio/                 root audio directory
    ~/Audio/audio.yaml       collection configuration
    ~/Audio/audio.db         SQLite3 metadata database
    ~/Audio/Music/           sub-directory for music
    ~/Audio/Podcasts/        sub-directory for podcasts
    ~/Audio/Theater/         sub-directory for theatre recordings
    ~/Audio/Books/           sub-directory for audiobooks

  The description in audio.yaml is set to "Audio collections for USER"
  where USER is the current login name.

EXAMPLES

  {app_name} init
`

const helpScan = `
{app_name} scan — scan ~/Audio for audio files

SYNOPSIS

  {app_name} scan

DESCRIPTION

  Walks ~/Audio and ingests every recognised audio file
  (.mp3 .flac .ogg .m4a .wma .wav).  For each file {app_name}:

    1. Computes a SHA-256 checksum (fixity).
    2. Reads embedded ID3/Vorbis/MP4 tags.
    3. Inserts a new record, or updates the existing one if the file path
       is already in the database.

  Filesystem and database errors abort the scan.  Tag-decode errors are
  printed as warnings and the scan continues with minimal metadata.

EXAMPLES

  {app_name} scan
`

const helpList = `
{app_name} list — list distinct albums, artists, or titles

SYNOPSIS

  {app_name} list [albums|artists|titles]

DESCRIPTION

  Prints a sorted, deduplicated list of the chosen category.
  Defaults to albums when the category is omitted.

OPTIONS

  -fmt text|json|yaml|xml   output format (default: text)

EXAMPLES

  {app_name} list
  {app_name} list artists
  {app_name} -fmt json list titles
`

const helpSearch = `
{app_name} search — search the collection

SYNOPSIS

  {app_name} search QUERY

DESCRIPTION

  Performs a case-insensitive substring search across recording name,
  album, and artist fields.  Multiple words are treated as a single query
  string joined with spaces.

OPTIONS

  -fmt text|json|yaml|xml   output format (default: text)

EXAMPLES

  {app_name} search "Goldberg Variations"
  {app_name} -fmt json search Bach
`

const helpShow = `
{app_name} show — display full metadata for a record

SYNOPSIS

  {app_name} show ID

DESCRIPTION

  Fetches and prints all stored metadata for the record identified by UUID.

OPTIONS

  -fmt text|json|yaml|xml   output format (default: text)

EXAMPLES

  {app_name} show 550e8400-e29b-41d4-a716-446655440000
  {app_name} -fmt json show 550e8400-e29b-41d4-a716-446655440000
`

const helpDelete = `
{app_name} delete — remove a record from the collection

SYNOPSIS

  {app_name} delete ID

DESCRIPTION

  Permanently removes the record with the given UUID from the database.
  The audio file on disk is not affected.

EXAMPLES

  {app_name} delete 550e8400-e29b-41d4-a716-446655440000
`

const helpSweep = `
{app_name} sweep — remove stale database records

SYNOPSIS

  {app_name} sweep

DESCRIPTION

  Compares every content_url stored in the database against the filesystem.
  Any record whose file no longer exists inside ~/Audio is permanently removed
  from the database and the full-text search index.

  A summary line reports how many records were removed.

EXAMPLES

  {app_name} sweep
`

const helpServer = `
{app_name} server — start a localhost web server for the collection

SYNOPSIS

  {app_name} [server]

DESCRIPTION

  Starts an HTTP server bound to 127.0.0.1 (default port 8010) and opens
  the collection in the operating system's default web browser.

  "server" is the default action: running {app_name} with no arguments is
  equivalent to "{app_name} server".

  The port, htdocs directory, and CORS policy are set in ~/Audio/audio.yaml:

    port (int)           port to listen on (default: 8010)
    htdocs (string)      path to static web content directory
    corsOrigin (string)  Access-Control-Allow-Origin value
                         (default: "*"; set to "off" to disable)

  The web UI provides controls to scan, sweep, and shut down the server.
  To stop the server from the command line press Ctrl-C.

ENDPOINTS

  GET  /api/status          collection status (initialized, track_count)
  POST /api/init            initialise or upgrade the collection
  GET  /api/list/albums     list distinct album names
  GET  /api/list/artists    list distinct artist names
  GET  /api/list/titles     list distinct recording titles
  GET  /api/search?q=QUERY  search the collection
  GET  /api/show/{id}       full metadata for one record
  DELETE /api/show/{id}     remove a record from the collection
  POST /api/scan            start async re-scan of audioDir
  GET  /api/scan/status     poll async scan progress
  POST /api/sweep           start async sweep of stale records
  GET  /api/sweep/status    poll async sweep progress
  GET  /api/audio/{id}      stream audio file (supports Range requests)
  POST /api/shutdown        gracefully stop the server
  GET  /api/help            API reference as Markdown

EXAMPLES

  {app_name}
  {app_name} server
`

const helpPlayer = `
{app_name} player — start the terminal (TUI) player

SYNOPSIS

  {app_name} player

DESCRIPTION

  Opens a full-screen terminal user interface for browsing and playing the
  collection.  Requires a terminal that supports ANSI escape codes.

KEY BINDINGS

  Tab          switch panel focus (browse ↔ queue)
  ← →          cycle browse tabs (Albums / Artists / Titles)
  ↑ ↓          navigate list
  PgUp / PgDn  page through list
  Home / End   jump to first / last item
  Enter        play selected item / jump to queued track
  a            append selected item to the queue
  Space        play / pause
  n            next track
  p            previous track
  +  -         volume up / down
  /            open search input
  Esc          cancel search
  q  Ctrl-C    quit

EXAMPLES

  {app_name} player
`

var helpTopics = map[string]string{
	"init":   helpInit,
	"scan":   helpScan,
	"sweep":  helpSweep,
	"list":   helpList,
	"search": helpSearch,
	"show":   helpShow,
	"delete": helpDelete,
	"server": helpServer,
	"player": helpPlayer,
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
		fmt.Println(audiobox.FmtHelp(helpGeneral, appName, audiobox.Version, audiobox.ReleaseDate, audiobox.ReleaseHash))
		os.Exit(0)
	}
	if *showLicense {
		fmt.Print(audiobox.LicenseText)
		os.Exit(0)
	}
	if *showVersion {
		fmt.Println(audiobox.FmtHelp("{app_name} {version} (released {release_date}, commit {release_hash})",
			appName, audiobox.Version, audiobox.ReleaseDate, audiobox.ReleaseHash))
		os.Exit(0)
	}

	format := parseFormat(*fmtStr)

	// No-argument default: start the web server.
	if flag.NArg() < 1 {
		col, err := loadOrInit()
		if err != nil {
			log.Fatalf("error opening collection: %v", err)
		}
		defer col.Close()
		handleServer(col)
		return
	}

	action := strings.ToLower(flag.Arg(0))
	args := flag.Args()[1:]

	switch action {
	case "init":
		handleInit(args)
	case "help":
		handleHelp(appName, args)
	case "server":
		// server auto-inits on first run just like the no-arg default.
		col, err := loadOrInit()
		if err != nil {
			log.Fatalf("error opening collection: %v", err)
		}
		defer col.Close()
		handleServer(col)
	case "scan", "sweep", "list", "search", "show", "delete", "player":
		col, err := audiobox.LoadAudiobox()
		if err != nil {
			log.Fatalf("error opening collection: %v", err)
		}
		defer col.Close()
		switch action {
		case "scan":
			handleScan(col)
		case "sweep":
			handleSweep(col)
		case "list":
			handleList(col, args, format)
		case "search":
			handleSearch(col, args, format)
		case "show":
			handleShow(col, args, format)
		case "delete":
			handleDelete(col, args)
		case "player":
			handlePlayer(col)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown action %q\n", action)
		fmt.Println(audiobox.FmtHelp(helpGeneral, appName, audiobox.Version, audiobox.ReleaseDate, audiobox.ReleaseHash))
		os.Exit(1)
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// loadOrInit tries to load the standard ~/Audio collection. If the config
// file or directory is missing it runs InitAudiobox() to create it first,
// then returns the initialised collection. This allows `audiobox` and
// `audiobox server` to work on a fresh system without requiring a prior
// `audiobox init` step.
func loadOrInit() (*audiobox.Collection, error) {
	col, err := audiobox.LoadAudiobox()
	if err == nil {
		return col, nil
	}
	// Config or directory missing — auto-init and try again.
	col, err = audiobox.InitAudiobox()
	if err != nil {
		return nil, fmt.Errorf("auto-init failed: %w", err)
	}
	return col, nil
}

// --------------------------------------------------------------------------
// Action handlers
// --------------------------------------------------------------------------

func handleHelp(appName string, args []string) {
	if len(args) == 0 {
		fmt.Println(audiobox.FmtHelp(helpGeneral, appName, audiobox.Version, audiobox.ReleaseDate, audiobox.ReleaseHash))
		return
	}
	topic := strings.ToLower(args[0])
	text, ok := helpTopics[topic]
	if !ok {
		fmt.Fprintf(os.Stderr, "no help available for %q\n", topic)
		os.Exit(1)
	}
	fmt.Println(audiobox.FmtHelp(text, appName, audiobox.Version, audiobox.ReleaseDate, audiobox.ReleaseHash))
}

func handleInit(args []string) {
	col, err := audiobox.InitAudiobox()
	if err != nil {
		log.Fatalf("error initialising audiobox: %v", err)
	}
	defer col.Close()

	cfg := col.Config()
	fmt.Printf("Initialised audiobox\n  config:   %s\n  database: %s\n  audioDir: %s\n",
		col.ConfigPath(), cfg.Database, cfg.AudioDir)
}

func handleScan(col *audiobox.Collection) {
	fmt.Print("Scanning…\n")
	if err := col.ScanDirectories(); err != nil {
		log.Fatalf("scan failed: %v", err)
	}
	fmt.Print("Scan complete.\n")
}

func handleSweep(col *audiobox.Collection) {
	n, err := col.Sweep()
	if err != nil {
		log.Fatalf("sweep failed: %v", err)
	}
	fmt.Printf("Sweep complete: %d stale record(s) removed.\n", n)
}

func handleList(col *audiobox.Collection, args []string, format OutputFormat) {
	category := "albums"
	if len(args) > 0 {
		category = strings.ToLower(args[0])
	}

	label := strings.ToUpper(category[:1]) + category[1:]

	if category == "albums" {
		albums, err := col.GetAlbumEntries()
		if err != nil {
			log.Fatalf("error listing albums: %v", err)
		}
		switch format {
		case FormatText:
			fmt.Printf("%s:\n", label)
			for _, a := range albums {
				fmt.Println(" ", a.DisplayName)
			}
		case FormatJSON:
			data, _ := json.MarshalIndent(map[string]interface{}{"albums": albums}, "", "  ")
			fmt.Println(string(data))
		case FormatYAML:
			data, _ := yaml.Marshal(map[string]interface{}{"albums": albums})
			fmt.Print(string(data))
		case FormatXML:
			type xmlAlbum struct {
				XMLName     xml.Name `xml:"album"`
				Name        string   `xml:"name"`
				DisplayName string   `xml:"displayName"`
				Dir         string   `xml:"dir"`
			}
			type List struct {
				XMLName xml.Name   `xml:"list"`
				Items   []xmlAlbum `xml:"album"`
			}
			xmlItems := make([]xmlAlbum, len(albums))
			for i, a := range albums {
				xmlItems[i] = xmlAlbum{Name: a.Name, DisplayName: a.DisplayName, Dir: a.Dir}
			}
			data, _ := xml.MarshalIndent(List{Items: xmlItems}, "", "  ")
			fmt.Println(xml.Header + string(data))
		}
		return
	}

	var items []string
	var err error
	switch category {
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

func handleSearch(col *audiobox.Collection, args []string, format OutputFormat) {
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

func handleShow(col *audiobox.Collection, args []string, format OutputFormat) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "error: show requires an ID argument")
		os.Exit(1)
	}
	info, err := col.Read(args[0])
	if err != nil {
		log.Fatalf("error reading record: %v", err)
	}
	printAudioInfoList([]audiobox.AudioInfo{info}, format)
}

func handleServer(col *audiobox.Collection) {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	cfg := col.Config()
	port := cfg.Port
	if port == 0 {
		port = 8010
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	go func() {
		time.Sleep(500 * time.Millisecond)
		if err := openBrowser(url); err != nil {
			logger.Printf("could not open browser: %v", err)
		}
	}()
	if err := col.Serve(logger); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func handleDelete(col *audiobox.Collection, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "error: delete requires an ID argument")
		os.Exit(1)
	}
	if err := col.Delete(args[0]); err != nil {
		log.Fatalf("error deleting record: %v", err)
	}
	fmt.Printf("Deleted %s\n", args[0])
}

func handlePlayer(col *audiobox.Collection) {
	if err := audiobox.RunPlayer(col); err != nil {
		log.Fatalf("player error: %v", err)
	}
}

// openBrowser opens url in the operating system's default web browser.
// Errors are non-fatal — headless systems without a display are silently skipped.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default: // linux, raspberry pi os, bsd, etc.
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
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

func toXMLInfo(a audiobox.AudioInfo) xmlAudioInfo {
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

func printAudioInfoList(items []audiobox.AudioInfo, format OutputFormat) {
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
