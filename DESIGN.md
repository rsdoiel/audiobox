# audiobox Design Document

## Overview

audiobox is a local audio collection manager. It indexes audio files from a
directory tree into a SQLite3 database, exposes a JSON HTTP API, and serves a
self-contained web UI for browsing and playback. A terminal (TUI) player is
also provided. The entire system ships as a single Go binary with the frontend
embedded.

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│  audiobox binary                                    │
│                                                     │
│  cmd/audiobox/main.go   ← CLI entry point           │
│                                                     │
│  audiobox package (Go)                              │
│  ├── audiobox.go        ← Collection, CRUD, search  │
│  ├── config.go          ← CollectionConfig, DB init │
│  ├── server.go          ← HTTP API, listener mgmt   │
│  ├── player.go          ← TUI player                │
│  └── types.go           ← AudioInfo, Agent, Album   │
│                                                     │
│  htdocs/ (embedded via go:embed)                    │
│  ├── index.html                                     │
│  └── module/audiobox.js ← bundled from TypeScript   │
└─────────────────────────────────────────────────────┘

TypeScript sources (compiled separately, not part of Go build)
├── audiobox_api.ts        ← typed fetch wrapper
├── audiobox_player.ts     ← <audiobox-player> web component
└── deno.json              ← build: deno bundle → htdocs/module/audiobox.js
```

### Data flow

```
Audio files on disk
      │ ScanDirectories()
      ▼
SQLite3 database (audio.db)
  ├── audio_files table     ← schema.org AudioObject/MusicRecording metadata
  ├── search_index (FTS5)   ← title, album, genre, artist for fast text search
  ├── playlists table       ← (planned) saved playlists
  └── playlist_tracks table ← (planned) ordered track membership
      │ HTTP JSON API
      ▼
<audiobox-player> web component (Shadow DOM)
  ├── Browse panel    ← Albums / Artists / Titles / Folders tabs
  ├── Player          ← always-visible transport controls
  ├── Queue panel     ← FIFO play queue with shuffle, clear, runtime estimate
  └── Library panel   ← Scan, Sweep, Share controls
```

---

## Data Model

### audio_files

| Column             | Type      | Notes                                      |
|--------------------|-----------|--------------------------------------------|
| id                 | TEXT PK   | UUID v4                                    |
| schema_type        | TEXT      | "AudioObject" or "MusicRecording"          |
| name               | TEXT      | recording title                            |
| description        | TEXT      |                                            |
| content_url        | TEXT UNIQ | absolute path to file on disk              |
| encoding_format    | TEXT      | MIME type (audio/flac, audio/mpeg, …)      |
| duration           | TEXT      | ISO 8601 (PT3M45S); set on ingest          |
| date_published     | TEXT      | year or full date from tags                |
| in_language        | TEXT      |                                            |
| genre              | TEXT      |                                            |
| identifiers        | JSON      | []Identifier (ISRC, DOI, ARK, …)          |
| by_artist          | JSON      | []Agent (Person or Organization)           |
| in_album           | TEXT      |                                            |
| disc_number        | INTEGER   | default 0                                  |
| track_number       | INTEGER   | default 0                                  |
| isrc_code          | TEXT      |                                            |
| recording_of       | TEXT      | composition title when different from name |
| checksum           | TEXT      | hex SHA-256                                |
| checksum_algorithm | TEXT      | "sha256"                                   |
| created            | TIMESTAMP |                                            |
| updated            | TIMESTAMP |                                            |

### search_index (FTS5)

Columns: `audio_id` (UNINDEXED), `name`, `in_album`, `genre`, `recording_of`,
`artist_names`. Tokenizer: `unicode61 remove_diacritics 1`.

### playlists (planned)

| Column  | Type      | Notes         |
|---------|-----------|---------------|
| id      | TEXT PK   | UUID v4       |
| name    | TEXT      |               |
| created | TIMESTAMP |               |

### playlist_tracks (planned)

| Column      | Type    | Notes                                         |
|-------------|---------|-----------------------------------------------|
| playlist_id | TEXT FK | → playlists(id) ON DELETE CASCADE             |
| position    | INTEGER | 1-based ordering                              |
| audio_id    | TEXT FK | → audio_files(id) ON DELETE CASCADE           |

---

## CollectionConfig (audio.yaml)

```yaml
name: audio
description: "Audio collections for alice"
database: audio.db       # relative to this file
audioDir: ~/Audio
htdocs: ""               # empty → use embedded htdocs
port: 8010
corsOrigin: ""           # "" → "*"; "off" → no CORS headers
shareAddress: ""         # preferred LAN IP for share mode; empty = not configured
```

`shareAddress` is written by `audiobox server share IP_ADDRESS` and read by
`audiobox server share` on subsequent runs. It is never set by the web UI
(which sends the address via `POST /api/share/on`).

---

## HTTP API Reference

All endpoints are relative to the server address (default `http://127.0.0.1:8010`).
JSON bodies use `Content-Type: application/json`.

