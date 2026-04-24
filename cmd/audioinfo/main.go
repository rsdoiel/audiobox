package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/rsdoiel/audioinfo"
)

// OutputFormat represents the format for output
type OutputFormat string

const (
	FormatText OutputFormat = "text"
	FormatJSON OutputFormat = "json"
	FormatYAML OutputFormat = "yaml"
	FormatXML  OutputFormat = "xml"
)

func parseOutputFormat(formatStr string) OutputFormat {
	format := FormatText // default
	switch strings.ToLower(formatStr) {
	case "json":
		format = FormatJSON
	case "yaml":
		format = FormatYAML
	case "xml":
		format = FormatXML
	}
	return format
}

func main() {
	// Define the format flag
	formatStr := flag.String("fmt", string(FormatText), "Output format: text, json, yaml, or xml")
	flag.Parse()

	// Check for minimum arguments
	if flag.NArg() < 2 {
		printUsage()
		os.Exit(1)
	}

	action := strings.ToLower(flag.Arg(0))
	collectionName := flag.Arg(1)

	// Parse the format
	format := parseOutputFormat(*formatStr)

	// Initialize the collection
	collection, err := audioinfo.OpenCollection(collectionName)
	if err != nil {
		log.Fatalf("Error opening collection: %v", err)
	}
	defer collection.Close()

	// Handle different actions
	switch action {
	case "init":
		handleInit(collectionName)
	case "scan":
		handleScan(collection, format)
	case "list":
		handleList(collection, flag.Args()[2:], format)
	case "search":
		handleSearch(collection, flag.Args()[2:], format)
	case "update":
		handleUpdate(collection, flag.Args()[2:], format)
	case "delete":
		handleDelete(collection, flag.Args()[2:], format)
	case "show":
		handleShow(collection, flag.Args()[2:], format)
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: audioinfo [OPTIONS] ACTION COLLECTION [PARAMETERS]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -fmt FORMAT    Output format: text, json, yaml, or xml (default: text)")
	fmt.Println()
	fmt.Println("Actions:")
	fmt.Println("  init COLLECTION_NAME    Initialize a new audio collection")
	fmt.Println("  scan COLLECTION_NAME   Scan directories for audio files")
	fmt.Println("  list COLLECTION_NAME [albums|artists|titles]  List albums, artists, or titles")
	fmt.Println("  search COLLECTION_NAME QUERY  Search for audio files")
	fmt.Println("  show COLLECTION_NAME ID  Show detailed information about an entry")
	fmt.Println("  update COLLECTION_NAME ID    Update an audio file entry")
	fmt.Println("  delete COLLECTION_NAME ID    Delete an audio file entry")
}

func handleInit(collectionName string) {
	// Get the directory to scan from environment variable or use current directory
	audioDir := os.Getenv("AUDIO_DIRECTORY")
	if audioDir == "" {
		audioDir = "."
	}

	// Initialize a new collection
	description := "Audio Collection"
	_, err := audioinfo.InitializeCollection(collectionName, audioDir, description)
	if err != nil {
		log.Fatalf("Error initializing collection: %v", err)
	}

	fmt.Printf("Initialized new collection '%s' in directory '%s'\n", collectionName, audioDir)
}

func handleScan(collection *audioinfo.Collection, format OutputFormat) {
	fmt.Println("Scanning directories for audio files...")
	err := collection.ScanDirectories()
	if err != nil {
		log.Fatalf("Error scanning directories: %v", err)
	}

	switch format {
	case FormatText:
		fmt.Println("Scan completed successfully")
	case FormatJSON:
		fmt.Println(`{"status": "success", "message": "Scan completed successfully"}`)
	case FormatYAML:
		fmt.Println(`status: success
message: Scan completed successfully`)
	case FormatXML:
		fmt.Println(`<response><status>success</status><message>Scan completed successfully</message></response>`)
	}
}

func handleList(collection *audioinfo.Collection, args []string, format OutputFormat) {
	var items []string
	var err error

	listType := "albums"
	if len(args) > 0 {
		listType = strings.ToLower(args[0])
	}

	switch listType {
	case "albums":
		items, err = collection.GetAlbums()
	case "artists":
		items, err = collection.GetArtists()
	case "titles":
		items, err = collection.GetTitles()
	default:
		fmt.Println("Invalid list type. Use 'albums', 'artists', or 'titles'")
		return
	}

	if err != nil {
		log.Fatalf("Error getting %s: %v", listType, err)
	}

	switch format {
	case FormatText:
		fmt.Printf("%s:\n", strings.Title(listType))
		for _, item := range items {
			fmt.Println("-", item)
		}
	case FormatJSON:
		data := map[string]interface{}{
			listType: items,
		}
		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			log.Fatalf("Error marshaling to JSON: %v", err)
		}
		fmt.Println(string(jsonData))
	case FormatYAML:
		data := map[string]interface{}{
			listType: items,
		}
		yamlData, err := yaml.Marshal(data)
		if err != nil {
			log.Fatalf("Error marshaling to YAML: %v", err)
		}
		fmt.Println(string(yamlData))
	case FormatXML:
		type ItemList struct {
			XMLName xml.Name `xml:"items"`
			Items   []string `xml:"item"`
		}
		data := ItemList{
			XMLName: xml.Name{Space: "", Local: strings.Title(listType)},
			Items:   items,
		}
		xmlData, err := xml.MarshalIndent(data, "", "  ")
		if err != nil {
			log.Fatalf("Error marshaling to XML: %v", err)
		}
		fmt.Println(xml.Header + string(xmlData))
	}
}

