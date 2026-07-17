
# Action Items

## Bugs

- [x] When searching for an artist need to able to queue albums in additional to individual songs — clicking an artist (or filtering Albums by artist name) now shows that artist's albums with a "+" add-to-queue button per album, resolved via exact album directory rather than tag text.
- [x] When picking the Artist button, the albums for an artist should be shown first when drilling down to see what the Artist is associated with, albums should have the plus button tto add a whole album to queue — Artists-tab drilldown now shows albums-first (falls back to a flat track list only when a track can't be resolved to an album directory).
- [x] When I add the "Travel" album to a playlist is pickups more than Jake Shimabukuro's "Travel" it also adds ZBS's "Travels with Jack" — root cause was two bugs: (1) field-scoped exact queries were silently falling back to Levenshtein fuzzy matching on zero results, conflating "Travel"/"Travels"; (2) Albums-tab clicks/add-to-queue resolved via tag-based search instead of the exact album directory already known from the browse list. Both fixed; see SearchAudioFiles/GetTracksByAlbumDir and the Albums-tab wiring in audiobox_player.ts.
- [x] When I search for Jake or Shimabukuro I am only getting back one of his albums, even when the "Artist" button was previously selected — root cause: `.wav` isn't a tag format dhowden/tag supports, so a `.wav` album stored one level deep (`Albums/<Album>/track.wav`, no separate Artist folder) fell back to artist "Unknown". Fixed by adding a RIFF "LIST"/"INFO" chunk parser (wavtags.go) used as a fallback metadata source for `.wav` files before the path-derived/"Unknown" fallback. Note: this recovers the artist only when the `.wav` file actually carries an IART tag in its RIFF INFO chunk — a `.wav` with no embedded metadata at all still has no way to know the artist and correctly falls back to "Unknown".
- [x] When I type in an artist name under the Albums list, it should return a list of albums by the Artest name, do I need to have query prefix like "artist:Shimabukuro" to do that? — no prefix needed: typing plain text while the Albums tab is active now filters by album name OR artist name (matches both `album:"…"` and `artist:"…"`); an explicit field prefix (album:/artist:/title:/genre:) still works and is documented in the search box's placeholder/tooltip.
- [x] The scan action has no visual indicator that is had completed, a progress indicator would be nice to include — already implemented (web UI shows a live elapsed-time status with completed/error styling; CLI prints "Scanning…"/"Scan complete."); verified by code inspection, no change needed.
- [x] The sweep action has no visual indicator that it has completed, a progress indicator would be nice to include — already implemented (web UI shows live status + removed-record count; CLI prints a completion summary); verified by code inspection, no change needed.
- [x] When I run `audiobox scan` I am seeing errors like ```MACMINI-RD:~ rsdoiel$ audiobox scan
Scanning…
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/01 143 (Kelly's Song) 2011.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/02 Bohemian Rhapsody.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/03 Bring Your Adz.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/04 Boy Meets Girl.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/05 Go For Broke.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/06 Trapped 2010.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/07 Variation On A Dance 2010.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/08 Pianoforte 2010.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/09 Five Dollars Unleaded 2010.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/10 Ukulele Bros..wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/11 Hallelujah.wav: no tags found
audiobox: warning: could not read tags from /Users/rsdoiel/Audio/Music/Albums/Peace-Love-Ukulele/12 Bohemian Rhapsody - Live Version.wav: no tags found
```
  Fixed for `.wav` files that carry RIFF INFO tags (same fix as the Jake Shimabukuro bug above — see wavtags.go); a genuinely untagged `.wav` still logs this warning, correctly, since there's nothing to read.
- [x] An Album's song list for ```MACMINI-RD:~ rsdoiel$ ls -1 Audio/Music/Albums/Milton-plus-Esperanza/
01 - the music was there.mp3
02 - Cais.mp3
03 - Late September.mp3
04 - Outubro.mp3
05 - A Day in the Life.mp3
06 - Interlude for Saci.mp3
07 - Saci [feat. Guinga].mp3
08 - Wings for the Thought Bird [feat. Elena Pinderhughes & Orquestra Ouro Preto].mp3
09 - The Way You Are.mp3
10 - Earth Song [feat. Dianne Reeves].mp3
11 - Morro Velho [feat. Orquestra Ouro Preto].mp3
12 - Saudade Dos Aviões Da Panair (Conversando No Bar) [feat. Lianne La Havas & Maria Gadú &.mp3
13 - Um Vento Passou (para Paul Simon) [feat. Paul Simon].mp3
14 - Get It By Now.mp3
15 - outro planeta.mp3
16 - When You Dream [feat. Carolina Shorter].mp3
``` doesn't show up when in look for it in the individual web view for Albums, it does show up if I search by artist
  Fixed: the Albums tab now resolves an album's tracks by its exact directory (GetTracksByAlbumDir / GET /api/list/album-tracks) instead of a tag-based search, so a tag/directory-name mismatch (e.g. tag text differing from the deslugified folder name) can no longer make an album appear empty.
- [x] The Album Travels, ```MACMINI-RD:~ rsdoiel$ ls -1 Audio/Music/Albums/Travels
01 Departure Suite - part I.wav
02 Train Ride.wav
03 Low Rider.wav
04 Travels.wav
05 Interlude 1.wav
06 Passport.wav
07 Hi'ilawe.wav
08 Everything Is Better With You (feat. The Side Order Band).wav
09 Departure Suite - parts II & III.wav
10 'Oama.wav
11 I'll Be There.wav
12 Haven't We Been Here Before.wav
13 Interlude 2.wav
14 Kawika.wav
15 Red-Eye.wav
16 Ichigo Ichie.wav
17 Dinner & A Movie.wav
18 Nada Sousou.wav
19 Early Song.wav
20 Tip Toe.wav
``` is also lists content from Travels with Jack in the Web View (might be an error in the SQL or how the metadata is stored in the SQLite3 database)
  Fixed: same root cause as the "Travel"/"Travels with Jack" playlist bug above (fuzzy fallback + tag-based album resolution) — directory-exact resolution means "Travels" can never again pull in a sibling directory like "Travels with Jack".
- [x] The folder view in the web UI does not let me drill down into the folders that are inside `Music/Album` — the folder tree was hardcoded to two levels (root + one child); it now expands every directory at every depth as its own row (see buildFolderTree), each independently selectable, toggleable, and queueable.
- [x] The search feature should be clearer about constraints/context of search (searching by album name, searching by artist, etc). — the search box now has a placeholder and tooltip documenting album:/artist:/title:/genre: prefixes and /regex/ syntax.

## Next

- [x] It would be nice to build playlists based on artist name, date of first release (example 1965 to 1975 music) — "Build Playlist…" button in the Library panel: prompts for an artist-name substring and/or a release-year range, queries GET /api/tracks (QueryTracks in track_query.go), and adds every match to the queue so it can be reviewed/reordered/shuffled and saved as a playlist via the existing Save as Playlist button. This is a one-time build from criteria, not a persistent/dynamic "smart playlist" that re-evaluates later — say the word if you want that instead.
- [x] There needs to be a means of importing and exporting a playlist as OPML files — GET /api/playlists/{id}/opml downloads a playlist as OPML 2.0; POST /api/playlists/import-opml uploads one and creates a new playlist. Each track becomes one &lt;outline&gt; (text=title, url=content_url, category=artist, description=album); import matches outlines back to tracks by content_url and reports imported/skipped counts for entries that no longer match a file in this collection. Web UI: "Export" (⇩) link per playlist row, "Import Playlist (OPML)" button in the Library panel. Uses github.com/rsdoiel/opml.
- [x] An A-Z list jump option or filter needs to be available for Albums, Artists and Titles — a jump bar (A-Z + "#") now appears above the list on the Albums/Artists/Titles tabs; clicking a letter scrolls to the first matching row. Letters with no matches in the current list render disabled. Applies after Albums-tab artist/album filtering too, since it's derived from whatever is currently rendered.
- [x] Sort order for Albums and Titles should use the librarian style ordering where words like "the" and "a" are ignored but still shown in the result, "The Dave Mathews Band" would be sorted as "Dave Mathews Band, The" so it shows up in the "D" list not "T" list — GetAlbumEntries/GetArtists/GetTitles now sort by a librarian key that ignores a leading standalone "The"/"A"/"An" (librarianSortKey in librarian_sort.go), so "The Dave Matthews Band" files under "D". Design note: the *displayed* name is left exactly as-is ("The Dave Matthews Band"), not rearranged to "Dave Matthews Band, The" — only the sort/jump-bar position changes. Say the word if you want the display text itself rearranged too; the sort-key plumbing is already there to build on.
- [x] Use an OPML file to import/export playlists — duplicate of the bullet above; see there.
- [x] Revise shuffle constraints, if an audio file is playing it and a user press shuffle for the queue it doesn't change that it is playing or the onces that have played. If no audio is playing and I press shuffle for the queue then the whole queue is shuffled and the selected audio file is set to the new top of list. — the "currently playing/already played never move" half already worked; fixed the other half: when nothing is currently selected (currentIndex -1, e.g. after removing the playing track), the whole queue now shuffles and the new top track is primed as selected (now-playing shown, audio loaded) without auto-playing, same as adding to an empty queue. Extracted as a pure, unit-tested shuffleQueue() function in audiobox_player.ts.
- [x] There needs to a way to explude specific folders from a playlist. — same "Build Playlist…" flow above accepts a one-off, comma-separated exclude-folders list distinct from (and unaffected by) the persistent Folders-tab browse-exclude toggle.
- [x] Adding content to the queue is done by pressing the "+" (add to queue) button only — already true: clicking a browse row drills down/shows details only, it never adds to the queue; only the "+"/"⊕" controls do. Verified by code inspection, no change needed.
- [x] There should be a switch or action to allow audiobox to be reachable via HTTP from machines on the same network (share audio button) — already implemented: Share button in the Library panel, POST /api/share/on|off, GET /api/share/status|addresses; remote non-loopback clients are read-only (POST/PUT/DELETE blocked). Verified by code inspection, no change needed.
- [x] Add a Folders list (with include/exclude options) in additional to albums, artists and titles — already implemented: Folders tab (GetFolders/GetFolderTracks) with a per-folder ON/OFF exclude toggle at every depth (see buildFolderTree); excluded folders are filtered out of the Albums/Artists/Titles tabs via excludeFolders. Verified by code inspection, no change needed.
  - This would solve the holiday music problem for when it is not the holiday season
- [x] Player improvements — all three already implemented; verified by code inspection, no change needed.
  - [x] The player should always be visible — the now-playing panel is unconditionally rendered (no hide/collapse state); covered by a template test.
  - [x] Remove the "delete" button from the player — there is no delete button anywhere in the player/now-playing controls; nothing to remove.
  - [x] The player should show the folder, album (if available ), artitle (if available) and title (if available) of the audio file to be played — _showNowPlaying sets title, "artist — album", and "📁 folder" from the track's ContentURL.
- [x] Search Improvments — substantially implemented via the grouped search view; one nuance below. Verified by code inspection, no change needed.
  - [x] searching needs to work for taking you to either the folder list, album list, artist and title depending on matching results. — implemented differently than literally described: instead of switching the active tab, a single search shows all matching groups (Albums/Artists/Titles/Tracks/Folders) at once, which surfaces the same information without forcing a choice. Say the word if you actually want tab-switching behavior instead.
  - [x] matching search results to include folders that match, album, artist and title (and indicate what type of match it is) — _runGroupedSearch groups results under labeled headers (Albums/Artists/Titles/Tracks/Folders), including a dedicated folder-match group.
    - [x] each matched result should have "+" button for adding that content to the queue and "-" button to remove the matching content from the queue — every track row in a search group has both row-add-btn (⊕) and row-remove-btn (⊖); each group header also has an "Add All" button.
    - [x] The individual title or album should have the "+" button to add to the queue — same as above; Albums/Titles groups included.
- [x] Queue improvements — all seven already implemented; verified by code inspection, no change needed.
  - [x] Add a clear button for the queue — .clear-queue-btn.
  - [x] For individual items in the queue there can be a "-" (remove from queue) button — .queue-remove-btn (×) per queue row.
  - [x] As items finish being played from the queue they should be removed from it — the "ended" handler splices the finished track out of the queue and auto-advances.
  - [x] The queue doesn't auto-play, first item should be ready in the player but the player doesn't start until the play button is pressed — _addToQueue/_shuffleQueue prime the first/new-top track (src loaded, shown as now-playing) via _refreshPlayState(false), never auto-playing.
  - [x] I should be able to do a shuffle on the queue after getting the content I want to play into it — .shuffle-btn, fixed this session (see the shuffle-constraints bullet above).
  - [x] The queue should indicate an estimate of run time for entries — queue title shows "Queue (N · ~H:MM:SS)", summed from each track's duration.
  - [x] You should be able to save the queue as a playlist — .save-playlist-btn.
  - [x] A playload should be loadable into the queue — playlist row's ▶ button loads a saved playlist into the queue.