### Collection

```
GET  /api/status
```
Returns collection metadata. Does not include share state (see `/api/share/status`).

```json
{
  "initialized": true,
  "version": "0.1.0",
  "collection_name": "audio",
  "audio_dir": "/home/alice/Audio",
  "track_count": 1234
}
```

```
POST /api/init
```
Initialises or upgrades the `~/Audio` collection layout.

### Browse

```
GET /api/list/albums     → []AlbumEntry
GET /api/list/artists    → []string
GET /api/list/titles     → []string
GET /api/list/folders    → []FolderEntry   (planned)
```

`AlbumEntry`: `{ name, displayName, dir }`
`FolderEntry`: `{ path, name, trackCount }`

```
GET /api/list/folder-tracks?dir=ENCODED_PATH → []AudioInfo   (planned)
```

Returns all tracks whose `content_url` begins with `dir`.

### Search

```
GET /api/search?q=QUERY → []AudioInfo
```

Query syntax: plain term, `"quoted phrase"`, `/regex/`, `field:term`,
`field:"phrase"`, `field:/regex/`. Field aliases: `title`/`name`, `album`,
`artist`, `genre`, `recording`/`recording_of`.

### Records

```
GET    /api/show/{id}    → AudioInfo
DELETE /api/show/{id}    → { "status": "deleted", "id": "…" }
```

### Audio streaming

```
GET /api/audio/{id}
```

Streams the audio file. Supports HTTP Range requests for seeking.
Returns 403 if `content_url` is outside `audioDir`.

### Scan / Sweep (write — blocked in read-only share mode)

```
POST /api/scan           → 202 { "status": "started", "started_at": "…" }
GET  /api/scan/status    → AsyncStatus
POST /api/sweep          → 202 { "status": "started", "started_at": "…" }
GET  /api/sweep/status   → AsyncStatus + records_removed
```

`AsyncStatus`: `{ status: "idle"|"running"|"completed"|"error", started_at?, completed_at?, error? }`

### Playlists (planned — write endpoints blocked in read-only share mode)

```
GET    /api/playlists         → []PlaylistInfo
POST   /api/playlists         body: { "name": "…", "trackIds": ["uuid", …] }
GET    /api/playlists/{id}    → []AudioInfo
DELETE /api/playlists/{id}
```

`PlaylistInfo`: `{ id, name, trackCount, created }`

### Network sharing

```
GET  /api/share/status
```

```json
{
  "sharing": false,
  "share_address": "",
  "share_url": ""
}
```

When `sharing` is true:

```json
{
  "sharing": true,
  "share_address": "192.168.1.5",
  "share_url": "http://192.168.1.5:8010"
}
```

```
GET /api/share/addresses → []string
```

Returns the machine's non-loopback IPv4 addresses, for the UI address picker.

```json
["192.168.1.5", "10.0.0.12"]
```

```
POST /api/share/on    body: { "address": "192.168.1.5" }
POST /api/share/off
```

Both respond immediately (before the listener restarts):

```json
{ "status": "restarting", "poll_url": "http://127.0.0.1:8010/api/share/status" }
```

