# audiobox Implementation Plan

Tasks are grouped into phases. Each phase is self-contained and releasable.
Within a phase, tasks are listed in dependency order — complete them top to
bottom. File paths are relative to the repo root.

---

## Phase 1 — Frontend UX improvements

All tasks in this phase touch only `audiobox_player.ts`. No Go changes.
After each task run `deno task bundle` and smoke-test in the browser.

### 1.1 Scan / sweep elapsed-time indicator

**Goal:** Show elapsed seconds during async operations instead of a static
"Scanning…" message.

**Files:** `audiobox_player.ts`

**Changes:**
- In `_pollScan`: when `s.status === "running"`, compute
  `Math.floor((Date.now() - new Date(s.started_at).getTime()) / 1000)` and
  set `status.textContent = \`Scanning… (\${elapsed}s)\``.
- Same pattern in `_pollSweep` → `"Sweeping… (Ns)"`.

---

### 1.2 Player always visible; remove delete button

**Goal:** The now-playing panel is always rendered. The delete-record button
is removed.

**Files:** `audiobox_player.ts`

**Changes:**
- Remove `.hidden` from `.now-playing` in `PLAYER_TEMPLATE`.
- Remove `.delete-track-btn` element from `PLAYER_TEMPLATE`.
- Remove `.delete-track-btn` CSS from `STYLES`.
- Remove the delete-button `addEventListener` block from `_bindControls`.
- Remove `deleteBtn.dataset.trackId = info.ID` from `_showNowPlaying`.
- Add `_initPlayer()` method: sets title to "(No track loaded)", clears sub
  and context, resets seek bar, calls `_refreshPlayState(false)`.
- Call `_initPlayer()` from:
  - `connectedCallback` (after status check succeeds, before `_loadTab`)
  - Clear queue handler (task 1.4)
  - When the last queued track finishes (task 1.4)

---

### 1.3 Player context: folder name

**Goal:** Show the folder containing the current track below the artist/album
line.

**Files:** `audiobox_player.ts`

**Changes:**
- In `_showNowPlaying`, derive folder name:
  ```typescript
  const parts = info.ContentURL.replace(/\\/g, "/").split("/");
  const folder = parts.length >= 2 ? parts[parts.length - 2] : "";
  ```
- Set `this.qs(".context-info").textContent = folder ? \`📁 \${folder}\` : ""`.
- Remove the `contextLabel` / `"From: …"` logic (no longer needed once
  drill-down is implemented in task 1.6).

---

### 1.4 Queue: clear button, per-item remove, auto-remove played, runtime estimate

**Goal:** Full queue management and display improvements.

**Files:** `audiobox_player.ts`

**Changes:**

**Template / styles:**
- Add `<button class="clear-queue-btn" disabled>Clear</button>` to
  `.queue-actions` in `PLAYER_TEMPLATE`.
- Change `.queue-item` to `display: flex; align-items: center; gap: 4px;`
  (removes `white-space: nowrap; overflow: hidden; text-overflow: ellipsis`
  from the row — those move to `.queue-item-name`).
- Add `.queue-item-name { flex: 1; white-space: nowrap; overflow: hidden;
  text-overflow: ellipsis; }`.
- Add `.queue-remove-btn { flex-shrink: 0; padding: 0 4px; border: none;
  background: transparent; color: #aaa; cursor: pointer; font-size: 14px; }`
  and `:hover { color: #e74c3c; }`.

**`_updateQueuePanel`:**
- Add `parseDurationSecs(iso: string): number` helper (parses PT…H…M…S).
- Sum durations of all items: `totalSecs = queue.reduce(…)`.
- Format total: `_fmtSecs(totalSecs)` — reuse existing helper.
- Queue title: `Queue (${n} · ~${totalStr})` or `Queue (${n})` when no
  durations available.
- Each `.queue-item` renders:
  ```html
  <div class="queue-item" data-queue-index="i">
    <span class="queue-item-name">title</span>
    <button class="queue-remove-btn" data-remove-index="i">×</button>
  </div>
  ```
- Enable/disable `.clear-queue-btn` based on `queue.length > 0`.

**Bindings:**
- Wire `.clear-queue-btn` click: `queue = []; currentIndex = -1;
  audioEl.pause(); audioEl.src = ""; _initPlayer(); _updateQueuePanel()`.
- Wire `.queue-remove-btn` via delegation on `.queue-list`: splice item at
  index, adjust `currentIndex` if needed, call `_updateQueuePanel()`.
  If the removed item was currently playing, pause and call `_initPlayer()`.

