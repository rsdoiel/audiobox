import type { Agent, AudioInfo } from "./audioinfo_api.ts";
import { AudioInfoAPI } from "./audioinfo_api.ts";

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
  max-width: 640px;
}
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
  margin-top: 8px; max-height: 180px; overflow-y: auto;
  border: 1px solid #e8e8e8; border-radius: 4px; background: #fff;
}
.list-item {
  padding: 6px 10px; cursor: pointer; border-bottom: 1px solid #f0f0f0;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.list-item:hover { background: #f0f0f0; }
.list-item:last-child { border-bottom: none; }
.list-empty { padding: 10px; color: #888; text-align: center; font-style: italic; }
.now-playing {
  padding: 10px; background: #fff; border-bottom: 1px solid #ddd;
}
.now-playing.hidden { display: none; }
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
.queue-panel { padding: 6px 10px 10px; }
.queue-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px; }
.queue-title { font-weight: 600; font-size: 12px; text-transform: uppercase; color: #888; }
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
  font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.queue-item:hover { background: #f0f0f0; }
.queue-item.current { background: #e8f0fe; font-weight: 600; }
.queue-item:last-child { border-bottom: none; }
`;

/** PLAYER_TEMPLATE is the inner HTML injected into the component's shadow root.
 * Exported so tests can parse and assert on the DOM structure without a live component.
 */
export const PLAYER_TEMPLATE = `
<style>${STYLES}</style>
<div class="browse-panel">
  <div class="tabs">
    <button class="tab active" data-tab="titles">Titles</button>
    <button class="tab" data-tab="albums">Albums</button>
    <button class="tab" data-tab="artists">Artists</button>
  </div>
  <div class="search-bar">
    <input type="search" placeholder="Search..." />
    <button class="search-btn">Search</button>
  </div>
  <div class="list-panel">
    <div class="list-empty">Select a tab to browse</div>
  </div>
</div>
<div class="now-playing hidden">
  <div class="track-title"></div>
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
    <button class="toggle-queue-btn">Hide</button>
  </div>
  <div class="queue-list">
    <div class="list-empty">No tracks queued</div>
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

/** AudioInfoPlayer is a self-contained web component for browsing and playing an audioinfo collection.
 *
 * Attributes:
 *   api-url (string) — base URL of the audioinfo server; defaults to "" (same-origin)
 *
 * Usage:
 *   &lt;audioinfo-player api-url="http://localhost:8010"&gt;&lt;/audioinfo-player&gt;
 *
 * The component renders a browse panel (Titles / Albums / Artists tabs and a search box),
 * a now-playing panel that activates when a track is selected, and a collapsible queue panel.
 */
export class AudioInfoPlayer extends _Base {
  static get observedAttributes(): string[] {
    return ["api-url"];
  }

  private api!: AudioInfoAPI;
  private audioEl!: HTMLAudioElement;
  private queue: AudioInfo[] = [];
  private currentIndex = -1;
  private contextLabel = "";
  private queueVisible = true;
  private seekDragging = false;

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
    this._loadTab("titles");
  }

  private get shadow(): ShadowRoot {
    return this.shadowRoot!;
  }

  private qs<T extends Element>(sel: string): T {
    return this.shadow.querySelector<T>(sel)!;
  }

  // ---- data loading -------------------------------------------------------

  private async _loadTab(tab: string): Promise<void> {
    this.shadow.querySelectorAll<HTMLButtonElement>(".tab").forEach((b) => {
      b.classList.toggle("active", b.dataset.tab === tab);
    });
    this._setListContent('<div class="list-empty">Loading...</div>');
    try {
      let items: string[] = [];
      if (tab === "albums") items = await this.api.listAlbums();
      else if (tab === "artists") items = await this.api.listArtists();
      else items = await this.api.listTitles();
      this._renderStringList(items, tab);
    } catch (e) {
      this._setListContent(`<div class="list-empty">${this._escHtml(String(e))}</div>`);
    }
  }

  private async _runSearch(q: string): Promise<void> {
    if (!q.trim()) return;
    this._setListContent('<div class="list-empty">Searching...</div>');
    try {
      const results = await this.api.search(q.trim());
      this._renderAudioList(results, `Search: ${q.trim()}`, false);
    } catch (e) {
      this._setListContent(`<div class="list-empty">${this._escHtml(String(e))}</div>`);
    }
  }

  // ---- list rendering -----------------------------------------------------

  private _setListContent(html: string): void {
    this.qs(".list-panel").innerHTML = html;
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
            `<div class="list-item" data-browse-tab="${this._escAttr(tab)}" data-browse-item="${this._escAttr(item)}">${this._escHtml(item)}</div>`,
        )
        .join(""),
    );
  }

  private _renderAudioList(tracks: AudioInfo[], context: string, autoPlay: boolean): void {
    this.queue = tracks;
    this.contextLabel = context;
    if (tracks.length === 0) {
      this._setListContent('<div class="list-empty">No results</div>');
      this._updateQueuePanel();
      return;
    }
    this._setListContent(
      tracks
        .map(
          (t, i) =>
            `<div class="list-item" data-queue-index="${i}">${this._escHtml(t.Name || "(untitled)")}</div>`,
        )
        .join(""),
    );
    this._updateQueuePanel();
    if (autoPlay) this._playIndex(0);
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
    this.qs(".now-playing").classList.remove("hidden");
    this.qs(".track-title").textContent = info.Name || "(untitled)";
    const sub = [formatArtists(info.ByArtist), info.InAlbum].filter(Boolean).join(" — ");
    this.qs(".track-sub").textContent = sub;
    this.qs(".context-info").textContent = this.contextLabel
      ? `From: ${this.contextLabel}`
      : "";
  }

  private _refreshPlayState(playing: boolean): void {
    this.qs(".play-pause-btn").textContent = playing ? "⏸" : "▶";
    (this.qs<HTMLButtonElement>(".prev-btn")).disabled = this.currentIndex <= 0;
    (this.qs<HTMLButtonElement>(".next-btn")).disabled =
      this.currentIndex >= this.queue.length - 1;
  }

  private _updateQueuePanel(): void {
    const list = this.qs(".queue-list");
    if (this.queue.length === 0) {
      list.innerHTML = '<div class="list-empty">No tracks queued</div>';
      return;
    }
    list.innerHTML = this.queue
      .map((t, i) => {
        const cls = i === this.currentIndex ? " current" : "";
        return `<div class="queue-item${cls}" data-queue-index="${i}">${this._escHtml(t.Name || "(untitled)")}</div>`;
      })
      .join("");
  }

  // ---- audio event binding ------------------------------------------------

  private _bindAudio(): void {
    this.audioEl.addEventListener("play", () => this._refreshPlayState(true));
    this.audioEl.addEventListener("pause", () => this._refreshPlayState(false));
    this.audioEl.addEventListener("ended", () => {
      if (this.currentIndex < this.queue.length - 1) {
        this._playIndex(this.currentIndex + 1);
      } else {
        this._refreshPlayState(false);
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
      const el = (e.target as Element).closest<HTMLElement>(".list-item");
      if (!el) return;
      if (el.dataset.browseTab) {
        // String list item: load all tracks for this album/artist/title and auto-play
        const q = buildBrowseQuery(el.dataset.browseTab, el.dataset.browseItem ?? "");
        const label = `${el.dataset.browseTab}: ${el.dataset.browseItem}`;
        this._setListContent('<div class="list-empty">Loading...</div>');
        this.api
          .search(q)
          .then((results) => this._renderAudioList(results, label, true))
          .catch((err) => this._setListContent(`<div class="list-empty">${this._escHtml(String(err))}</div>`));
      } else if (el.dataset.queueIndex !== undefined) {
        // Track result: play the selected track
        this._playIndex(parseInt(el.dataset.queueIndex, 10));
      }
    });

    // Queue clicks (delegated on queue-list)
    this.qs(".queue-list").addEventListener("click", (e: Event) => {
      const el = (e.target as Element).closest<HTMLElement>(".queue-item");
      if (el?.dataset.queueIndex !== undefined) {
        this._playIndex(parseInt(el.dataset.queueIndex, 10));
      }
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
    seekBar.addEventListener("mousedown", () => {
      this.seekDragging = true;
    });
    seekBar.addEventListener("input", () => {
      if (!isNaN(this.audioEl.duration)) {
        this.audioEl.currentTime = (parseFloat(seekBar.value) / 100) * this.audioEl.duration;
      }
    });
    seekBar.addEventListener("mouseup", () => {
      this.seekDragging = false;
    });

    // Volume
    const volBar = this.qs<HTMLInputElement>(".volume-bar");
    volBar.addEventListener("input", () => {
      this.audioEl.volume = parseFloat(volBar.value);
    });

    // Queue toggle
    const toggleBtn = this.qs<HTMLButtonElement>(".toggle-queue-btn");
    const queueList = this.qs<HTMLElement>(".queue-list");
    toggleBtn.addEventListener("click", () => {
      this.queueVisible = !this.queueVisible;
      queueList.style.display = this.queueVisible ? "" : "none";
      toggleBtn.textContent = this.queueVisible ? "Hide" : "Show";
    });
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
  customElements.define("audioinfo-player", AudioInfoPlayer);
}