Both endpoints are **blocked in read-only share mode** (non-loopback callers
receive 403).

### Utility

```
POST /api/shutdown
GET  /api/help          → text/markdown
```

---

## Network Sharing Design

### Motivation

By default the server binds to `127.0.0.1` (loopback only). Share mode
rebinds to `0.0.0.0` so that other devices on the local network can stream
audio from the collection. The owner retains full administrative access;
remote clients are restricted to read-only operations.

### Listener architecture

A single `http.Server` runs inside a managed goroutine. A `listenerManager`
struct owns the goroutine and communicates via a control channel:

```go
type listenRequest struct {
    addr string        // e.g. "127.0.0.1:8010" or "0.0.0.0:8010"
    done chan error     // closed after the new listener is up
}

type listenerManager struct {
    port    int
    handler http.Handler
    ctrl    chan listenRequest
    mu      sync.RWMutex
    sharing bool
    shareAddr string
}
```

On receiving a `listenRequest` the manager:
1. Calls `srv.Shutdown(ctx)` on the current server (5 s timeout).
2. Creates a new `http.Server` bound to `req.addr`.
3. Starts it in a goroutine.
4. Closes `req.done` when the new server is accepting connections.

The outer `Serve()` call starts the manager and blocks until the global
shutdown channel fires, at which point both the manager and the current
server are stopped.

### Access control

A `remoteAccessMiddleware` wraps the mux. For every request it checks
`r.RemoteAddr`:

- If the IP is `127.0.0.1` or `::1` → full access, pass through unchanged.
- Otherwise → read-only: `POST`, `DELETE`, and `PUT` methods return
  `403 Forbidden` with `{"error":"read-only in share mode"}`.

This also blocks `POST /api/share/on` and `POST /api/share/off` from LAN
clients.

### CLI

```
audiobox server              # binds 127.0.0.1:8010
audiobox server share        # binds 0.0.0.0:8010 using saved shareAddress
audiobox server share 192.168.1.5   # saves shareAddress, binds 0.0.0.0:8010
```

`audiobox server share` without an address reads `shareAddress` from
`audio.yaml` and exits with an error if that field is empty.

`audiobox server share IP_ADDRESS` writes `shareAddress: IP_ADDRESS` to
`audio.yaml` before starting.

### Web UI flow

**Enabling share:**
1. Owner clicks Share button → UI calls `GET /api/share/addresses`.
2. Dialog shows list of available LAN IPs; owner picks one.
3. UI calls `POST /api/share/on` with `{"address":"192.168.1.5"}`.
4. Server responds with `{"status":"restarting","poll_url":"http://127.0.0.1:8010/api/share/status"}`.
5. UI polls `poll_url` until `sharing === true`.
6. Share button shows: `Sharing: http://192.168.1.5:8010 [Copy] [Disable]`.
7. Owner stays on `http://127.0.0.1:8010` — full access throughout (0.0.0.0 covers loopback).

**Disabling share:**
1. Owner clicks Disable → UI calls `POST /api/share/off`.
2. Server responds with `{"status":"restarting","poll_url":"http://127.0.0.1:8010/api/share/status"}`.
3. UI polls `poll_url` until `sharing === false`.
4. Share button returns to "Share" state.

---

## Browse & Queue Design

### Browse drill-down

Clicking a browse item (album, artist, title, folder) opens a **track detail
view** inside the list panel — it does not modify the queue or start playback.
The detail view shows:

```
[← Back]  Albums: Goldberg Variations          [⊕ Add All]
─────────────────────────────────────────────────────────
Aria                       Glenn Gould · Goldberg  [⊕]
Variation 1                Glenn Gould · Goldberg  [⊕]
…
```

`⊕` on an individual track adds that one track to the queue.
`⊕ Add All` adds all tracks in the detail view to the queue.
`← Back` returns to the browse list without changing the queue.

Queue contents are never modified by browsing — only by explicit `⊕` actions.

### Queue behaviour

- Items are added to the tail of the queue via `⊕` buttons.
- When the first item is added to an empty queue the player loads (but does
  not start) that track — `audioEl.src` is set, play does not begin until
  the user presses ▶.