**Auto-remove on `ended` (in `_bindAudio`):**
- Replace current `ended` handler:
  ```typescript
  this.queue.splice(this.currentIndex, 1);
  if (this.currentIndex < this.queue.length) {
    this._playIndex(this.currentIndex);   // next item shifted into place
  } else {
    this.currentIndex = -1;
    this._refreshPlayState(false);
    this._initPlayer();
    this._updateQueuePanel();
  }
  ```

---

### 1.5 Queue does not auto-play when items are added

**Goal:** Adding items to the queue primes the player but does not start
playback. The user presses ▶ to begin.

**Files:** `audiobox_player.ts`

**Changes:**
- In `_addToQueue`, when the queue was empty and `currentIndex < 0`:
  ```typescript
  this.currentIndex = 0;
  this.audioEl.src = this.api.audioUrl(this.queue[0].ID);
  this._showNowPlaying(this.queue[0]);
  this._refreshPlayState(false);   // do NOT call audioEl.play()
  ```
- Remove `autoPlay` parameter from `_renderAudioList` (or remove the method
  entirely — see task 1.6).

---

### 1.6 Browse drill-down; search shows typed results

**Goal:** Clicking a browse item opens a track detail view in the list panel
without modifying the queue. Only explicit `⊕` actions add to the queue.
Search results are shown in the same drill-down style.

**Files:** `audiobox_player.ts`

**New state fields:**
```typescript
private drilldownTracks: AudioInfo[] = [];
private drilldownLabel  = "";
private currentBrowseTab = "albums";
```

**New styles (add to `STYLES`):**
```css
.drill-back-bar {
  display: flex; align-items: center; gap: 6px; padding: 6px 8px;
  background: #f0f0f0; border-bottom: 1px solid #e0e0e0; font-size: 12px;
}
.drill-back-btn {
  padding: 2px 8px; border: 1px solid #ccc; border-radius: 3px;
  background: #fff; cursor: pointer; font-size: 12px; flex-shrink: 0;
}
.drill-back-btn:hover { background: #e8e8e8; }
.drill-context-label {
  flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: #555;
}
.drill-add-all-btn {
  padding: 2px 8px; border: 1px solid #4a90d9; border-radius: 3px;
  background: #fff; color: #1a73e8; cursor: pointer; font-size: 12px; flex-shrink: 0;
}
.drill-add-all-btn:hover { background: #e8f0fe; }
```

**New `_showDrilldown(tracks, label)` method:**
- Stores tracks in `this.drilldownTracks`, label in `this.drilldownLabel`.
- Renders the list panel with a back bar and one `.list-item` per track
  (title + sub-line + `⊕` button using `data-drilldown-index`).
- Does NOT touch `this.queue`.

**Updated list-panel click delegation (`_bindControls`):**
- `⊕` button with `data-add-tab` / `data-add-item` → unchanged (browse-level add).
- `⊕` button with `data-drilldown-index` → `_addToQueue([drilldownTracks[i]])`.
- `.drill-back-btn` → re-render current tab via `_loadTab(currentBrowseTab)`.
- `.drill-add-all-btn` → `_addToQueue(drilldownTracks)`.
- `.list-item` with `data-browse-tab` → call new `_drillDown(tab, item)` instead
  of `_renderAudioList`.

**`_drillDown(tab, item)` method:**
```typescript
private _drillDown(tab: string, item: string): void {
  this._setListContent('<div class="list-empty">Loading…</div>');
  const q = buildBrowseQuery(tab, item);
  const label = `${tab}: ${item}`;
  this.api.search(q)
    .then(tracks => this._showDrilldown(tracks, label))
    .catch(err  => this._setListContent(
      `<div class="list-empty">${this._escHtml(String(err))}</div>`));
}
```

**`_runSearch` update:**
- Replace `_renderAudioList(results, …, false)` with
  `_showDrilldown(results, \`Search: \${q}\`)`.

**Remove `_renderAudioList`** (no longer used after this task).

---

## Phase 2 — Folders tab

Adds filesystem-level browsing to both backend and frontend.

### 2.1 Backend: FolderEntry type and GetFolders method

**Files:** `audiobox.go`

**Changes:**
- Add `FolderEntry` struct: `Path`, `Name` (deslugified last component),
  `TrackCount int`.
- Add `GetFolders() ([]FolderEntry, error)`: query distinct
  `filepath.Dir(content_url)` from `audio_files`, count tracks per dir,
  deslugify the last component, sort by `Path`.
