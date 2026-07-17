import type { Agent, AlbumEntry, AudioInfo, CollectionStatus, FolderEntry, PlaylistInfo, ShareStatus } from "./audiobox_api.ts";
import { AudioInfoAPI } from "./audiobox_api.ts";

// ---------------------------------------------------------------------------
// Pure utility functions — exported for testing without a DOM environment
// ---------------------------------------------------------------------------

/** formatArtists joins artist names from an Agent array into a comma-separated string.
 *
 * Parameters:
 *   artists (Agent[]) — list of artist agents
 *
 * Returns:
 *   string — comma-joined display names, or "" if the array is empty
 *
 * Example:
 *   formatArtists([{ type: "Person", name: "Glenn Gould" }]) // "Glenn Gould"
 */
export function formatArtists(artists: Agent[]): string {
  if (!artists || artists.length === 0) return "";
  return artists.map((a) => a.name).join(", ");
}

/** formatDuration converts an ISO 8601 duration string to a mm:ss or h:mm:ss display string.
 *
 * Parameters:
 *   iso (string) — ISO 8601 duration, e.g. "PT3M45S" or "PT1H2M3S"
 *
 * Returns:
 *   string — formatted duration, e.g. "3:45" or "1:02:03"; "" if unparseable
 *
 * Example:
 *   formatDuration("PT3M45S") // "3:45"
 */
export function formatDuration(iso: string): string {
  if (!iso) return "";
  const m = iso.match(/PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?/);
  if (!m || (!m[1] && !m[2] && !m[3])) return "";
  const h = parseInt(m[1] ?? "0", 10);
  const min = parseInt(m[2] ?? "0", 10);
  const sec = parseInt(m[3] ?? "0", 10);
  const mm = String(min).padStart(h > 0 ? 2 : 1, "0");
  const ss = String(sec).padStart(2, "0");
  return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
}

/** parseDurationSecs converts an ISO 8601 duration string to a total seconds count.
 *
 * Parameters:
 *   iso (string) — ISO 8601 duration, e.g. "PT3M45S" or "PT1H2M3S"
 *
 * Returns:
 *   number — total seconds; 0 if the string is empty or unparseable
 *
 * Example:
 *   parseDurationSecs("PT3M45S") // 225
 */
export function parseDurationSecs(iso: string): number {
  if (!iso) return 0;
  const m = iso.match(/PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?/);
  if (!m) return 0;
  const h = parseInt(m[1] ?? "0", 10);
  const min = parseInt(m[2] ?? "0", 10);
  const sec = parseInt(m[3] ?? "0", 10);
  return h * 3600 + min * 60 + sec;
}

/** buildBrowseQuery constructs a field-scoped search query for a browse tab selection.
 *
 * Parameters:
 *   tab  (string) — "albums" | "artists" | "titles"
 *   item (string) — the selected item label
 *
 * Returns:
 *   string — a query string suitable for AudioInfoAPI.search()
 *
 * Example:
 *   buildBrowseQuery("albums", "Goldberg Variations") // 'album:"Goldberg Variations"'
 */
export function buildBrowseQuery(tab: string, item: string): string {
  const escaped = item.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
  switch (tab) {
    case "albums":  return `album:"${escaped}"`;
    case "artists": return `artist:"${escaped}"`;
    case "titles":  return `title:"${escaped}"`;
    default:        return item;
  }
}

/** isFieldScopedQuery reports whether every term of a search query is
 * scoped to a known field (album:, artist:, title:, genre:, recording:,
 * recording_of:, name:), matching the server's parseQuery field-alias
 * table — an unrecognised "alias:" prefix is plain text there too, so
 * this must agree or a client-side "is this exact?" decision would
 * disagree with the search actually performed.
 *
 * Parameters:
 *   q (string) — raw search box text
 *
 * Returns:
 *   boolean — true when q starts with a recognised field: prefix
 *
 * Example:
 *   isFieldScopedQuery("artist:Shimabukuro") // true
 *   isFieldScopedQuery("Shimabukuro")        // false
 */
export function isFieldScopedQuery(q: string): boolean {
  return /^\s*(title|name|album|artist|genre|recording_of|recording)\s*:/i.test(q);
}

/** dirOfContentURL returns the directory portion of a track's ContentURL,
 * matching how the server derives an Album's `dir` (filepath.Dir of the
 * stored content_url) — used to map free-text search results back to the
 * exact album directories they belong to.
 *
 * Parameters:
 *   url (string) — a track's ContentURL, relative or absolute, either separator style
 *
 * Returns:
 *   string — the directory portion, "" when url has no directory component
 *
 * Example:
 *   dirOfContentURL("Travels/01-departure.wav") // "Travels"
 */
export function dirOfContentURL(url: string): string {
  const parts = url.replace(/\\/g, "/").split("/");
  parts.pop();
  return parts.join("/");
}

/** FolderTreeNode is one navigable row of a fully-expanded, indented folder
 * tree built from the flat FolderEntry list returned by GET /api/list/folders.
 */
export interface FolderTreeNode {
  path: string;
  depth: number;
  name: string;
  /** Tracks stored directly in this exact directory (not its subfolders). */
  ownCount: number;
  /** Tracks in this directory plus every subfolder beneath it. */
  totalCount: number;
  hasChildren: boolean;
}

/** buildFolderTree expands the flat FolderEntry list (which only contains
 * directories that directly hold audio files) into a node for every
 * directory AND every ancestor of every directory, at unlimited depth, so a
 * folder nested arbitrarily deep (e.g. "Music/Albums/Artist/Album/Disc-1")
 * gets its own row instead of being silently folded into a shallower
 * ancestor's track count with no way to select it on its own.
 *
 * Parameters:
 *   folders (FolderEntry[]) — flat list as returned by GET /api/list/folders
 *
 * Returns:
 *   FolderTreeNode[] — one node per distinct directory path (leaf or
 *   ancestor), unsorted; sort by `path` for a parent-before-children display
 *   order (safe because "/" sorts before any letter or digit)
 *
 * Example:
 *   const tree = buildFolderTree(await api.listFolders());
 *   tree.sort((a, b) => a.path.localeCompare(b.path));
 */
export function buildFolderTree(folders: FolderEntry[]): FolderTreeNode[] {
  const nodes = new Map<string, FolderTreeNode>();
  const ensure = (path: string, depth: number): FolderTreeNode => {
    let n = nodes.get(path);
    if (!n) {
      n = {
        path,
        depth,
        name: path.split("/").pop() ?? path,
        ownCount: 0,
        totalCount: 0,
        hasChildren: false,
      };
      nodes.set(path, n);
    }
    return n;
  };

  for (const f of folders) {
    const parts = f.path.replace(/\\/g, "/").split("/").filter(Boolean);
    if (parts.length === 0) continue;
    ensure(parts.join("/"), parts.length - 1).ownCount += f.trackCount;
    let acc = "";
    for (let i = 0; i < parts.length; i++) {
      acc = i === 0 ? parts[0] : `${acc}/${parts[i]}`;
      ensure(acc, i).totalCount += f.trackCount;
      if (i > 0) {
        ensure(parts.slice(0, i).join("/"), i - 1).hasChildren = true;
      }
    }
  }

  return [...nodes.values()];
}

// ---------------------------------------------------------------------------
// Shadow DOM template
// ---------------------------------------------------------------------------