func handleSearch(collection *audioinfo.Collection, args []string, format OutputFormat) {
	if len(args) < 1 {
		fmt.Println("Please provide a search query")
		return
	}

	query := strings.Join(args, " ")
	results, err := collection.SearchAudioFiles(query)
	if err != nil {
		log.Fatalf("Error searching: %v", err)
	}

	if len(results) == 0 {
		fmt.Println("No results found")
		return
	}

	switch format {
	case FormatText:
		fmt.Printf("Found %d results:\n", len(results))
		for _, result := range results {
			fmt.Printf("ID: %s\n", result.ID)
			fmt.Printf("Title: %s\n", result.Title)
			fmt.Printf("Artist: %s\n", result.Artist)
			fmt.Printf("Album: %s\n", result.Album)
			fmt.Printf("Path: %s\n", result.Path)
			fmt.Println("---")
		}
	case FormatJSON:
		jsonData, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			log.Fatalf("Error marshaling to JSON: %v", err)
		}
		fmt.Println(string(jsonData))
	case FormatYAML:
		yamlData, err := yaml.Marshal(results)
		if err != nil {
			log.Fatalf("Error marshaling to YAML: %v", err)
		}
		fmt.Println(string(yamlData))
	case FormatXML:
		type AudioInfoXML struct {
			XMLName         xml.Name `xml:"audioInfo"`
			ID              string   `xml:"id"`
			Title           string   `xml:"title"`
			Artist          string   `xml:"artist"`
			Album           string   `xml:"album"`
			Path            string   `xml:"path"`
			MIME            string   `xml:"mime"`
			Genre           string   `xml:"genre"`
			Description     string   `xml:"description"`
			PublicationDate string   `xml:"publicationDate"`
			DOI             string   `xml:"doi"`
		}

		type ResultsXML struct {
			XMLName xml.Name       `xml:"results"`
			Count   int            `xml:"count,attr"`
			Items   []AudioInfoXML `xml:"audioInfo"`
		}

		xmlItems := make([]AudioInfoXML, len(results))
		for i, result := range results {
			xmlItems[i] = AudioInfoXML{
				ID:              result.ID,
				Title:           result.Title,
				Artist:          result.Artist,
				Album:           result.Album,
				Path:            result.Path,
				MIME:            result.MIME,
				Genre:           result.Genre,
				Description:     result.Description,
				PublicationDate: result.PublicationDate,
				DOI:             result.DOI,
			}
		}

		data := ResultsXML{
			Count:   len(results),
			Items:   xmlItems,
		}
		xmlData, err := xml.MarshalIndent(data, "", "  ")
		if err != nil {
			log.Fatalf("Error marshaling to XML: %v", err)
		}
		fmt.Println(xml.Header + string(xmlData))
	}
}