- Add `GetFolderTracks(dir string) ([]AudioInfo, error)`: LIKE prefix query
  `content_url LIKE escapeLIKE(dir) + sep + "%"`, ordered by `disc_number,
  track_number, name`.

### 2.2 Backend: folder API endpoints

**Files:** `server.go`

**Changes:**
- Register `GET /api/list/folders` → `handleListFolders`.
- Register `GET /api/list/folder-tracks` → `handleListFolderTracks` (reads
  `?dir=` query param).
- Update `apiHelpMarkdown` with the two new endpoints.

### 2.3 Frontend: FolderEntry type and API methods

**Files:** `audiobox_api.ts`

**Changes:**
- Add `FolderEntry` interface: `{ path: string; name: string; trackCount: number }`.
- Add `listFolders(): Promise<FolderEntry[]>` to `AudioInfoAPI`.
- Add `listFolderTracks(dir: string): Promise<AudioInfo[]>` to `AudioInfoAPI`.

### 2.4 Frontend: Folders tab

**Files:** `audiobox_player.ts`

**Changes:**
- Add `<button class="tab" data-tab="folders">Folders</button>` to tabs in
  `PLAYER_TEMPLATE`.
- In `_loadTab("folders")`: call `api.listFolders()`, render folder list.
  Each row shows `name` + `(N tracks)` sub-line and a `⊕` button (uses
  `data-add-tab="folders" data-add-item="encodedPath"`).
- Browse click on a folder row (`data-browse-tab="folders"`) calls
  `_drillDownFolder(path)` which calls `api.listFolderTracks(path)` then
  `_showDrilldown(tracks, label)`.
- Update `buildBrowseQuery` to handle `"folders"` tab (or handle it separately
  since folder lookup uses path, not a search query).

---

## Phase 3 — Search improvements

Typed, grouped search results with per-result queue controls.

### 3.1 Grouped search UI

**Files:** `audiobox_player.ts`

**Changes:**
- Add `_runGroupedSearch(q)` method: issues four parallel API calls
  (`album:"q"`, `artist:"q"`, `title:"q"`, plain `q`) via `Promise.all`.
- Deduplicates results across groups by `ID`.
- Renders grouped sections in the list panel (Albums, Artists, Titles,
  Tracks) each with a `⊕ Add All` header button and per-item `⊕` / `⊖`
  buttons.
- `⊖` removes tracks matching that item from `this.queue`.
- Replace `_runSearch` with `_runGroupedSearch` in the search-button binding.
- Folder name matching done client-side against the last-loaded folder list
  (if available); folder section appears when folder names match `q`.

---

## Phase 4 — Network sharing

### 4.1 Config: shareAddress field

**Files:** `config.go`

**Changes:**
- Add `ShareAddress string \`yaml:"shareAddress,omitempty"\`` to
  `CollectionConfig`.

### 4.2 Backend: listener manager

**Files:** `server.go`

**Changes:**
- Add `shareState` struct: `mu sync.RWMutex`, `sharing bool`,
  `shareAddress string`.
- Add `listenerManager` struct and `run()` goroutine (see DESIGN.md).
- Refactor `Serve()` to use `listenerManager`: start with
  `127.0.0.1:port` (or `0.0.0.0:port` when `shareMode` is true at startup).
- Add `remoteAccessMiddleware`: check `r.RemoteAddr`; non-loopback +
  non-GET/HEAD → 403.
- Handle global `shutdownCh` to stop the listener manager cleanly.

### 4.3 Backend: share API endpoints

**Files:** `server.go`

**Changes:**
- `GET /api/share/status` → `handleShareStatus`.
- `GET /api/share/addresses` → `handleShareAddresses` (enumerate
  non-loopback IPv4 via `net.InterfaceAddrs()`).
- `POST /api/share/on` → `handleShareOn`: parse `{"address":"…"}`, respond
  immediately with `{"status":"restarting","poll_url":"…"}`, then async
  restart listener on `0.0.0.0:port`, save address to config.
- `POST /api/share/off` → `handleShareOff`: respond then async restart on
  `127.0.0.1:port`.
- Update `apiHelpMarkdown`.

### 4.4 CLI: `audiobox server share [IP]`

**Files:** `cmd/audiobox/main.go`

**Changes:**
- In the `"server"` case, check `args[0] == "share"` after loading the
  collection.
- If `args[1]` is present: validate it as an IPv4 address, save to config,
  start with share mode.