const STYLES = `
:host {
  display: block;
  font-family: system-ui, sans-serif;
  font-size: 14px;
  color: #1a1a1a;
  background: #f8f8f8;
  border: 1px solid #ccc;
  border-radius: 6px;
  overflow: hidden;
  max-width: 680px;
}

/* ---- header bar ---- */
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: #222;
  color: #eee;
}
.header-title { font-weight: 600; font-size: 15px; }
.shutdown-btn {
  padding: 4px 10px;
  border: 1px solid #555;
  border-radius: 4px;
  background: transparent;
  color: #eee;
  cursor: pointer;
  font-size: 13px;
}
.shutdown-btn:hover { background: #c0392b; border-color: #c0392b; }

/* ---- overlay screens (offline / init) ---- */
.screen {
  padding: 32px 24px;
  text-align: center;
  background: #fff;
}
.screen h2 { margin: 0 0 12px; font-size: 18px; }
.screen p  { margin: 0 0 16px; color: #555; font-size: 13px; }
.screen-btn {
  padding: 8px 20px;
  border: none;
  border-radius: 4px;
  background: #333;
  color: #fff;
  cursor: pointer;
  font-size: 14px;
}
.screen-btn:hover { background: #555; }
.screen-btn + .screen-btn { margin-left: 8px; }

/* ---- browse panel ---- */
.browse-panel { padding: 10px; border-bottom: 1px solid #ddd; }
.tabs { display: flex; gap: 4px; margin-bottom: 8px; }
.tab {
  padding: 5px 12px; border: 1px solid #ccc; border-radius: 4px;
  background: #fff; cursor: pointer; font-size: 13px;
}
.tab.active { background: #333; color: #fff; border-color: #333; }
.search-bar { display: flex; gap: 6px; }
.search-bar input {
  flex: 1; padding: 5px 8px; border: 1px solid #ccc;
  border-radius: 4px; font-size: 13px;
}
.search-bar button {
  padding: 5px 10px; border: 1px solid #ccc; border-radius: 4px;
  background: #fff; cursor: pointer; font-size: 13px;
}
.list-panel {
  margin-top: 8px; max-height: 200px; overflow-y: auto;
  border: 1px solid #e8e8e8; border-radius: 4px; background: #fff;
}
.list-item {
  padding: 6px 10px; cursor: pointer; border-bottom: 1px solid #f0f0f0;
  display: flex; align-items: center; gap: 6px;
}
.list-item:hover { background: #f0f0f0; }
.list-item:last-child { border-bottom: none; }
.list-item-main { flex: 1; min-width: 0; }
.list-item-title {
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-size: 13px;
}
.list-item-sub {
  font-size: 11px; color: #888; white-space: nowrap;
  overflow: hidden; text-overflow: ellipsis; margin-top: 1px;
}
.row-add-btn {
  flex-shrink: 0; padding: 2px 6px; border: 1px solid #ccc;
  border-radius: 3px; background: #fff; color: #666;
  cursor: pointer; font-size: 14px; line-height: 1.2;
}
.row-add-btn:hover { background: #e8f0fe; border-color: #4a90d9; color: #1a73e8; }
.row-remove-btn {
  flex-shrink: 0; padding: 2px 6px; border: 1px solid #ccc;
  border-radius: 3px; background: #fff; color: #aaa;
  cursor: pointer; font-size: 14px; line-height: 1.2;
}
.row-remove-btn:hover { background: #fde8e8; border-color: #e74c3c; color: #e74c3c; }
.list-empty { padding: 10px; color: #888; text-align: center; font-style: italic; }

/* ---- folder tree ---- */
.folder-toggle-btn {
  flex-shrink: 0; padding: 2px 8px; border: 1px solid #ccc;
  border-radius: 3px; background: #f5f5f5; color: #999; cursor: pointer; font-size: 11px;
}
.folder-toggle-btn.on { background: #e8f5e9; color: #2e7d32; border-color: #a5d6a7; }
.folder-toggle-btn:hover { opacity: 0.8; }
.folder-child > .list-item-main .list-item-title,
.folder-child > .list-item-main .list-item-sub { padding-left: 18px; }
.folder-child { background: #fafafa; }

/* ---- grouped search results ---- */
.search-group { border-bottom: 1px solid #e8e8e8; }
.search-group:last-child { border-bottom: none; }
.search-group-header {
  display: flex; align-items: center; gap: 6px;
  padding: 5px 10px; background: #f0f0f0; border-bottom: 1px solid #e8e8e8;
}
.search-group-label {
  flex: 1; font-size: 11px; font-weight: 600; text-transform: uppercase; color: #666;
}

/* ---- drill-down bar ---- */
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

/* ---- now-playing panel ---- */
.now-playing {
  padding: 10px; background: #fff; border-bottom: 1px solid #ddd;
}
.track-title { font-weight: 600; font-size: 15px; }
.track-sub { color: #555; font-size: 12px; margin-top: 2px; }
.context-info { font-size: 11px; color: #888; margin: 4px 0 6px; }
.controls { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.ctrl-btn {
  padding: 4px 10px; border: 1px solid #ccc; border-radius: 4px;
  background: #fff; cursor: pointer; font-size: 16px; line-height: 1;
}
.ctrl-btn:disabled { opacity: 0.4; cursor: default; }
.progress {
  display: flex; align-items: center; gap: 6px;
  margin-bottom: 6px; font-size: 12px; color: #555;
}
.seek-bar { flex: 1; cursor: pointer; }
.volume-row { display: flex; align-items: center; gap: 6px; font-size: 12px; color: #555; }
.volume-bar { width: 80px; cursor: pointer; }

/* ---- queue panel ---- */
.queue-panel { padding: 6px 10px 10px; border-bottom: 1px solid #ddd; }
.queue-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px; }
.queue-title { font-weight: 600; font-size: 12px; text-transform: uppercase; color: #888; }
.queue-actions { display: flex; gap: 4px; align-items: center; }
.save-playlist-btn {
  font-size: 11px; padding: 2px 6px; border: 1px solid #4a90d9;
  border-radius: 3px; background: #fff; color: #1a73e8; cursor: pointer;
}
.save-playlist-btn:disabled { opacity: 0.4; cursor: default; }
.save-playlist-btn:not(:disabled):hover { background: #e8f0fe; }
.clear-queue-btn {
  font-size: 11px; padding: 2px 6px; border: 1px solid #ccc;
  border-radius: 3px; background: #fff; cursor: pointer;
}
.clear-queue-btn:disabled { opacity: 0.4; cursor: default; }
.clear-queue-btn:not(:disabled):hover { background: #f0f0f0; }
.shuffle-btn {
  font-size: 11px; padding: 2px 6px; border: 1px solid #ccc;
  border-radius: 3px; background: #fff; cursor: pointer;
}
.shuffle-btn:disabled { opacity: 0.4; cursor: default; }
.shuffle-btn:not(:disabled):hover { background: #f0f0f0; }
.toggle-queue-btn {
  font-size: 11px; padding: 2px 6px; border: 1px solid #ccc;
  border-radius: 3px; background: #fff; cursor: pointer;
}
.queue-list {
  max-height: 140px; overflow-y: auto;
  border: 1px solid #e8e8e8; border-radius: 4px; background: #fff;
}
.queue-item {
  padding: 5px 10px; cursor: pointer; border-bottom: 1px solid #f0f0f0;
  font-size: 12px; display: flex; align-items: center; gap: 4px;
}
.queue-item:hover { background: #f0f0f0; }
.queue-item.current { background: #e8f0fe; font-weight: 600; }
.queue-item:last-child { border-bottom: none; }
.queue-item-name { flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.queue-remove-btn {
  flex-shrink: 0; padding: 0 4px; border: none;
  background: transparent; color: #aaa; cursor: pointer; font-size: 14px;
}
.queue-remove-btn:hover { color: #e74c3c; }

/* ---- library panel ---- */
.library-panel { padding: 8px 10px 10px; }
.library-header {
  display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px;
}
.library-title { font-weight: 600; font-size: 12px; text-transform: uppercase; color: #888; }
.toggle-library-btn {
  font-size: 11px; padding: 2px 6px; border: 1px solid #ccc;
  border-radius: 3px; background: #fff; cursor: pointer;
}
.library-body { display: flex; flex-direction: column; gap: 6px; }
.library-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.lib-btn {
  padding: 4px 12px; border: 1px solid #ccc; border-radius: 4px;
  background: #fff; cursor: pointer; font-size: 12px;
}
.lib-btn:hover { background: #f0f0f0; }
.lib-btn:disabled { opacity: 0.5; cursor: default; }
.lib-status {
  font-size: 11px; color: #555;
}
.lib-status.error { color: #c0392b; }
.lib-status.ok    { color: #27ae60; }
.share-url-text { font-family: monospace; font-size: 11px; color: #27ae60; }
.share-addr-select {
  font-size: 11px; padding: 2px 4px; border: 1px solid #ccc;
  border-radius: 3px; background: #fff;
}
`;

/** PLAYER_TEMPLATE is the inner HTML injected into the component's shadow root.
 * Exported so tests can parse and assert on the DOM structure without a live component.
 */
