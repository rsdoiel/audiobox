import type { Agent, AudioInfo, CollectionStatus } from "./audiobox_api.ts";
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
}
.list-item:hover { background: #f0f0f0; }
.list-item:last-child { border-bottom: none; }
.list-item-title {
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-size: 13px;
}
.list-item-sub {
  font-size: 11px; color: #888; white-space: nowrap;
  overflow: hidden; text-overflow: ellipsis; margin-top: 1px;
}
.list-empty { padding: 10px; color: #888; text-align: center; font-style: italic; }

/* ---- now-playing panel ---- */
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
.delete-track-btn {
  margin-left: auto; padding: 3px 8px; border: 1px solid #e74c3c;
  border-radius: 4px; background: #fff; color: #e74c3c;
  cursor: pointer; font-size: 12px;
}
.delete-track-btn:hover { background: #e74c3c; color: #fff; }
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
    <button class="delete-track-btn" title="Remove from collection">🗑 Delete</button>
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
  private contextLabel = "";
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

  // ---- data loading -------------------------------------------------------

  private async _loadTab(tab: string): Promise<void> {
    this.shadow.querySelectorAll<HTMLButtonElement>(".tab").forEach((b) => {
      b.classList.toggle("active", b.dataset.tab === tab);
    });
    this._setListContent('<div class="list-empty">Loading…</div>');
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
    this._setListContent('<div class="list-empty">Searching…</div>');
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
            `<div class="list-item" data-browse-tab="${this._escAttr(tab)}" data-browse-item="${this._escAttr(item)}">` +
            `<div class="list-item-title">${this._escHtml(item)}</div>` +
            `</div>`,
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
        .map((t, i) => {
          const sub = [formatArtists(t.ByArtist), t.InAlbum].filter(Boolean).join(" · ");
          return (
            `<div class="list-item" data-queue-index="${i}">` +
            `<div class="list-item-title">${this._escHtml(t.Name || "(untitled)")}</div>` +
            (sub ? `<div class="list-item-sub">${this._escHtml(sub)}</div>` : "") +
            `</div>`
          );
        })
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
    const deleteBtn = this.qs<HTMLButtonElement>(".delete-track-btn");
    deleteBtn.dataset.trackId = info.ID;
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
    if (this.queue.length === 0) {
      list.innerHTML = '<div class="list-empty">No tracks queued</div>';
      title.textContent = "Queue";
      return;
    }
    title.textContent = `Queue (${this.queue.length})`;
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
        const q = buildBrowseQuery(el.dataset.browseTab, el.dataset.browseItem ?? "");
        const label = `${el.dataset.browseTab}: ${el.dataset.browseItem}`;
        this._setListContent('<div class="list-empty">Loading…</div>');
        this.api
          .search(q)
          .then((results) => this._renderAudioList(results, label, true))
          .catch((err) => this._setListContent(`<div class="list-empty">${this._escHtml(String(err))}</div>`));
      } else if (el.dataset.queueIndex !== undefined) {
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

    // Delete current track
    this.qs(".delete-track-btn").addEventListener("click", async () => {
      const btn = this.qs<HTMLButtonElement>(".delete-track-btn");
      const id = btn.dataset.trackId;
      if (!id) return;
      const name = this.qs(".track-title").textContent || "this track";
      if (!confirm(`Remove "${name}" from the collection?\n\nThe audio file on disk will not be deleted.`)) return;
      try {
        await this.api.deleteRecord(id);
        // Remove from queue and advance if needed.
        this.queue.splice(this.currentIndex, 1);
        if (this.queue.length === 0) {
          this.currentIndex = -1;
          this.audioEl.pause();
          this.audioEl.src = "";
          this.qs(".now-playing").classList.add("hidden");
        } else {
          const nextIndex = Math.min(this.currentIndex, this.queue.length - 1);
          this._playIndex(nextIndex);
        }
        this._updateQueuePanel();
      } catch (e) {
        alert(`Delete failed: ${String(e)}`);
      }
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

    // Scan
    this.qs(".scan-btn").addEventListener("click", () => this._startScan());

    // Sweep
    this.qs(".sweep-btn").addEventListener("click", () => this._startSweep());

    // Shutdown
    this.qs(".shutdown-btn").addEventListener("click", () => this._requestShutdown());
  }

  // ---- library actions ----------------------------------------------------

  private _startScan(): void {
    const btn = this.qs<HTMLButtonElement>(".scan-btn");
    const status = this.qs<HTMLElement>(".scan-status");
    btn.disabled = true;
    status.className = "lib-status";
    status.textContent = "Starting…";
    this.api.startScan()
      .then(() => {
        status.textContent = "Scanning…";
        this._pollScan();
      })
      .catch((e) => {
        btn.disabled = false;
        status.className = "lib-status error";
        status.textContent = String(e);
      });
  }

  private _pollScan(): void {
    if (this.scanPollTimer) clearInterval(this.scanPollTimer);
    this.scanPollTimer = setInterval(async () => {
      try {
        const s = await this.api.scanStatus();
        const btn = this.qs<HTMLButtonElement>(".scan-btn");
        const status = this.qs<HTMLElement>(".scan-status");
        if (s.status === "completed") {
          clearInterval(this.scanPollTimer);
          btn.disabled = false;
          status.className = "lib-status ok";
          status.textContent = "Scan complete";
          // Refresh the current browse tab.
          const activeTab = this.shadow.querySelector<HTMLButtonElement>(".tab.active");
          if (activeTab?.dataset.tab) this._loadTab(activeTab.dataset.tab);
        } else if (s.status === "error") {
          clearInterval(this.scanPollTimer);
          btn.disabled = false;
          status.className = "lib-status error";
          status.textContent = `Error: ${s.error ?? "unknown"}`;
        } else {
          status.textContent = "Scanning…";
        }
      } catch (_e) {
        // transient network error — keep polling
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
      .then(() => {
        status.textContent = "Sweeping…";
        this._pollSweep();
      })
      .catch((e) => {
        btn.disabled = false;
        status.className = "lib-status error";
        status.textContent = String(e);
      });
  }

  private _pollSweep(): void {
    if (this.sweepPollTimer) clearInterval(this.sweepPollTimer);
    this.sweepPollTimer = setInterval(async () => {
      try {
        const s = await this.api.sweepStatus();
        const btn = this.qs<HTMLButtonElement>(".sweep-btn");
        const status = this.qs<HTMLElement>(".sweep-status");
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
        } else {
          status.textContent = "Sweeping…";
        }
      } catch (_e) {
        // transient network error — keep polling
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
