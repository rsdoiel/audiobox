
./bin/audiobox(1) — audio collection metadata manager

SYNOPSIS

  ./bin/audiobox [OPTIONS] ACTION [COLLECTION.yaml] [PARAMETERS]

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

  sweep COLLECTION.yaml
    Remove database records whose audio files are no longer present on disk.

  help [ACTION]
    Display detailed help for an action.

EXAMPLES

  ./bin/audiobox init mymusic ~/Music "My personal archive"
  ./bin/audiobox scan mymusic.yaml
  ./bin/audiobox sweep mymusic.yaml
  ./bin/audiobox list mymusic.yaml artists
  ./bin/audiobox search mymusic.yaml "Bach"
  ./bin/audiobox show mymusic.yaml 550e8400-e29b-41d4-a716-446655440000
  ./bin/audiobox delete mymusic.yaml 550e8400-e29b-41d4-a716-446655440000

SEE ALSO

  ./bin/audiobox(1) man page, https://github.com/rsdoiel/audiobox