export const PLAYER_TEMPLATE = `
<style>${STYLES}</style>

<div class="header">
  <span class="header-title">audiobox</span>
  <button class="shutdown-btn" title="Shut down the audiobox server">⏻ Power Off</button>
</div>

<div class="browse-panel">
  <div class="tabs">
    <button class="tab active" data-tab="albums">Albums</button>
    <button class="tab" data-tab="artists">Artists</button>
    <button class="tab" data-tab="titles">Titles</button>
    <button class="tab" data-tab="folders">Folders</button>
    <button class="tab" data-tab="playlists">Playlists</button>
  </div>
  <div class="search-bar">
    <input type="search" placeholder="Search, or album:Name / artist:Name / title:Name / /regex/"
           title="Plain text searches everywhere. Narrow with album:, artist:, title:, or genre: (quote multi-word values), or use /regex/. On the Albums tab, plain text filters by album or artist name." />
    <button class="search-btn">Search</button>
  </div>
  <div class="list-panel">
    <div class="list-empty">Select a tab to browse</div>
  </div>
</div>

<div class="now-playing">
  <div class="track-title">(No track loaded)</div>
  <div class="track-sub"></div>
  <div class="context-info"></div>
  <div class="controls">
    <button class="ctrl-btn prev-btn" title="Previous">⏮</button>
    <button class="ctrl-btn play-pause-btn" title="Play">▶</button>
    <button class="ctrl-btn next-btn" title="Next">⏭</button>
  </div>
  <div class="progress">
    <span class="current-time">0:00</span>
    <input type="range" class="seek-bar" min="0" max="100" value="0" step="0.1" />
    <span class="total-time">0:00</span>
  </div>
  <div class="volume-row">
    <span>Vol</span>
    <input type="range" class="volume-bar" min="0" max="1" step="0.05" value="1" />
  </div>
  <audio class="audio-el"></audio>
</div>

<div class="queue-panel">
  <div class="queue-header">
    <span class="queue-title">Queue</span>
    <div class="queue-actions">
      <button class="save-playlist-btn" disabled>Save as Playlist</button>
      <button class="clear-queue-btn" disabled>Clear</button>
      <button class="shuffle-btn" title="Shuffle remaining tracks" disabled>⇄ Shuffle</button>
      <button class="toggle-queue-btn">Hide</button>
    </div>
  </div>
  <div class="queue-list">
    <div class="list-empty">No tracks queued</div>
  </div>
</div>

<div class="library-panel">
  <div class="library-header">
    <span class="library-title">Library</span>
    <button class="toggle-library-btn">Hide</button>
  </div>
  <div class="library-body">
    <div class="library-row">
      <button class="lib-btn scan-btn">Scan</button>
      <span class="lib-status scan-status"></span>
    </div>
    <div class="library-row">
      <button class="lib-btn sweep-btn">Sweep</button>
      <span class="lib-status sweep-status"></span>
    </div>
    <div class="library-row">
      <button class="lib-btn share-btn">Share</button>
      <span class="lib-status share-status"></span>
    </div>
  </div>
</div>
`;

// ---------------------------------------------------------------------------
// Web Component
// ---------------------------------------------------------------------------

// Deno (no DOM) guard: fall back to a plain class so the module can be
// imported in tests without crashing on `extends HTMLElement`.
function getBaseClass(): typeof HTMLElement {
  if (typeof HTMLElement !== "undefined") return HTMLElement;
  return class {} as unknown as typeof HTMLElement;
}
const _Base = getBaseClass();

/** AudioInfoPlayer is a self-contained web component for browsing and playing an audiobox collection.
 *
 * Attributes:
 *   api-url (string) — base URL of the audiobox server; defaults to "" (same-origin)
 *
 * Usage:
 *   &lt;audiobox-player api-url="http://localhost:8010"&gt;&lt;/audiobox-player&gt;
 *
 * On connect the component checks /api/status. If the service is offline it shows
 * a retry screen. If the collection is not yet initialized it shows a setup screen.
 * Once connected, the component renders a browse panel (Albums / Artists / Titles tabs
 * and a search box), a now-playing panel, a collapsible queue panel, and a library
 * management panel with Scan and Sweep controls.
 */
export class AudioInfoPlayer extends _Base {
  static get observedAttributes(): string[] {
    return ["api-url"];
  }

  private api!: AudioInfoAPI;
  private audioEl!: HTMLAudioElement;
  private queue: AudioInfo[] = [];
  private currentIndex = -1;
  private drilldownTracks: AudioInfo[] = [];
  private drilldownLabel = "";
  private currentBrowseTab = "albums";
  private folderCache: FolderEntry[] | null = null;
  private folderEnabled = new Map<string, boolean>();
  private searchResultsMap = new Map<string, AudioInfo>();
  private searchGroups = new Map<string, AudioInfo[]>();
  private queueVisible = true;
  private libraryVisible = true;
  private seekDragging = false;
  private scanPollTimer = 0;
  private sweepPollTimer = 0;

  constructor() {
    super();
    const shadow = this.attachShadow({ mode: "open" });
    shadow.innerHTML = PLAYER_TEMPLATE;
    this.api = new AudioInfoAPI(this.getAttribute("api-url") ?? "");
    this.audioEl = shadow.querySelector<HTMLAudioElement>(".audio-el")!;
    this._bindControls();
    this._bindAudio();
  }

  attributeChangedCallback(name: string, _old: string | null, val: string | null): void {
    if (name === "api-url") {
      this.api = new AudioInfoAPI(val ?? "");
    }
  }

  connectedCallback(): void {
    this._checkStatus();
  }

  private get shadow(): ShadowRoot {
    return this.shadowRoot!;
  }

  private qs<T extends Element>(sel: string): T {
    return this.shadow.querySelector<T>(sel)!;
  }

  // ---- startup: status check + init screen --------------------------------

  private async _checkStatus(): Promise<void> {
    try {
      const s: CollectionStatus = await this.api.status();
      if (!s.initialized) {
        this._showInitScreen();
      } else {
        this._initPlayer();
        await this._loadFolderEnabledFromServer();
        this._initShareStatus();
        this._loadTab("albums");
      }
    } catch (_e) {
      this._showOfflineScreen();
    }
  }

  private _showOfflineScreen(): void {
    this._overlayHTML(`
      <div class="screen">
        <h2>Service Offline</h2>
        <p>The audiobox server is not reachable.<br>
           Run <code>audiobox</code> in a terminal to start it.</p>
        <button class="screen-btn retry-btn">Retry</button>
      </div>
    `);
    this.shadow.querySelector(".retry-btn")?.addEventListener("click", () => {
      this._clearOverlay();
      this._checkStatus();
    });
  }

  private _showInitScreen(): void {
    this._overlayHTML(`
      <div class="screen">
        <h2>Welcome to audiobox</h2>
        <p>Your collection has not been set up yet.<br>
           Click Initialize to create the standard ~/Audio directory layout.</p>
        <button class="screen-btn init-btn">Initialize Collection</button>
      </div>
    `);
    this.shadow.querySelector(".init-btn")?.addEventListener("click", async () => {
      const btn = this.shadow.querySelector<HTMLButtonElement>(".init-btn")!;
      btn.disabled = true;
      btn.textContent = "Initializing…";
      try {
        await this.api.init();
        this._clearOverlay();
        this._initPlayer();
        this._loadTab("albums");
      } catch (e) {
        btn.disabled = false;
        btn.textContent = "Initialize Collection";
        const p = this.shadow.querySelector(".screen p")!;
        p.textContent = `Error: ${String(e)}`;
      }
    });
  }

  private _overlayHTML(html: string): void {
    // Insert overlay after the header, replacing the browse panel temporarily.
    const browsePanel = this.qs<HTMLElement>(".browse-panel");
    let overlay = this.shadow.querySelector<HTMLElement>(".overlay");
    if (!overlay) {
      overlay = document.createElement("div");
      overlay.className = "overlay";
      browsePanel.parentNode!.insertBefore(overlay, browsePanel);
    }
    overlay.innerHTML = html;
    browsePanel.style.display = "none";
  }

  private _clearOverlay(): void {
    this.shadow.querySelector(".overlay")?.remove();
    const browsePanel = this.qs<HTMLElement>(".browse-panel");
    browsePanel.style.display = "";
  }

  // ---- player init --------------------------------------------------------

  /** _initPlayer resets the now-playing panel to an idle state. */
  private _initPlayer(): void {
    this.qs(".track-title").textContent = "(No track loaded)";
    this.qs(".track-sub").textContent = "";
    this.qs(".context-info").textContent = "";
    (this.qs<HTMLInputElement>(".seek-bar")).value = "0";
    this.qs(".current-time").textContent = "0:00";
    this.qs(".total-time").textContent = "0:00";
    this._refreshPlayState(false);
  }

  // ---- data loading -------------------------------------------------------

  private async _loadTab(tab: string): Promise<void> {
    this.currentBrowseTab = tab;
    this.shadow.querySelectorAll<HTMLButtonElement>(".tab").forEach((b) => {
      b.classList.toggle("active", b.dataset.tab === tab);
    });
    this._setListContent('<div class="list-empty">Loading…</div>');
    try {
      const excl = this._getExcludedFolderPaths();
      if (tab === "albums") {
        const albums = await this.api.listAlbums(excl);
        this._renderAlbumList(albums);
      } else if (tab === "folders") {
        const folders = await this.api.listFolders();
        this.folderCache = folders;
        this._renderFolderList(folders);
      } else if (tab === "playlists") {
        const lists = await this.api.listPlaylists();
        this._renderPlaylistList(lists);
      } else {
        const items = tab === "artists"
          ? await this.api.listArtists(excl)
          : await this.api.listTitles(excl);
        this._renderStringList(items, tab);
      }
    } catch (e) {
      this._setListContent(`<div class="list-empty">${this._escHtml(String(e))}</div>`);
    }
  }

  /** _runSearch dispatches a search-box query to the right handler for the
   * currently active browse tab. On the Albums tab it filters the album list
   * itself (_searchAlbumsTab) so a query like an artist's name — with or
   * without an explicit "artist:" prefix — narrows down to that artist's
   * albums while staying in the Albums browsing view (click/add-to-queue
   * still resolve by exact directory, not by tag). Every other tab keeps the
   * existing cross-tab grouped-results search.
   */
  private _runSearch(q: string): void {
    if (this.currentBrowseTab === "albums") {
      this._searchAlbumsTab(q);
    } else {
      this._runGroupedSearch(q);
    }
  }

