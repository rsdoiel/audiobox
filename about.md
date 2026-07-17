---
title: audiobox
abstract: |-
  audiobox is a Go + TypeScript application for managing an audio collection. It stores metadata in a SQLite3 database (using schema.org AudioObject / MusicRecording vocabulary), serves a web UI from an embedded file system, and streams audio over HTTP. It uses "https://github.com/dhowden/tag" to extract embedded metadata from audio files.

  This software is for Lourdes.
authors:
  - family_name: Doiel
    given_name: R. S.
    id: https://orcid.org/0000-0003-0900-6903



repository_code: https://github.com/rsdoiel/audiobox
version: 0.0.5
license_url: https://www.gnu.org/licenses/agpl-3.0.txt

programming_language:
  - Go >= 1.26.2
  - TypeScript


date_released: 2026-07-17
---

About this software
===================

## audiobox 0.0.5

- Album/folder resolution: Albums and Folders tabs now resolve tracks by exact directory instead of a tag-based search, fixing albums that appeared empty or pulled in tracks from a similarly-named sibling album (e.g. "Travel" vs "Travels with Jack")
- Search correctness: exact field-scoped queries (album:/artist:/title:) no longer silently fall back to fuzzy matching on zero results — only free-text queries get typo tolerance
- Artists tab: clicking an artist now shows their albums first (each with its own add-to-queue button) instead of a flat track list
- Albums tab: plain-text search now filters by album name or artist name, with no prefix required; explicit album:/artist:/title:/genre: prefixes documented in the search box
- Folders tab: the browse tree now expands every directory at every depth (was capped at two levels), each independently selectable, toggleable, and queueable
- Librarian-style sort order for Albums, Artists, and Titles: a leading "The"/"A"/"An" is ignored for filing purposes (display text is unchanged)
- A-Z jump bar on Albums/Artists/Titles: stays visible while scrolling; each entry is reachable from both its filed-under letter and its literal first-word letter
- Playlists: import/export as OPML 2.0 files (matched back to tracks by file path, not database ID, so playlists survive rescans); "Build Playlist…" generates a playlist from criteria (artist name substring, release year range, one-off folder exclusions)
- Queue shuffle: only protects tracks once playback has actually started — a freshly built, unplayed queue reshuffles fully instead of always keeping the same track first
- WAV metadata: added a RIFF "LIST"/"INFO" chunk parser as a fallback tag source for .wav files (previously unsupported), recovering artist/title/album/genre when embedded
- Fixed a metadata fallback bug where a library laid out as e.g. Music/Albums/<Album>/track.wav could misattribute the artist as the literal container folder name ("Music") instead of falling back correctly

## Authors

- [R. S. Doiel](https://orcid.org/0000-0003-0900-6903)






audiobox is a Go + TypeScript application for managing an audio collection. It stores metadata in a SQLite3 database (using schema.org AudioObject / MusicRecording vocabulary), serves a web UI from an embedded file system, and streams audio over HTTP. It uses "https://github.com/dhowden/tag" to extract embedded metadata from audio files.

This software is for Lourdes.

- [License](https://www.gnu.org/licenses/agpl-3.0.txt)
- [Code Repository](https://github.com/rsdoiel/audiobox)
  - [Issue Tracker](https://github.com/rsdoiel/audiobox/issues)

## Programming languages

- Go >= 1.26.2
- TypeScript




## Software Requirements

- Go >= 1.26.2


## Software Suggestions

- CMTools >= 0.0.45b
- Pandoc >= 3.1
- GNU Make >= 3
- Deno >= 2.4.0
- FFmpeg >= 6.0