- When a track finishes playing (the `ended` event) it is **removed** from the
  queue. The next item shifts to the front and loads automatically; playback
  continues without pause.
- Shuffle applies a Fisher-Yates shuffle to all unplayed tracks (those after
  `currentIndex`).
- The queue header shows track count and total estimated runtime derived from
  `duration` fields (ISO 8601; tracks with no duration contribute 0).
- Individual items can be removed via a `×` button on each queue row.
- A Clear button empties the queue and resets the player to idle.

### Saving and loading playlists

- "Save as Playlist" button in the queue header prompts for a name and calls
  `POST /api/playlists` with the current queue's track IDs.
- A Playlists section (or tab) lists saved playlists. Clicking a playlist
  calls `GET /api/playlists/{id}` and appends the tracks to the queue via
  `_addToQueue`.

---

## Search Design

### Current behaviour

`GET /api/search?q=QUERY` returns a flat `[]AudioInfo` ranked by relevance.
The query engine tries FTS5 first and falls back to Levenshtein fuzzy search.

### Planned: typed result groups

The frontend issues up to four parallel searches for a user query `q`:

| Search | API call |
|--------|----------|
| Album matches | `GET /api/search?q=album:"q"` |
| Artist matches | `GET /api/search?q=artist:"q"` |
| Title matches | `GET /api/search?q=title:"q"` |
| Folder matches | client-side filter of loaded folder names |

Results are grouped by category in the UI:

```
Albums matching "bach"
  Bach: The Goldberg Variations  [⊕]
  Bach: Brandenburg Concertos    [⊕]

Artists matching "bach"
  Johann Sebastian Bach          [⊕]

Titles matching "bach"
  Bachianas Brasileiras No. 5    [⊕]
```

Each entry has a `⊕` to add to queue and a `⊖` to remove matching tracks from
the queue. The `⊕` on a group header adds all tracks for that item.

This requires no new backend endpoints — all field-scoped query syntax already
exists.

---

## Folder Support Design

### Motivation

The existing Albums / Artists / Titles browse tabs are tag-driven. Folders
provide a filesystem-native view that works correctly even when tags are
incomplete, and enables include/exclude by directory (e.g. excluding a
"Christmas Music" folder outside the holiday season).

### Backend

New method `GetFolders() ([]FolderEntry, error)` on `Collection`:

```go
type FolderEntry struct {
    Path       string `json:"path"`
    Name       string `json:"name"`        // deslugified last component
    TrackCount int    `json:"trackCount"`
}
```

Implemented by querying distinct `filepath.Dir(content_url)` values and
counting tracks per directory.

New endpoints:
- `GET /api/list/folders` — returns `[]FolderEntry` sorted by `Path`
- `GET /api/list/folder-tracks?dir=ENCODED_PATH` — returns `[]AudioInfo`
  for all tracks under `dir` (LIKE prefix match)

### Frontend

A "Folders" tab is added to the browse panel alongside Albums / Artists /
Titles. Each folder row shows its name and track count; the `⊕` button adds
all tracks in that folder to the queue. Clicking the row drills down to the
track detail view.

---

## Player Design

### Always visible

The now-playing panel is always rendered. When no track is loaded it shows
an idle state:

```
(No track loaded)
[⏮ disabled]  [▶ disabled]  [⏭ disabled]
```

The delete-record button is removed from the player entirely. Record deletion
is a library management concern, not a playback concern.

### Context display

When a track is loaded the player shows four lines:

```
Goldberg Variations: Aria                   ← track title
Glenn Gould · Goldberg Variations           ← artist · album
📁 1981-Goldberg-Variations                 ← containing folder name
```

The folder name is derived from the last directory component of `ContentURL`.

---

## Scan / Sweep Progress

The scan and sweep status polling loops display elapsed time during the
operation:

```
Scanning… (12s)
Sweeping… (3s)
```

Elapsed time is computed from `started_at` returned in the async status
response. On completion the status shows "Scan complete" or
"N stale record(s) removed".