  /** _searchAlbumsTab filters the Albums browse list down to albums that
   * match the query by album name or artist name — a bare query (no field
   * prefix) is tried against both album: and artist: so that typing e.g.
   * "Shimabukuro" while browsing Albums finds his albums without requiring
   * the user to know the artist: prefix syntax. An explicit field:value
   * query (album:, artist:, title:, ...) is passed through as-is.
   *
   * Matches are resolved to albums by comparing each result track's
   * directory (dirOfContentURL) against each album's exact `dir` — the same
   * directory-based identity used by listAlbumTracks — so a tag/directory
   * mismatch never causes an album to wrongly appear or disappear here.
   */
  private async _searchAlbumsTab(q: string): Promise<void> {
    const trimmed = q.trim();
    if (!trimmed) return;
    this._setListContent('<div class="list-empty">Searching…</div>');
    try {
      let tracks: AudioInfo[];
      if (isFieldScopedQuery(trimmed)) {
        tracks = await this.api.search(trimmed);
      } else {
        const esc = (s: string) => s.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
        const [byAlbum, byArtist] = await Promise.all([
          this.api.search(`album:"${esc(trimmed)}"`),
          this.api.search(`artist:"${esc(trimmed)}"`),
        ]);
        tracks = [...byAlbum, ...byArtist];
      }
      if (tracks.length === 0) {
        this._setListContent('<div class="list-empty">No matching albums</div>');
        return;
      }
      const filtered = await this._albumsMatchingTracks(tracks);
      if (filtered.length === 0) {
        this._setListContent('<div class="list-empty">No matching albums</div>');
        return;
      }
      this._renderAlbumList(filtered);
    } catch (e) {
      this._setListContent(`<div class="list-empty">${this._escHtml(String(e))}</div>`);
    }
  }

  /** _albumsMatchingTracks resolves a set of tracks (e.g. from a tag-based
   * search) back to the exact AlbumEntry objects they belong to, by matching
   * each track's directory against each album's `dir`. Reused by both
   * _searchAlbumsTab and _drillDownArtistAlbums so "queue this album" always
   * goes through the same exact directory-based resolution as the Albums tab.
   */
  private async _albumsMatchingTracks(tracks: AudioInfo[]): Promise<AlbumEntry[]> {
    const matchedDirs = new Set(tracks.map((t) => dirOfContentURL(t.ContentURL ?? "")));
    const albums = await this.api.listAlbums(this._getExcludedFolderPaths());
    return albums.filter((a) => matchedDirs.has(a.dir));
  }

  /** _drillDownArtistAlbums shows the albums an artist appears on (each with
   * its own "add to queue" button) rather than a flat list of their
   * individual tracks, so a whole album can be queued in one click from the
   * Artists tab. Falls back to a flat track drilldown for tracks that can't
   * be resolved to an album directory (e.g. loose singles at the AudioDir
   * root).
   */
  private async _drillDownArtistAlbums(artistName: string): Promise<void> {
    this._setListContent('<div class="list-empty">Loading…</div>');
    try {
      const esc = (s: string) => s.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
      const tracks = await this.api.search(`artist:"${esc(artistName)}"`);
      if (tracks.length === 0) {
        this._setListContent('<div class="list-empty">No albums found</div>');
        return;
      }
      const albums = await this._albumsMatchingTracks(tracks);
      if (albums.length === 0) {
        this._showDrilldown(tracks, `Artist: ${artistName}`);
        return;
      }
      this._renderAlbumList(albums);
    } catch (e) {
      this._setListContent(`<div class="list-empty">${this._escHtml(String(e))}</div>`);
    }
  }

  private async _runGroupedSearch(q: string): Promise<void> {
    if (!q.trim()) return;
    const trimmed = q.trim();
    this._setListContent('<div class="list-empty">Searching…</div>');
    this.searchResultsMap.clear();
    this.searchGroups.clear();
    try {
      const esc = (s: string) => s.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
      const [albumTracks, artistTracks, titleTracks, anyTracks] = await Promise.all([
        this.api.search(`album:"${esc(trimmed)}"`),
        this.api.search(`artist:"${esc(trimmed)}"`),
        this.api.search(`title:"${esc(trimmed)}"`),
        this.api.search(trimmed),
      ]);

      // Deduplicate: each track ID appears only in the first group that claims it.
      const seen = new Set<string>();
      const dedup = (tracks: AudioInfo[]): AudioInfo[] => {
        const out: AudioInfo[] = [];
        for (const t of tracks) {
          if (!seen.has(t.ID)) { seen.add(t.ID); out.push(t); }
        }
        return out;
      };
      const groups: Array<{ label: string; tracks: AudioInfo[] }> = [
        { label: "Albums",  tracks: dedup(albumTracks) },
        { label: "Artists", tracks: dedup(artistTracks) },
        { label: "Titles",  tracks: dedup(titleTracks) },
        { label: "Tracks",  tracks: dedup(anyTracks) },
      ].filter((g) => g.tracks.length > 0);

      for (const g of groups) {
        this.searchGroups.set(g.label, g.tracks);
        for (const t of g.tracks) this.searchResultsMap.set(t.ID, t);
      }

      const matchingFolders = this.folderCache
        ? this.folderCache.filter((f) =>
            f.name.toLowerCase().includes(trimmed.toLowerCase()) ||
            f.path.toLowerCase().includes(trimmed.toLowerCase()))
        : [];

      if (groups.length === 0 && matchingFolders.length === 0) {
        this._setListContent('<div class="list-empty">No results</div>');
        return;
      }
      const html = [
        ...groups.map((g) => this._renderSearchGroup(g.label, g.tracks)),
        ...(matchingFolders.length > 0 ? [this._renderFolderSearchGroup(matchingFolders)] : []),
      ].join("");
      this._setListContent(html);
    } catch (e) {
      this._setListContent(`<div class="list-empty">${this._escHtml(String(e))}</div>`);
    }
  }

  private _renderSearchGroup(label: string, tracks: AudioInfo[]): string {
    const header =
      `<div class="search-group-header">` +
      `<span class="search-group-label">${this._escHtml(label)} (${tracks.length})</span>` +
      `<button class="drill-add-all-btn" title="Add all to queue" data-group-label="${this._escAttr(label)}">⊕ Add All</button>` +
      `</div>`;
    const items = tracks.map((t) => {
      const sub = [formatArtists(t.ByArtist), t.InAlbum].filter(Boolean).join(" · ");
      return (
        `<div class="list-item">` +
        `<div class="list-item-main">` +
        `<div class="list-item-title">${this._escHtml(t.Name || "(untitled)")}</div>` +
        (sub ? `<div class="list-item-sub">${this._escHtml(sub)}</div>` : "") +
        `</div>` +
        `<button class="row-add-btn" title="Add to queue" data-search-track-id="${this._escAttr(t.ID)}">⊕</button>` +
        `<button class="row-remove-btn" title="Remove from queue" data-remove-track-id="${this._escAttr(t.ID)}">⊖</button>` +
        `</div>`
      );
    }).join("");
    return `<div class="search-group">${header}${items}</div>`;
  }

  private _renderFolderSearchGroup(folders: FolderEntry[]): string {
    const header =
      `<div class="search-group-header">` +
      `<span class="search-group-label">Folders (${folders.length})</span>` +
      `</div>`;
    const items = folders.map((f) =>
      `<div class="list-item">` +
      `<div class="list-item-main">` +
      `<div class="list-item-title">${this._escHtml(f.name)}</div>` +
      `<div class="list-item-sub">${this._escHtml(f.path)} · ${f.trackCount} track${f.trackCount !== 1 ? "s" : ""}</div>` +
      `</div>` +
      `<button class="row-add-btn" title="Add to queue" data-add-tab="folders" data-add-item="${this._escAttr(f.path)}">⊕</button>` +
      `</div>`
    ).join("");
    return `<div class="search-group">${header}${items}</div>`;
  }

  // ---- list rendering -----------------------------------------------------

  private _setListContent(html: string): void {
    this.qs(".list-panel").innerHTML = html;
  }

  /** _renderAlbumList renders the albums browse list. data-browse-item is set to the
   * album's directory (Album.dir) rather than its name, so drilling down or adding to
   * queue resolves tracks by exact directory (listAlbumTracks) instead of a tag-based
   * search — this avoids mixing tracks from a similarly-named sibling album (e.g.
   * "Travel" vs "Travels with Jack") or missing tracks whose tags disagree with the
   * directory-derived name. The visible text shows displayName; data-browse-label
   * carries it through so the drilldown header reads correctly.
   */
  private _renderAlbumList(albums: AlbumEntry[]): void {
    if (albums.length === 0) {
      this._setListContent('<div class="list-empty">No items found</div>');
      return;
    }
    this._setListContent(
      albums
        .map(
          (a) =>
            `<div class="list-item" data-browse-tab="albums" data-browse-item="${this._escAttr(a.dir)}" data-browse-label="${this._escAttr(a.displayName)}">` +
            `<div class="list-item-main"><div class="list-item-title">${this._escHtml(a.displayName)}</div></div>` +
            `<button class="row-add-btn" title="Add to queue" data-add-tab="albums" data-add-item="${this._escAttr(a.dir)}">⊕</button>` +
            `</div>`,
        )
        .join(""),
    );
  }