func handleShow(collection *audioinfo.Collection, args []string, format OutputFormat) {
	if len(args) < 1 {
		fmt.Println("Please provide an ID to show")
		return
	}

	id := args[0]
	info, err := collection.Read(id)
	if err != nil {
		log.Fatalf("Error reading entry: %v", err)
	}

	switch format {
	case FormatText:
		fmt.Printf("ID: %s\n", info.ID)
		fmt.Printf("Title: %s\n", info.Title)
		fmt.Printf("Artist: %s\n", info.Artist)
		fmt.Printf("Album: %s\n", info.Album)
		fmt.Printf("Path: %s\n", info.Path)
		fmt.Printf("MIME: %s\n", info.MIME)
		fmt.Printf("Genre: %s\n", info.Genre)
		fmt.Printf("Description: %s\n", info.Description)
		fmt.Printf("Created: %v\n", info.Created)
		fmt.Printf("Updated: %v\n", info.Updated)
		fmt.Printf("Publication Date: %s\n", info.PublicationDate)
		fmt.Printf("DOI: %s\n", info.DOI)
		fmt.Printf("Extended Metadata: %v\n", info.ExtendedMetadata)
	case FormatJSON:
		jsonData, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			log.Fatalf("Error marshaling to JSON: %v", err)
		}
		fmt.Println(string(jsonData))
	case FormatYAML:
		yamlData, err := yaml.Marshal(info)
		if err != nil {
			log.Fatalf("Error marshaling to YAML: %v", err)
		}
		fmt.Println(string(yamlData))
	case FormatXML:
		type AudioInfoXML struct {
			XMLName         xml.Name              `xml:"audioInfo"`
			ID              string               `xml:"id"`
			Title           string               `xml:"title"`
			Artist          string               `xml:"artist"`
			Album           string               `xml:"album"`
			Path            string               `xml:"path"`
			MIME            string               `xml:"mime"`
			Genre           string               `xml:"genre"`
			Description     string               `xml:"description"`
			Created         string               `xml:"created"`
			Updated         string               `xml:"updated"`
			PublicationDate string               `xml:"publicationDate"`
			DOI             string               `xml:"doi"`
			ExtendedMetadata map[string]string   `xml:"extendedMetadata>item"`
		}

		// Convert ExtendedMetadata to map[string]string for XML
		extMeta := make(map[string]string)
		for k, v := range info.ExtendedMetadata {
			extMeta[k] = fmt.Sprintf("%v", v)
		}

		data := AudioInfoXML{
			ID:              info.ID,
			Title:           info.Title,
			Artist:          info.Artist,
			Album:           info.Album,
			Path:            info.Path,
			MIME:            info.MIME,
			Genre:           info.Genre,
			Description:     info.Description,
			Created:         info.Created.Format(time.RFC3339),
			Updated:         info.Updated.Format(time.RFC3339),
			PublicationDate: info.PublicationDate,
			DOI:             info.DOI,
			ExtendedMetadata: extMeta,
		}
		xmlData, err := xml.MarshalIndent(data, "", "  ")
		if err != nil {
			log.Fatalf("Error marshaling to XML: %v", err)
		}
		fmt.Println(xml.Header + string(xmlData))
	}
}

func handleUpdate(collection *audioinfo.Collection, args []string, format OutputFormat) {
	if len(args) < 1 {
		fmt.Println("Please provide an ID to update")
		return
	}

	id := args[0]

	// In a real implementation, you would collect updated information here
	// For now, we'll just print a message
	switch format {
	case FormatText:
		fmt.Printf("Update functionality for ID %s would go here\n", id)
		fmt.Println("This would prompt for or accept new metadata values")
	case FormatJSON:
		fmt.Printf(`{"status": "info", "message": "Update functionality for ID %s would go here", "action": "This would prompt for or accept new metadata values"}`+"\n", id)
	case FormatYAML:
		fmt.Printf(`status: info
message: Update functionality for ID %s would go here
action: This would prompt for or accept new metadata values`+"\n", id)
	case FormatXML:
		fmt.Printf(`<response><status>info</status><message>Update functionality for ID %s would go here</message><action>This would prompt for or accept new metadata values</action></response>`+"\n", id)
	}
}

func handleDelete(collection *audioinfo.Collection, args []string, format OutputFormat) {
	if len(args) < 1 {
		fmt.Println("Please provide an ID to delete")
		return
	}

	id := args[0]
	err := collection.Delete(id)
	if err != nil {
		log.Fatalf("Error deleting entry: %v", err)
	}

	switch format {
	case FormatText:
		fmt.Printf("Successfully deleted entry with ID %s\n", id)
	case FormatJSON:
		fmt.Printf(`{"status": "success", "message": "Successfully deleted entry with ID %s"}`+"\n", id)
	case FormatYAML:
		fmt.Printf(`status: success
message: Successfully deleted entry with ID %s`+"\n", id)
	case FormatXML:
		fmt.Printf(`<response><status>success</status><message>Successfully deleted entry with ID %s</message></response>`+"\n", id)
	}
}