- If no `args[1]`: read `cfg.ShareAddress`; exit with error if empty.
- Update `handleServer` signature or add `handleServerShare(col, addr)`.
- Update `helpServer` text.

### 4.5 Frontend: share API types and methods

**Files:** `audiobox_api.ts`

**Changes:**
- Add `ShareStatus` interface: `{ sharing: bool; share_address: string;
  share_url: string }`.
- Add `shareStatus()`, `shareAddresses()`, `shareOn(address)`,
  `shareOff()` to `AudioInfoAPI`.

### 4.6 Frontend: share UI in library panel

**Files:** `audiobox_player.ts`

**Changes:**
- Add share row to Library panel in `PLAYER_TEMPLATE`:
  ```html
  <div class="library-row">
    <button class="lib-btn share-btn">Share</button>
    <span class="lib-status share-status"></span>
  </div>
  ```
- `_initShareStatus()`: calls `api.shareStatus()` on startup; if sharing,
  shows current share URL.
- `_bindControls`: wire `.share-btn` click →
  1. Call `api.shareAddresses()`.
  2. Show inline picker (or simple prompt if only one address).
  3. Call `api.shareOn(address)`.
  4. Poll `api.shareStatus()` until `sharing === true`.
  5. Update share-status span with URL + Copy button + Disable button.
- Disable button → `api.shareOff()` → poll until `sharing === false` →
  reset share-status span.
- Copy button → `navigator.clipboard.writeText(shareUrl)`.

---

## Phase 5 — Playlists

### 5.1 Backend: schema migration

**Files:** `config.go` (inside `initSchema`)

**Changes:**
- Add `CREATE TABLE IF NOT EXISTS playlists (…)`.
- Add `CREATE TABLE IF NOT EXISTS playlist_tracks (…)`.
- (Schema defined in DESIGN.md.)

### 5.2 Backend: playlist CRUD methods

**Files:** `audiobox.go`

**Changes:**
- Add `PlaylistInfo` struct: `ID`, `Name`, `TrackCount`, `Created`.
- Add `SavePlaylist(name string, trackIDs []string) (string, error)`.
- Add `GetPlaylists() ([]PlaylistInfo, error)`.
- Add `LoadPlaylist(id string) ([]AudioInfo, error)`.
- Add `DeletePlaylist(id string) error`.

### 5.3 Backend: playlist API endpoints

**Files:** `server.go`

**Changes:**
- `GET    /api/playlists`      → `handleListPlaylists`
- `POST   /api/playlists`      → `handleSavePlaylist` (blocked read-only)
- `GET    /api/playlists/{id}` → `handleLoadPlaylist`
- `DELETE /api/playlists/{id}` → `handleDeletePlaylist` (blocked read-only)
- Update `apiHelpMarkdown`.

### 5.4 Frontend: playlist API types and methods

**Files:** `audiobox_api.ts`

**Changes:**
- Add `PlaylistInfo` interface: `{ id, name, trackCount, created }`.
- Add `listPlaylists()`, `savePlaylist(name, trackIds)`,
  `loadPlaylist(id)`, `deletePlaylist(id)` to `AudioInfoAPI`.

### 5.5 Frontend: save queue as playlist

**Files:** `audiobox_player.ts`

**Changes:**
- Add `<button class="save-playlist-btn" disabled>Save as Playlist</button>`
  to `.queue-actions` in `PLAYER_TEMPLATE`.
- Enable when `queue.length > 0`.
- On click: prompt for playlist name (simple `window.prompt` or inline input
  in queue header); call `api.savePlaylist(name, queue.map(t => t.ID))`.
- Show brief confirmation in queue title area.

### 5.6 Frontend: playlists browse section

**Files:** `audiobox_player.ts`

**Changes:**
- Add a `<button class="tab" data-tab="playlists">Playlists</button>` tab
  (or a section in the Library panel — TBD based on space).
- `_loadTab("playlists")`: calls `api.listPlaylists()`, renders list. Each
  row shows playlist name + track count, with a `▶ Load` button and a
  `✕ Delete` button.
- `▶ Load` → `api.loadPlaylist(id)` → `_addToQueue(tracks)`.
- `✕ Delete` → confirm → `api.deletePlaylist(id)` → refresh list.

---

## Rebuild step (after any TypeScript change)

```bash
cd ~/Laboratory/audiobox
deno task bundle
```

The bundle command is: `deno bundle --platform browser audiobox_player.ts -o htdocs/module/audiobox.js`

## Build and test (after any Go change)

```bash
cd ~/Laboratory/audiobox
go test ./...
make build
```