  private _renderStringList(items: string[], tab: string): void {
    if (items.length === 0) {
      this._setListContent('<div class="list-empty">No items found</div>');
      return;
    }
    this._setListContent(
      items
        .map(
          (item) =>
            `<div class="list-item" data-browse-tab="${this._escAttr(tab)}" data-browse-item="${this._escAttr(item)}">` +
            `<div class="list-item-main"><div class="list-item-title">${this._escHtml(item)}</div></div>` +
            `<button class="row-add-btn" title="Add to queue" data-add-tab="${this._escAttr(tab)}" data-add-item="${this._escAttr(item)}">⊕</button>` +
            `</div>`,
        )
        .join(""),
    );
  }

  /** _renderFolderList renders the full folder tree as a single flat,
   * indented list — one row per directory at every depth (not just the top
   * two levels) — so a folder nested arbitrarily deep can still be selected,
   * toggled, or drilled into directly. See buildFolderTree.
   */
  private _renderFolderList(folders: FolderEntry[]): void {
    if (folders.length === 0) {
      this._setListContent('<div class="list-empty">No folders found</div>');
      return;
    }

    const tree = buildFolderTree(folders).sort((a, b) => a.path.localeCompare(b.path));

    let html = "";
    for (const node of tree) {
      const selfEnabled = this._isFolderEnabled(node.path);
      const effectiveEnabled = this._isFolderEffectivelyEnabled(node.path);
      const name = this._deslugify(node.name);
      const tCls = selfEnabled ? " on" : "";
      const addDis = effectiveEnabled ? "" : " disabled";
      const rowCls = node.depth > 0 ? " folder-child" : "";
      const indent = node.depth > 0 ? ` style="padding-left: ${node.depth * 1.25}em"` : "";
      html +=
        `<div class="list-item${rowCls}" data-browse-tab="folders" data-browse-item="${this._escAttr(node.path)}">` +
        `<div class="list-item-main"${indent}>` +
        `<div class="list-item-title">${this._escHtml(name)}</div>` +
        `<div class="list-item-sub">${node.totalCount} track${node.totalCount !== 1 ? "s" : ""}` +
        (node.hasChildren ? ` · sub-folders` : "") +
        `</div></div>` +
        `<button class="folder-toggle-btn${tCls}" data-toggle-folder="${this._escAttr(node.path)}">${selfEnabled ? "ON" : "OFF"}</button>` +
        `<button class="row-add-btn"${addDis} title="Add to queue" data-add-tab="folders" data-add-item="${this._escAttr(node.path)}">⊕</button>` +
        `</div>`;
    }

    this._setListContent(html);
  }

  private _renderPlaylistList(lists: PlaylistInfo[]): void {
    if (lists.length === 0) {
      this._setListContent('<div class="list-empty">No saved playlists</div>');
      return;
    }
    this._setListContent(
      lists.map((pl) =>
        `<div class="list-item">` +
        `<div class="list-item-main">` +
        `<div class="list-item-title">${this._escHtml(pl.name)}</div>` +
        `<div class="list-item-sub">${pl.trackCount} track${pl.trackCount !== 1 ? "s" : ""}</div>` +
        `</div>` +
        `<button class="row-add-btn" title="Load playlist into queue" data-playlist-load="${this._escAttr(pl.id)}">▶</button>` +
        `<button class="row-remove-btn" title="Delete playlist" data-playlist-delete="${this._escAttr(pl.id)}">✕</button>` +
        `</div>`
      ).join(""),
    );
  }

  /** _showDrilldown renders a track list inside the list panel with a back bar.
   * Stores the tracks in drilldownTracks for add-all and per-item add actions.
   * Does NOT modify the queue.
   *
   * Parameters:
   *   tracks (AudioInfo[]) — tracks to display
   *   label  (string)      — descriptive label shown in the back bar
   */
  private _showDrilldown(tracks: AudioInfo[], label: string): void {
    this.drilldownTracks = tracks;
    this.drilldownLabel = label;
    const backBar =
      `<div class="drill-back-bar">` +
      `<button class="drill-back-btn">← Back</button>` +
      `<span class="drill-context-label">${this._escHtml(label)}</span>` +
      (tracks.length > 0
        ? `<button class="drill-add-all-btn">⊕ Add All</button>`
        : "") +
      `</div>`;
    if (tracks.length === 0) {
      this._setListContent(backBar + '<div class="list-empty">No results</div>');
      return;
    }
    const items = tracks
      .map((t, i) => {
        const sub = [formatArtists(t.ByArtist), t.InAlbum].filter(Boolean).join(" · ");
        return (
          `<div class="list-item">` +
          `<div class="list-item-main">` +
          `<div class="list-item-title">${this._escHtml(t.Name || "(untitled)")}</div>` +
          (sub ? `<div class="list-item-sub">${this._escHtml(sub)}</div>` : "") +
          `</div>` +
          `<button class="row-add-btn" title="Add to queue" data-drilldown-index="${i}">⊕</button>` +
          `</div>`
        );
      })
      .join("");
    this._setListContent(backBar + items);
  }

  /** _drillDownFolder fetches all tracks under a folder path and renders them in the list panel. */
  private _drillDownFolder(path: string): void {
    this._setListContent('<div class="list-empty">Loading…</div>');
    const parts = path.replace(/\\/g, "/").split("/").filter(Boolean);
    const label = `📁 ${parts[parts.length - 1] || path}`;
    this.api.listFolderTracks(path)
      .then((tracks) => this._showDrilldown(tracks, label))
      .catch((err) => this._setListContent(
        `<div class="list-empty">${this._escHtml(String(err))}</div>`,
      ));
  }

  /** _drillDownAlbum fetches all tracks under an album directory and renders them
   * in the list panel. Uses the exact directory rather than a tag-based search so
   * a tag/directory-name mismatch or a similarly-named sibling album never causes
   * wrong or missing tracks.
   */
  private _drillDownAlbum(dir: string, label: string): void {
    this._setListContent('<div class="list-empty">Loading…</div>');
    this.api.listAlbumTracks(dir)
      .then((tracks) => this._showDrilldown(tracks, label))
      .catch((err) => this._setListContent(
        `<div class="list-empty">${this._escHtml(String(err))}</div>`,
      ));
  }

  /** _drillDown fetches tracks for a browse item and renders them in the list panel. */
  private _drillDown(tab: string, item: string): void {
    this._setListContent('<div class="list-empty">Loading…</div>');
    const q = buildBrowseQuery(tab, item);
    const label = `${tab}: ${item}`;
    this.api.search(q)
      .then((tracks) => this._showDrilldown(tracks, label))
      .catch((err) => this._setListContent(
        `<div class="list-empty">${this._escHtml(String(err))}</div>`,
      ));
  }

  /** _addToQueue appends tracks to the playback queue without starting playback.
   * When the queue was empty the first track is primed in the player but not played.
   *
   * Parameters:
   *   tracks (AudioInfo[]) — tracks to append
   */
  private _addToQueue(tracks: AudioInfo[]): void {
    if (tracks.length === 0) return;
    const wasEmpty = this.queue.length === 0 && this.currentIndex < 0;
    this.queue = [...this.queue, ...tracks];
    if (wasEmpty) {
      this.currentIndex = 0;
      this.audioEl.src = this.api.audioUrl(this.queue[0].ID);
      this._showNowPlaying(this.queue[0]);
      this._refreshPlayState(false);
    }
    this._updateQueuePanel();
  }

  /** _shuffleQueue applies a Fisher-Yates shuffle to all unplayed tracks in
   * the queue (those after currentIndex). The currently-playing track and any
   * already-played tracks are left in place. If nothing is playing the entire
   * queue is shuffled.
   */
  private _shuffleQueue(): void {
    const splitAt = this.currentIndex + 1;
    const played = this.queue.slice(0, splitAt);
    const remaining = this.queue.slice(splitAt);
    for (let i = remaining.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [remaining[i], remaining[j]] = [remaining[j], remaining[i]];
    }
    this.queue = [...played, ...remaining];
    this._updateQueuePanel();
  }

  // ---- playback -----------------------------------------------------------

  private _playIndex(index: number): void {
    if (index < 0 || index >= this.queue.length) return;
    this.currentIndex = index;
    const info = this.queue[index];
    this.audioEl.src = this.api.audioUrl(info.ID);
    this.audioEl.play().catch(() => {});
    this._showNowPlaying(info);
    this._updateQueuePanel();
  }

