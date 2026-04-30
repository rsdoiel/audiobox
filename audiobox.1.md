
./bin/audiobox(1) — audio collection metadata manager

SYNOPSIS

  ./bin/audiobox [OPTIONS] ACTION [PARAMETERS]

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
    Start a localhost web server for the collection.

  sweep
    Remove database records whose audio files are no longer present on disk.

  help [ACTION]
    Display detailed help for an action.

EXAMPLES

  ./bin/audiobox init
  ./bin/audiobox scan
  ./bin/audiobox sweep
  ./bin/audiobox list artists
  ./bin/audiobox search "Bach"
  ./bin/audiobox show 550e8400-e29b-41d4-a716-446655440000
  ./bin/audiobox delete 550e8400-e29b-41d4-a716-446655440000

SEE ALSO

  ./bin/audiobox(1) man page, https://github.com/rsdoiel/audiobox

