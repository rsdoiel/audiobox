

# audiobox

audiobox is a Go + TypeScript application for managing an audio collection. It stores metadata in a SQLite3 database (using schema.org AudioObject / MusicRecording vocabulary), serves a web UI from an embedded file system, and streams audio over HTTP. It uses "https://github.com/dhowden/tag" to extract embedded metadata from audio files.

This software is for Lourdes.

## Release Notes

- version: 0.0.4
- status: active
- released: 2026-05-29

- Folders tab: two-level browse tree (root → sub-folder) with per-folder ON/OFF toggles; exclusion state is persisted server-side in audio.yaml and applied automatically to Albums, Artists, and Titles browse views
- Playlists: save the current queue as a named playlist; load or delete saved playlists from a new Playlists tab; backend schema adds playlists and playlist_tracks tables
- Network sharing: Share button enables LAN access on a chosen IPv4 address; remoteAccessMiddleware restricts write operations to loopback clients; share address persisted in config
- Relative path storage: content_url values are stored relative to AudioDir; automatic migration on DB open converts any absolute paths
- Search improvements: grouped results (Albums / Artists / Titles / Tracks) with per-group Add All and per-item add/remove queue buttons
- Player UX: always-visible now-playing panel, folder context line, queue runtime estimate, per-item remove, auto-advance on track end, no-autoplay on queue add
- Scan / sweep elapsed-time indicator during async operations
- Bug fixes: scan/sweep/share status buttons now capture DOM elements at click time to prevent null-reference errors in async poll callbacks; share listener restart delayed 300 ms to ensure HTTP response is delivered before connection closes


### Authors

- Doiel, R. S.



## Software Requirements

- Go >= 1.26.2

### Software Suggestions

- CMTools >= 0.0.45b
- Pandoc >= 3.1
- GNU Make >= 3
- Deno >= 2.4.0
- FFmpeg >= 6.0



## Related resources



- [Getting Help, Reporting bugs](https://github.com/rsdoiel/audiobox/issues)
- [LICENSE](https://www.gnu.org/licenses/agpl-3.0.txt)
- [Installation](INSTALL.md)
- [About](about.md)