  private _showNowPlaying(info: AudioInfo): void {
    this.qs(".track-title").textContent = info.Name || "(untitled)";
    const sub = [formatArtists(info.ByArtist), info.InAlbum].filter(Boolean).join(" — ");
    this.qs(".track-sub").textContent = sub;
    const parts = (info.ContentURL ?? "").replace(/\\/g, "/").split("/");
    const folder = parts.length >= 2 ? parts[parts.length - 2] : "";
    this.qs(".context-info").textContent = folder ? `📁 ${folder}` : "";
  }

  private _refreshPlayState(playing: boolean): void {
    this.qs(".play-pause-btn").textContent = playing ? "⏸" : "▶";
    (this.qs<HTMLButtonElement>(".prev-btn")).disabled = this.currentIndex <= 0;
    (this.qs<HTMLButtonElement>(".next-btn")).disabled =
      this.currentIndex >= this.queue.length - 1;
  }

  private _updateQueuePanel(): void {
    const list = this.qs(".queue-list");
    const title = this.qs(".queue-title");
    const shuffleBtn = this.qs<HTMLButtonElement>(".shuffle-btn");
    const clearBtn = this.qs<HTMLButtonElement>(".clear-queue-btn");
    const savePlaylistBtn = this.qs<HTMLButtonElement>(".save-playlist-btn");
    shuffleBtn.disabled = this.queue.length === 0;
    clearBtn.disabled = this.queue.length === 0;
    savePlaylistBtn.disabled = this.queue.length === 0;
    if (this.queue.length === 0) {
      list.innerHTML = '<div class="list-empty">No tracks queued</div>';
      title.textContent = "Queue";
      return;
    }
    const totalSecs = this.queue.reduce((sum, t) => sum + parseDurationSecs(t.Duration), 0);
    const totalStr = totalSecs > 0 ? this._fmtSecs(totalSecs) : "";
    title.textContent = totalStr
      ? `Queue (${this.queue.length} · ~${totalStr})`
      : `Queue (${this.queue.length})`;
    list.innerHTML = this.queue
      .map((t, i) => {
        const cls = i === this.currentIndex ? " current" : "";
        return (
          `<div class="queue-item${cls}" data-queue-index="${i}">` +
          `<span class="queue-item-name">${this._escHtml(t.Name || "(untitled)")}</span>` +
          `<button class="queue-remove-btn" data-remove-index="${i}">×</button>` +
          `</div>`
        );
      })
      .join("");
  }

  // ---- audio event binding ------------------------------------------------

  private _bindAudio(): void {
    this.audioEl.addEventListener("play", () => this._refreshPlayState(true));
    this.audioEl.addEventListener("pause", () => this._refreshPlayState(false));
    this.audioEl.addEventListener("ended", () => {
      this.queue.splice(this.currentIndex, 1);
      if (this.currentIndex < this.queue.length) {
        this._playIndex(this.currentIndex);
      } else {
        this.currentIndex = -1;
        this._refreshPlayState(false);
        this._initPlayer();
        this._updateQueuePanel();
      }
    });
    this.audioEl.addEventListener("timeupdate", () => {
      if (this.seekDragging || isNaN(this.audioEl.duration)) return;
      const pct = (this.audioEl.currentTime / this.audioEl.duration) * 100;
      (this.qs<HTMLInputElement>(".seek-bar")).value = String(pct);
      this.qs(".current-time").textContent = this._fmtSecs(this.audioEl.currentTime);
      this.qs(".total-time").textContent = this._fmtSecs(this.audioEl.duration);
    });
    this.audioEl.addEventListener("loadedmetadata", () => {
      if (!isNaN(this.audioEl.duration)) {
        this.qs(".total-time").textContent = this._fmtSecs(this.audioEl.duration);
      }
    });
  }

  // ---- UI control binding -------------------------------------------------

