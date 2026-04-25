
./bin/audioinfo(1) — audio collection metadata manager

SYNOPSIS

  ./bin/audioinfo [OPTIONS] ACTION [COLLECTION.yaml] [PARAMETERS]

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
    Walk the collection's rootDir and ingest every audio file found.

  list COLLECTION.yaml [albums|artists|titles]
    List distinct albums, artists, or titles (default: albums).

  search COLLECTION.yaml QUERY
    Search records by title, album, or artist.

  show COLLECTION.yaml ID
    Display full metadata for the record with the given UUID.

  delete COLLECTION.yaml ID
    Remove the record with the given UUID from the collection.

  help [ACTION]
    Display detailed help for an action.

EXAMPLES

  ./bin/audioinfo init mymusic ~/Music "My personal archive"
  ./bin/audioinfo scan mymusic.yaml
  ./bin/audioinfo list mymusic.yaml artists
  ./bin/audioinfo search mymusic.yaml "Bach"
  ./bin/audioinfo show mymusic.yaml 550e8400-e29b-41d4-a716-446655440000
  ./bin/audioinfo delete mymusic.yaml 550e8400-e29b-41d4-a716-446655440000

SEE ALSO

  ./bin/audioinfo(1) man page, https://github.com/rsdoiel/audioinfo