  private _bindControls(): void {
    // Tab buttons
    this.shadow.querySelectorAll<HTMLButtonElement>(".tab").forEach((btn) => {
      btn.addEventListener("click", () => this._loadTab(btn.dataset.tab!));
    });

    // Search
    const searchInput = this.qs<HTMLInputElement>(".search-bar input");
    this.qs(".search-btn").addEventListener("click", () => this._runSearch(searchInput.value));
    searchInput.addEventListener("keydown", (e: Event) => {
      if ((e as KeyboardEvent).key === "Enter") this._runSearch(searchInput.value);
    });

    // Browse/result list clicks (delegated on list-panel)
    this.qs(".list-panel").addEventListener("click", (e: Event) => {
      // Folder toggle button.
      const toggleBtn = (e.target as Element).closest<HTMLElement>("[data-toggle-folder]");
      if (toggleBtn) {
        const path = toggleBtn.dataset.toggleFolder!;
        this.folderEnabled.set(path, !this._isFolderEnabled(path));
        // Persist to server (fire-and-forget; log errors only).
        this.api.setExcludedFolders(this._getExcludedFolderPaths())
          .catch((e) => console.warn("save folder exclusions:", e));
        // Re-render the folder tree immediately.
        if (this.folderCache) this._renderFolderList(this.folderCache);
        // Refresh whichever browse tab is active so exclusions take effect.
        const activeTab = this.shadow.querySelector<HTMLButtonElement>(".tab.active");
        const activeTabName = activeTab?.dataset.tab;
        if (activeTabName && activeTabName !== "folders") {
          this._loadTab(activeTabName);
        }
        return;
      }

      // Playlist load button.
      const plLoadBtn = (e.target as Element).closest<HTMLElement>("[data-playlist-load]");
      if (plLoadBtn) {
        const id = plLoadBtn.dataset.playlistLoad!;
        this.api.loadPlaylist(id)
          .then((tracks) => this._addToQueue(tracks))
          .catch((err) => console.warn("load playlist:", String(err)));
        return;
      }

      // Playlist delete button.
      const plDelBtn = (e.target as Element).closest<HTMLElement>("[data-playlist-delete]");
      if (plDelBtn) {
        const id = plDelBtn.dataset.playlistDelete!;
        if (!confirm("Delete this playlist?")) return;
        this.api.deletePlaylist(id)
          .then(() => this._loadTab("playlists"))
          .catch((err) => console.warn("delete playlist:", String(err)));
        return;
      }

      // ⊖ remove-from-queue button.
      const removeBtn = (e.target as Element).closest<HTMLElement>(".row-remove-btn");
      if (removeBtn) {
        const id = removeBtn.dataset.removeTrackId ?? "";
        const idx = this.queue.findIndex((t) => t.ID === id);
        if (idx !== -1) {
          const wasPlaying = idx === this.currentIndex;
          this.queue.splice(idx, 1);
          if (wasPlaying) {
            this.audioEl.pause();
            this.audioEl.src = "";
            this.currentIndex = -1;
            this._initPlayer();
          } else if (idx < this.currentIndex) {
            this.currentIndex--;
          }
          this._updateQueuePanel();
        }
        return;
      }

      // ⊕ add-to-queue button takes priority over the row click.
      const addBtn = (e.target as Element).closest<HTMLElement>(".row-add-btn");
      if (addBtn && !(addBtn as HTMLButtonElement).disabled) {
        // Search result: track identified by ID stored in the results map.
        const searchId = addBtn.dataset.searchTrackId;
        if (searchId !== undefined) {
          const track = this.searchResultsMap.get(searchId);
          if (track) this._addToQueue([track]);
          return;
        }
        // Drilldown: track identified by index into drilldownTracks.
        const ddIdx = addBtn.dataset.drilldownIndex;
        if (ddIdx !== undefined) {
          const track = this.drilldownTracks[parseInt(ddIdx, 10)];
          if (track) this._addToQueue([track]);
          return;
        }
        const tab = addBtn.dataset.addTab;
        const item = addBtn.dataset.addItem ?? "";
        if (tab === "folders") {
          this.api.listFolderTracks(item)
            .then((tracks) => this._addToQueue(tracks))
            .catch((err) => console.warn("add folder to queue:", String(err)));
        } else if (tab === "albums") {
          this.api.listAlbumTracks(item)
            .then((tracks) => this._addToQueue(tracks))
            .catch((err) => console.warn("add album to queue:", String(err)));
        } else if (tab) {
          const q = buildBrowseQuery(tab, item);
          this.api.search(q)
            .then((tracks) => this._addToQueue(tracks))
            .catch((err) => console.warn("add to queue:", String(err)));
        }
        return;
      }

      // Back button → return to current browse tab.
      if ((e.target as Element).closest(".drill-back-btn")) {
        this._loadTab(this.currentBrowseTab);
        return;
      }

      // Add-all button — search group (has data-group-label) or drilldown.
      const addAllBtn = (e.target as Element).closest<HTMLElement>(".drill-add-all-btn");
      if (addAllBtn) {
        if (addAllBtn.dataset.groupLabel !== undefined) {
          this._addToQueue(this.searchGroups.get(addAllBtn.dataset.groupLabel) ?? []);
        } else {
          this._addToQueue(this.drilldownTracks);
        }
        return;
      }

      // Browse row click → drill down into tracks.
      const el = (e.target as Element).closest<HTMLElement>(".list-item");
      if (!el) return;
      if (el.dataset.browseTab) {
        if (el.dataset.browseTab === "folders") {
          this._drillDownFolder(el.dataset.browseItem ?? "");
        } else if (el.dataset.browseTab === "albums") {
          const dir = el.dataset.browseItem ?? "";
          const label = el.dataset.browseLabel ?? dir;
          this._drillDownAlbum(dir, `Album: ${label}`);
        } else if (el.dataset.browseTab === "artists") {
          this._drillDownArtistAlbums(el.dataset.browseItem ?? "");
        } else {
          this._drillDown(el.dataset.browseTab, el.dataset.browseItem ?? "");
        }
      }
    });

    // Queue clicks (delegated on queue-list)
    this.qs(".queue-list").addEventListener("click", (e: Event) => {
      // Remove button
      const removeBtn = (e.target as Element).closest<HTMLElement>(".queue-remove-btn");
      if (removeBtn) {
        const idx = parseInt(removeBtn.dataset.removeIndex ?? "-1", 10);
        if (idx < 0 || idx >= this.queue.length) return;
        const wasPlaying = idx === this.currentIndex;
        this.queue.splice(idx, 1);
        if (wasPlaying) {
          this.audioEl.pause();
          this.audioEl.src = "";
          this.currentIndex = -1;
          this._initPlayer();
        } else if (idx < this.currentIndex) {
          this.currentIndex--;
        }
        this._updateQueuePanel();
        return;
      }

      // Queue item click → play that track
      const el = (e.target as Element).closest<HTMLElement>(".queue-item");
      if (el?.dataset.queueIndex !== undefined) {
        this._playIndex(parseInt(el.dataset.queueIndex, 10));
      }
    });

    // Clear queue
    this.qs(".clear-queue-btn").addEventListener("click", () => {
      this.queue = [];
      this.currentIndex = -1;
      this.audioEl.pause();
      this.audioEl.src = "";
      this._initPlayer();
      this._updateQueuePanel();
    });

    // Playback buttons
    this.qs(".play-pause-btn").addEventListener("click", () => {
      if (this.audioEl.paused) {
        if (!this.audioEl.src && this.queue.length > 0) this._playIndex(0);
        else this.audioEl.play().catch(() => {});
      } else {
        this.audioEl.pause();
      }
    });
    this.qs(".prev-btn").addEventListener("click", () => {
      if (this.currentIndex > 0) this._playIndex(this.currentIndex - 1);
    });
    this.qs(".next-btn").addEventListener("click", () => {
      if (this.currentIndex < this.queue.length - 1) this._playIndex(this.currentIndex + 1);
    });

    // Seek bar
    const seekBar = this.qs<HTMLInputElement>(".seek-bar");
    seekBar.addEventListener("mousedown", () => { this.seekDragging = true; });
    seekBar.addEventListener("input", () => {
      if (!isNaN(this.audioEl.duration)) {
        this.audioEl.currentTime = (parseFloat(seekBar.value) / 100) * this.audioEl.duration;
      }
    });
    seekBar.addEventListener("mouseup", () => { this.seekDragging = false; });

    // Volume
    const volBar = this.qs<HTMLInputElement>(".volume-bar");
    volBar.addEventListener("input", () => {
      this.audioEl.volume = parseFloat(volBar.value);
    });

    // Queue toggle
    const toggleQueueBtn = this.qs<HTMLButtonElement>(".toggle-queue-btn");
    const queueList = this.qs<HTMLElement>(".queue-list");
    toggleQueueBtn.addEventListener("click", () => {
      this.queueVisible = !this.queueVisible;
      queueList.style.display = this.queueVisible ? "" : "none";
      toggleQueueBtn.textContent = this.queueVisible ? "Hide" : "Show";
    });

    // Library toggle
    const toggleLibraryBtn = this.qs<HTMLButtonElement>(".toggle-library-btn");
    const libraryBody = this.qs<HTMLElement>(".library-body");
    toggleLibraryBtn.addEventListener("click", () => {
      this.libraryVisible = !this.libraryVisible;
      libraryBody.style.display = this.libraryVisible ? "" : "none";
      toggleLibraryBtn.textContent = this.libraryVisible ? "Hide" : "Show";
    });

    // Shuffle queue
    this.qs(".shuffle-btn").addEventListener("click", () => this._shuffleQueue());

    // Save queue as playlist
    this.qs(".save-playlist-btn").addEventListener("click", () => this._saveQueueAsPlaylist());

    // Scan / Sweep — capture elements here for the same reason as share.
    this.qs(".scan-btn").addEventListener("click", () => this._startScan());
    this.qs(".sweep-btn").addEventListener("click", () => this._startSweep());

    // Share — capture elements once so async handlers never re-query a potentially stale DOM.
    const shareBtn = this.qs<HTMLButtonElement>(".share-btn");
    const shareStatus = this.qs<HTMLElement>(".share-status");
    shareBtn.addEventListener("click", async () => {
      shareBtn.disabled = true;
      shareStatus.className = "lib-status";
      shareStatus.textContent = "Loading…";
      let addresses: string[];
      try {
        addresses = await this.api.shareAddresses();
      } catch (e) {
        shareBtn.disabled = false;
        shareStatus.className = "lib-status error";
        shareStatus.textContent = String(e);
        return;
      }
      if (addresses.length === 0) {
        shareBtn.disabled = false;
        shareStatus.className = "lib-status error";
        shareStatus.textContent = "No network interfaces available";
        return;
      }
      if (addresses.length === 1) {
        await this._enableShare(shareBtn, shareStatus, addresses[0]);
      } else {
        shareBtn.disabled = false;
        shareStatus.textContent = "";
        this._showSharePicker(shareBtn, shareStatus, addresses);
      }
    });

    // Shutdown
    this.qs(".shutdown-btn").addEventListener("click", () => this._requestShutdown());
  }

  // ---- share actions ------------------------------------------------------

  private async _initShareStatus(): Promise<void> {
    // Capture elements here so _applyShareStatus has stable refs from startup.
    const shareBtn = this.qs<HTMLButtonElement>(".share-btn");
    const shareStatus = this.qs<HTMLElement>(".share-status");
    try {
      const s = await this.api.shareStatus();
      this._applyShareStatus(shareBtn, shareStatus, s);
    } catch (_e) {
      // Server may not support share endpoints — silently skip.
    }
  }

  private _applyShareStatus(
    shareBtn: HTMLButtonElement,
    shareStatus: HTMLElement,
    s: ShareStatus,
  ): void {
    if (s.sharing) {
      shareBtn.disabled = true;
      shareBtn.textContent = "Sharing";
      shareStatus.className = "lib-status ok";
      shareStatus.innerHTML =
        `<span class="share-url-text">${this._escHtml(s.share_url)}</span> ` +
        `<button class="share-copy-btn lib-btn">Copy</button> ` +
        `<button class="share-disable-btn lib-btn">Disable</button>`;
      shareStatus.querySelector(".share-copy-btn")?.addEventListener("click", () => {
        navigator.clipboard?.writeText(s.share_url).catch(() => {});
      });
      shareStatus.querySelector(".share-disable-btn")?.addEventListener("click", async () => {
        shareStatus.textContent = "Stopping…";
        try {
          await this.api.shareOff();
          await this._pollShareStatus(shareBtn, shareStatus, false);
        } catch (e) {
          shareStatus.className = "lib-status error";
          shareStatus.textContent = String(e);
        }
      });
    } else {
      shareBtn.disabled = false;
      shareBtn.textContent = "Share";
      shareStatus.className = "lib-status";
      shareStatus.innerHTML = "";
    }
  }

  private async _enableShare(
    shareBtn: HTMLButtonElement,
    shareStatus: HTMLElement,
    address: string,
  ): Promise<void> {
    shareStatus.className = "lib-status";
    shareStatus.textContent = "Starting…";
    try {
      await this.api.shareOn(address);
      await this._pollShareStatus(shareBtn, shareStatus, true);
    } catch (e) {
      shareBtn.disabled = false;
      shareStatus.className = "lib-status error";
      shareStatus.textContent = String(e);
    }
  }

  private _showSharePicker(
    shareBtn: HTMLButtonElement,
    shareStatus: HTMLElement,
    addresses: string[],
  ): void {
    const opts = addresses
      .map((a) => `<option value="${this._escAttr(a)}">${this._escHtml(a)}</option>`)
      .join("");
    shareStatus.className = "lib-status";
    shareStatus.innerHTML =
      `<select class="share-addr-select">${opts}</select> ` +
      `<button class="share-confirm-btn lib-btn">Enable</button> ` +
      `<button class="share-cancel-btn lib-btn">Cancel</button>`;
    shareStatus.querySelector(".share-confirm-btn")?.addEventListener("click", async () => {
      const sel = shareStatus.querySelector<HTMLSelectElement>(".share-addr-select");
      const addr = sel?.value ?? "";
      if (addr) await this._enableShare(shareBtn, shareStatus, addr);
    });
    shareStatus.querySelector(".share-cancel-btn")?.addEventListener("click", () => {
      shareBtn.disabled = false;
      shareStatus.className = "lib-status";
      shareStatus.innerHTML = "";
    });
  }

  private _pollShareStatus(
    shareBtn: HTMLButtonElement,
    shareStatus: HTMLElement,
    waitFor: boolean,
  ): Promise<void> {
    return new Promise((resolve, reject) => {
      let attempts = 0;
      const timer = setInterval(async () => {
        attempts++;
        if (attempts > 30) {
          clearInterval(timer);
          reject(new Error("timed out waiting for share status change"));
          return;
        }
        try {
          const s = await this.api.shareStatus();
          if (s.sharing === waitFor) {
            clearInterval(timer);
            this._applyShareStatus(shareBtn, shareStatus, s);
            resolve();
          }
        } catch (_e) { /* keep polling */ }
      }, 1000) as unknown as number;
    });
  }

  // ---- library actions ----------------------------------------------------

  private _startScan(): void {
    const btn = this.qs<HTMLButtonElement>(".scan-btn");
    const status = this.qs<HTMLElement>(".scan-status");
    btn.disabled = true;
    status.className = "lib-status";
    status.textContent = "Starting…";
    this.api.startScan()
      .then((s) => {
        this._pollScan(btn, status, new Date(s.started_at).getTime());
      })
      .catch((e) => {
        btn.disabled = false;
        status.className = "lib-status error";
        status.textContent = String(e);
      });
  }

  private _pollScan(btn: HTMLButtonElement, status: HTMLElement, startedAt: number): void {
    if (this.scanPollTimer) clearInterval(this.scanPollTimer);
    this.scanPollTimer = setInterval(async () => {
      try {
        const s = await this.api.scanStatus();
        if (s.status === "completed") {
          clearInterval(this.scanPollTimer);
          btn.disabled = false;
          status.className = "lib-status ok";
          status.textContent = "Scan complete";
          const activeTab = this.shadow.querySelector<HTMLButtonElement>(".tab.active");
          if (activeTab?.dataset.tab) this._loadTab(activeTab.dataset.tab);
        } else if (s.status === "error") {
          clearInterval(this.scanPollTimer);
          btn.disabled = false;
          status.className = "lib-status error";
          status.textContent = `Error: ${s.error ?? "unknown"}`;
        } else if (s.status === "idle") {
          clearInterval(this.scanPollTimer);
          btn.disabled = false;
          status.className = "lib-status";
          status.textContent = "";
        } else {
          const elapsed = Math.floor((Date.now() - startedAt) / 1000);
          status.textContent = `Scanning… (${elapsed}s)`;
        }
      } catch (e) {
        console.warn("scan poll:", e);
      }
    }, 1500) as unknown as number;
  }

  private _startSweep(): void {
    const btn = this.qs<HTMLButtonElement>(".sweep-btn");
    const status = this.qs<HTMLElement>(".sweep-status");
    btn.disabled = true;
    status.className = "lib-status";
    status.textContent = "Starting…";
    this.api.startSweep()
      .then((s) => {
        this._pollSweep(btn, status, new Date(s.started_at).getTime());
      })
      .catch((e) => {
        btn.disabled = false;
        status.className = "lib-status error";
        status.textContent = String(e);
      });
  }

  private _pollSweep(btn: HTMLButtonElement, status: HTMLElement, startedAt: number): void {
    if (this.sweepPollTimer) clearInterval(this.sweepPollTimer);
    this.sweepPollTimer = setInterval(async () => {
      try {
        const s = await this.api.sweepStatus();
        if (s.status === "completed") {
          clearInterval(this.sweepPollTimer);
          btn.disabled = false;
          const n = s.records_removed ?? 0;
          status.className = "lib-status ok";
          status.textContent = `${n} stale record${n !== 1 ? "s" : ""} removed`;
        } else if (s.status === "error") {
          clearInterval(this.sweepPollTimer);
          btn.disabled = false;
          status.className = "lib-status error";
          status.textContent = `Error: ${s.error ?? "unknown"}`;
        } else if (s.status === "idle") {
          clearInterval(this.sweepPollTimer);
          btn.disabled = false;
          status.className = "lib-status";
          status.textContent = "";
        } else {
          const elapsed = Math.floor((Date.now() - startedAt) / 1000);
          status.textContent = `Sweeping… (${elapsed}s)`;
        }
      } catch (e) {
        console.warn("sweep poll:", e);
      }
    }, 1500) as unknown as number;
  }

  private async _requestShutdown(): Promise<void> {
    if (!confirm("Shut down the audiobox server?\n\nThe web UI will no longer be accessible.")) return;
    try {
      await this.api.shutdown();
    } catch (_e) {
      // Connection drop is expected as part of shutdown — ignore.
    }
    // Replace entire shadow content with a shutdown message.
    this.shadowRoot!.innerHTML = `
      <style>
        :host { display: flex; align-items: center; justify-content: center;
                min-height: 200px; font-family: system-ui, sans-serif; }
        .msg { text-align: center; color: #555; }
        .msg h2 { margin: 0 0 8px; font-size: 18px; color: #1a1a1a; }
        .msg p  { margin: 0; font-size: 13px; }
      </style>
      <div class="msg">
        <h2>Server is shutting down</h2>
        <p>You can close this tab.</p>
      </div>
    `;
  }

  // ---- folder inclusion helpers -------------------------------------------

  private _getExcludedFolderPaths(): string[] {
    const excluded: string[] = [];
    for (const [path, enabled] of this.folderEnabled) {
      if (enabled) continue;
      // Skip a child path if its root is already excluded (root covers all sub-paths).
      const slashIdx = path.indexOf("/");
      if (slashIdx !== -1) {
        const root = path.slice(0, slashIdx);
        if (!this._isFolderEnabled(root)) continue;
      }
      excluded.push(path);
    }
    return excluded;
  }

  private _isFolderEnabled(path: string): boolean {
    const v = this.folderEnabled.get(path);
    return v === undefined ? true : v;
  }

  /** _isFolderEffectivelyEnabled reports whether path AND every ancestor of
   * path is enabled, so a subfolder can never appear addable while an
   * ancestor several levels up has been excluded.
   */
  private _isFolderEffectivelyEnabled(path: string): boolean {
    const parts = path.split("/");
    let acc = "";
    for (const p of parts) {
      acc = acc ? `${acc}/${p}` : p;
      if (!this._isFolderEnabled(acc)) return false;
    }
    return true;
  }

  private async _loadFolderEnabledFromServer(): Promise<void> {
    try {
      const excluded = await this.api.getExcludedFolders();
      this.folderEnabled.clear();
      for (const path of excluded) {
        this.folderEnabled.set(path, false);
      }
    } catch { /* server may not have saved exclusions yet — start with all enabled */ }
  }

  private _deslugify(s: string): string {
    return s.replace(/[-_]+/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
  }

  // ---- playlist actions ---------------------------------------------------

  private async _saveQueueAsPlaylist(): Promise<void> {
    if (this.queue.length === 0) return;
    const name = prompt("Playlist name:");
    if (!name || !name.trim()) return;
    const saveBtn = this.qs<HTMLButtonElement>(".save-playlist-btn");
    const title = this.qs<HTMLElement>(".queue-title");
    saveBtn.disabled = true;
    const prev = title.textContent ?? "";
    title.textContent = "Saving…";
    try {
      await this.api.savePlaylist(name.trim(), this.queue.map((t) => t.ID));
      title.textContent = `Saved as "${name.trim()}"`;
      setTimeout(() => {
        this._updateQueuePanel();
        // Refresh playlists tab if it is currently open.
        const activeTab = this.shadow.querySelector<HTMLButtonElement>(".tab.active");
        if (activeTab?.dataset.tab === "playlists") this._loadTab("playlists");
      }, 1500);
    } catch (e) {
      title.textContent = prev;
      console.warn("save playlist:", String(e));
      saveBtn.disabled = false;
    }
  }

  // ---- private helpers ----------------------------------------------------

  private _fmtSecs(secs: number): string {
    if (isNaN(secs)) return "0:00";
    const h = Math.floor(secs / 3600);
    const m = Math.floor((secs % 3600) / 60);
    const s = Math.floor(secs % 60);
    const mm = String(m).padStart(h > 0 ? 2 : 1, "0");
    const ss = String(s).padStart(2, "0");
    return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
  }

  private _escHtml(s: string): string {
    return s
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  private _escAttr(s: string): string {
    return s.replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  }
}

// Register the element only in browser environments.
if (typeof customElements !== "undefined") {
  customElements.define("audiobox-player", AudioInfoPlayer);
}
