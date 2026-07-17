/** Identifier represents a schema.org PropertyValue used to attach persistent IDs to a resource. */
export interface Identifier {
  propertyID: string;
  value: string;
  url?: string;
  name?: string;
}

/** Agent represents a schema.org Person or Organization. */
export interface Agent {
  type: string;
  name: string;
  identifiers?: Identifier[];
}

/** AudioInfo mirrors the Go AudioInfo struct as returned by the JSON API.
 * Field names match the Go struct field names (no json tags on the Go side).
 */
export interface AudioInfo {
  ID: string;
  Created: string;
  Updated: string;
  SchemaType: string;
  Name: string;
  Description: string;
  ContentURL: string;
  EncodingFormat: string;
  Duration: string;
  DatePublished: string;
  InLanguage: string;
  Genre: string;
  Identifiers: Identifier[];
  ByArtist: Agent[];
  InAlbum: string;
  IsrcCode: string;
  RecordingOf: string;
  Checksum: string;
  ChecksumAlgorithm: string;
}

/** AlbumEntry represents a single album returned by GET /api/list/albums.
 * name is the plain album name used as the search key.
 * displayName is the qualified label shown to the user (may include a
 * parent-directory qualifier when two albums share the same name).
 */
export interface AlbumEntry {
  name: string;
  displayName: string;
  dir: string;
}

/** FolderEntry represents a directory containing audio files, as returned by GET /api/list/folders.
 * path is relative to the collection's AudioDir.
 * name is the deslugified last path component.
 * trackCount is the number of tracks under the folder.
 */
export interface FolderEntry {
  path: string;
  name: string;
  trackCount: number;
}

/** CollectionStatus describes the current state of the audiobox collection. */
export interface CollectionStatus {
  initialized: boolean;
  version: string;
  collection_name: string;
  audio_dir: string;
  track_count: number;
}

/** ScanStatus describes the current state of an async collection scan or sweep. */
export interface ScanStatus {
  status: "idle" | "running" | "completed" | "error";
  started_at?: string;
  completed_at?: string;
  error?: string;
}

/** SweepStatus extends ScanStatus with a records_removed count. */
export interface SweepStatus extends ScanStatus {
  records_removed?: number;
}

/** ShareStatus describes the current network-sharing state of the server. */
export interface ShareStatus {
  sharing: boolean;
  share_address: string;
  share_url: string;
}

/** PlaylistInfo describes a saved playlist as returned by GET /api/playlists. */
export interface PlaylistInfo {
  id: string;
  name: string;
  trackCount: number;
  created: string;
}

/** ScanStarted is returned when POST /api/scan or POST /api/sweep accepts the request. */
export interface ScanStarted {
  status: string;
  started_at: string;
}

/** AudioInfoAPI is a typed fetch wrapper for all audiobox HTTP JSON endpoints.
 *
 * Parameters:
 *   baseUrl (string) — origin of the audiobox server; defaults to "" (same-origin)
 *
 * Example:
 *   const api = new AudioInfoAPI("http://localhost:8010");
 *   const albums = await api.listAlbums();
 */
export class AudioInfoAPI {
  private readonly baseUrl: string;

  constructor(baseUrl = "") {
    this.baseUrl = baseUrl.replace(/\/$/, "");
  }

  private async getJSON<T>(path: string): Promise<T> {
    const resp = await fetch(this.baseUrl + path);
    if (!resp.ok) {
      const body = await resp.json().catch(() => ({ error: resp.statusText })) as {
        error?: string;
      };
      throw new Error(body.error ?? resp.statusText);
    }
    return resp.json() as Promise<T>;
  }

  private async postJSON<T>(path: string): Promise<T> {
    const resp = await fetch(this.baseUrl + path, { method: "POST" });
    if (!resp.ok) {
      const body = await resp.json().catch(() => ({ error: resp.statusText })) as {
        error?: string;
      };
      throw new Error(body.error ?? resp.statusText);
    }
    return resp.json() as Promise<T>;
  }

  /** status returns the current collection status including whether it is initialized.
   *
   * Returns:
   *   Promise<CollectionStatus> — status, version, track count, and audio_dir
   *
   * Example:
   *   const s = await api.status();
   *   if (s.track_count === 0) { ... }
   */
  async status(): Promise<CollectionStatus> {
    return this.getJSON<CollectionStatus>("/api/status");
  }

  /** init initialises or upgrades the standard ~/Audio collection on the server.
   *
   * Returns:
   *   Promise<{status: string, audio_dir: string}> — confirmation with audio_dir path
   *
   * Example:
   *   await api.init();
   */
  async init(): Promise<{ status: string; audio_dir: string }> {
    return this.postJSON<{ status: string; audio_dir: string }>("/api/init");
  }

  /** listAlbums returns all album entries in the collection.
   * Each entry has a plain name (used for searching) and a displayName
   * (shown to the user; may include a qualifier when two albums share the same name).
   *
   * Returns:
   *   Promise<AlbumEntry[]> — sorted album entries
   *
   * Example:
   *   const albums = await api.listAlbums();
   *   albums.forEach(a => console.log(a.displayName));
   */
  async listAlbums(excludeFolders: string[] = []): Promise<AlbumEntry[]> {
    const q = excludeFolders.length > 0
      ? `?exclude=${excludeFolders.map(encodeURIComponent).join(",")}`
      : "";
    return this.getJSON<AlbumEntry[]>(`/api/list/albums${q}`);
  }

  /** listArtists returns all distinct artist names in the collection.
   *
   * Returns:
   *   Promise<string[]> — sorted artist names
   *
   * Example:
   *   const artists = await api.listArtists();
   */
  async listArtists(excludeFolders: string[] = []): Promise<string[]> {
    const q = excludeFolders.length > 0
      ? `?exclude=${excludeFolders.map(encodeURIComponent).join(",")}`
      : "";
    return this.getJSON<string[]>(`/api/list/artists${q}`);
  }

  /** listTitles returns all distinct recording titles in the collection.
   *
   * Returns:
   *   Promise<string[]> — sorted titles
   *
   * Example:
   *   const titles = await api.listTitles();
   */
  async listTitles(excludeFolders: string[] = []): Promise<string[]> {
    const q = excludeFolders.length > 0
      ? `?exclude=${excludeFolders.map(encodeURIComponent).join(",")}`
      : "";
    return this.getJSON<string[]>(`/api/list/titles${q}`);
  }

  /** listFolders returns all directories that contain audio files, sorted by path.
   * Each entry includes the path (relative to AudioDir), a deslugified name, and a track count.
   *
   * Returns:
   *   Promise<FolderEntry[]> — sorted folder entries
   *
   * Example:
   *   const folders = await api.listFolders();
   *   folders.forEach(f => console.log(f.name, f.trackCount));
   */
  async listFolders(): Promise<FolderEntry[]> {
    return this.getJSON<FolderEntry[]>("/api/list/folders");
  }

  /** listFolderTracks returns all audio files under the given folder path.
   *
   * Parameters:
   *   dir (string) — relative folder path as returned by listFolders(), e.g. "Jazz/Miles-Davis"
   *
   * Returns:
   *   Promise<AudioInfo[]> — tracks in disc/track/name order
   *
   * Example:
   *   const tracks = await api.listFolderTracks("Jazz/Miles-Davis/Kind-Of-Blue");
   */
  async listFolderTracks(dir: string): Promise<AudioInfo[]> {
    return this.getJSON<AudioInfo[]>(`/api/list/folder-tracks?dir=${encodeURIComponent(dir)}`);
  }

  /** listAlbumTracks returns all audio files under the given album directory.
   * Resolves by directory, not by tag — use this (rather than search) to load
   * an album's tracks once you already have its Album.dir from listAlbums(),
   * so a tag/directory-name mismatch or a similarly-named sibling album
   * never causes wrong or missing tracks.
   *
   * Parameters:
   *   dir (string) — album directory as returned by listAlbums(), e.g. "Jazz/Kind-Of-Blue"
   *
   * Returns:
   *   Promise<AudioInfo[]> — tracks in disc/track/name order
   *
   * Example:
   *   const albums = await api.listAlbums();
   *   const tracks = await api.listAlbumTracks(albums[0].dir);
   */
  async listAlbumTracks(dir: string): Promise<AudioInfo[]> {
    return this.getJSON<AudioInfo[]>(`/api/list/album-tracks?dir=${encodeURIComponent(dir)}`);
  }

  /** search queries the collection by title, album, or artist.
   *
   * Parameters:
   *   q (string) — query; supports plain terms, field:value, and /regex/ syntax
   *
   * Returns:
   *   Promise<AudioInfo[]> — matching records
   *
   * Example:
   *   const results = await api.search("artist:Bach");
   */
  async search(q: string): Promise<AudioInfo[]> {
    return this.getJSON<AudioInfo[]>(`/api/search?q=${encodeURIComponent(q)}`);
  }

  /** show returns full metadata for the record identified by UUID.
   *
   * Parameters:
   *   id (string) — UUID of the record
   *
   * Returns:
   *   Promise<AudioInfo> — full metadata record; throws on 404
   *
   * Example:
   *   const info = await api.show("550e8400-e29b-41d4-a716-446655440000");
   */
  async show(id: string): Promise<AudioInfo> {
    return this.getJSON<AudioInfo>(`/api/show/${encodeURIComponent(id)}`);
  }

  /** deleteRecord removes the record with the given UUID from the collection.
   *
   * Parameters:
   *   id (string) — UUID of the record to delete
   *
   * Returns:
   *   Promise<{status: string, id: string}> — confirmation; throws on 404
   *
   * Example:
   *   await api.deleteRecord("550e8400-e29b-41d4-a716-446655440000");
   */
  async deleteRecord(id: string): Promise<{ status: string; id: string }> {
    const resp = await fetch(
      this.baseUrl + `/api/show/${encodeURIComponent(id)}`,
      { method: "DELETE" },
    );
    if (!resp.ok) {
      const body = await resp.json().catch(() => ({ error: resp.statusText })) as {
        error?: string;
      };
      throw new Error(body.error ?? resp.statusText);
    }
    return resp.json() as Promise<{ status: string; id: string }>;
  }

  /** startScan initiates an asynchronous re-scan of the collection's music directory.
   *
   * Returns:
   *   Promise<ScanStarted> — confirmation with started_at timestamp; throws on 409 conflict
   *
   * Example:
   *   const result = await api.startScan();
   *   console.log(result.status); // "started"
   */
  async startScan(): Promise<ScanStarted> {
    const resp = await fetch(this.baseUrl + "/api/scan", { method: "POST" });
    if (resp.status !== 202) {
      const body = await resp.json().catch(() => ({ error: resp.statusText })) as {
        error?: string;
      };
      throw new Error(body.error ?? resp.statusText);
    }
    return resp.json() as Promise<ScanStarted>;
  }

  /** scanStatus returns the current state of the async scan.
   *
   * Returns:
   *   Promise<ScanStatus> — idle | running | completed | error with optional timestamps
   *
   * Example:
   *   const s = await api.scanStatus();
   *   if (s.status === "running") { ... }
   */
  async scanStatus(): Promise<ScanStatus> {
    return this.getJSON<ScanStatus>("/api/scan/status");
  }

  /** startSweep initiates an asynchronous sweep to remove stale database records.
   *
   * Returns:
   *   Promise<ScanStarted> — confirmation with started_at timestamp; throws on 409 conflict
   *
   * Example:
   *   const result = await api.startSweep();
   */
  async startSweep(): Promise<ScanStarted> {
    const resp = await fetch(this.baseUrl + "/api/sweep", { method: "POST" });
    if (resp.status !== 202) {
      const body = await resp.json().catch(() => ({ error: resp.statusText })) as {
        error?: string;
      };
      throw new Error(body.error ?? resp.statusText);
    }
    return resp.json() as Promise<ScanStarted>;
  }

  /** sweepStatus returns the current state of the async sweep.
   *
   * Returns:
   *   Promise<SweepStatus> — idle | running | completed | error; completed includes records_removed
   *
   * Example:
   *   const s = await api.sweepStatus();
   *   if (s.status === "completed") console.log(s.records_removed);
   */
  async sweepStatus(): Promise<SweepStatus> {
    return this.getJSON<SweepStatus>("/api/sweep/status");
  }

  /** shareStatus returns the current network-sharing state of the server.
   *
   * Returns:
   *   Promise<ShareStatus> — {sharing, share_address, share_url}
   *
   * Example:
   *   const s = await api.shareStatus();
   *   if (s.sharing) console.log("Sharing at", s.share_url);
   */
  async shareStatus(): Promise<ShareStatus> {
    return this.getJSON<ShareStatus>("/api/share/status");
  }

  /** shareAddresses returns the available non-loopback IPv4 addresses on the host.
   *
   * Returns:
   *   Promise<string[]> — list of IPv4 address strings
   *
   * Example:
   *   const addrs = await api.shareAddresses();
   */
  async shareAddresses(): Promise<string[]> {
    return this.getJSON<string[]>("/api/share/addresses");
  }

  /** shareOn enables LAN sharing on the given IPv4 address and restarts the listener.
   * The server responds immediately; poll shareStatus() until sharing === true.
   *
   * Parameters:
   *   address (string) — IPv4 address to bind the LAN listener to
   *
   * Returns:
   *   Promise<{status, poll_url}> — "restarting" acknowledgement
   *
   * Example:
   *   const r = await api.shareOn("192.168.1.5");
   */
  async shareOn(address: string): Promise<{ status: string; poll_url: string }> {
    const resp = await fetch(this.baseUrl + "/api/share/on", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ address }),
    });
    if (!resp.ok) {
      const body = await resp.json().catch(() => ({ error: resp.statusText })) as {
        error?: string;
      };
      throw new Error(body.error ?? resp.statusText);
    }
    return resp.json() as Promise<{ status: string; poll_url: string }>;
  }

  /** shareOff disables LAN sharing and restarts the listener on loopback only.
   * The server responds immediately; poll shareStatus() until sharing === false.
   *
   * Returns:
   *   Promise<{status, poll_url}> — "restarting" acknowledgement
   *
   * Example:
   *   const r = await api.shareOff();
   */
  async shareOff(): Promise<{ status: string; poll_url: string }> {
    return this.postJSON<{ status: string; poll_url: string }>("/api/share/off");
  }

  /** getExcludedFolders returns the list of folder paths currently excluded from browse views.
   *
   * Returns:
   *   Promise<string[]> — excluded folder paths (relative to AudioDir); empty when none are excluded
   *
   * Example:
   *   const excluded = await api.getExcludedFolders();
   */
  async getExcludedFolders(): Promise<string[]> {
    return this.getJSON<string[]>("/api/excluded-folders");
  }

  /** setExcludedFolders saves the list of excluded folder paths to the server config.
   * Pass an empty array to clear all exclusions.
   *
   * Parameters:
   *   excluded (string[]) — folder paths to exclude from browse views
   *
   * Returns:
   *   Promise<string[]> — the saved list as confirmed by the server
   *
   * Example:
   *   await api.setExcludedFolders(["Music/Seasonal"]);
   */
  async setExcludedFolders(excluded: string[]): Promise<string[]> {
    const resp = await fetch(this.baseUrl + "/api/excluded-folders", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ excluded }),
    });
    if (!resp.ok) {
      const body = await resp.json().catch(() => ({ error: resp.statusText })) as {
        error?: string;
      };
      throw new Error(body.error ?? resp.statusText);
    }
    return resp.json() as Promise<string[]>;
  }

  /** listPlaylists returns all saved playlists ordered by creation time descending.
   *
   * Returns:
   *   Promise<PlaylistInfo[]> — playlist summaries
   *
   * Example:
   *   const lists = await api.listPlaylists();
   */
  async listPlaylists(): Promise<PlaylistInfo[]> {
    return this.getJSON<PlaylistInfo[]>("/api/playlists");
  }

  /** savePlaylist saves the given track IDs as a named playlist.
   *
   * Parameters:
   *   name     (string)   — display name for the playlist
   *   trackIds (string[]) — ordered list of AudioInfo UUIDs
   *
   * Returns:
   *   Promise<{status, id}> — confirmation with the new playlist UUID
   *
   * Example:
   *   const r = await api.savePlaylist("Morning Mix", queue.map(t => t.ID));
   */
  async savePlaylist(
    name: string,
    trackIds: string[],
  ): Promise<{ status: string; id: string }> {
    const resp = await fetch(this.baseUrl + "/api/playlists", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, trackIds }),
    });
    if (!resp.ok) {
      const body = await resp.json().catch(() => ({ error: resp.statusText })) as {
        error?: string;
      };
      throw new Error(body.error ?? resp.statusText);
    }
    return resp.json() as Promise<{ status: string; id: string }>;
  }

  /** loadPlaylist returns the ordered tracks for a saved playlist.
   *
   * Parameters:
   *   id (string) — UUID of the playlist
   *
   * Returns:
   *   Promise<AudioInfo[]> — tracks in playlist order; throws on 404
   *
   * Example:
   *   const tracks = await api.loadPlaylist("550e8400-e29b-41d4-a716-446655440000");
   */
  async loadPlaylist(id: string): Promise<AudioInfo[]> {
    return this.getJSON<AudioInfo[]>(`/api/playlists/${encodeURIComponent(id)}`);
  }

  /** deletePlaylist removes a playlist from the collection.
   *
   * Parameters:
   *   id (string) — UUID of the playlist to delete
   *
   * Returns:
   *   Promise<{status, id}> — confirmation; throws on 404
   *
   * Example:
   *   await api.deletePlaylist("550e8400-e29b-41d4-a716-446655440000");
   */
  async deletePlaylist(id: string): Promise<{ status: string; id: string }> {
    const resp = await fetch(
      this.baseUrl + `/api/playlists/${encodeURIComponent(id)}`,
      { method: "DELETE" },
    );
    if (!resp.ok) {
      const body = await resp.json().catch(() => ({ error: resp.statusText })) as {
        error?: string;
      };
      throw new Error(body.error ?? resp.statusText);
    }
    return resp.json() as Promise<{ status: string; id: string }>;
  }

  /** audioUrl returns the streaming URL for the audio file with the given UUID.
   * No network request is made.
   *
   * Parameters:
   *   id (string) — UUID of the record
   *
   * Returns:
   *   string — URL suitable for an HTMLAudioElement src attribute
   *
   * Example:
   *   audio.src = api.audioUrl("550e8400-e29b-41d4-a716-446655440000");
   */
  audioUrl(id: string): string {
    return `${this.baseUrl}/api/audio/${encodeURIComponent(id)}`;
  }

  /** shutdown requests the server to gracefully stop.
   *
   * Returns:
   *   Promise<{status: string}> — acknowledgement before the server exits
   *
   * Example:
   *   await api.shutdown();
   */
  async shutdown(): Promise<{ status: string }> {
    const resp = await fetch(this.baseUrl + "/api/shutdown", { method: "POST" });
    // Server may close the connection before sending a full response; treat any
    // network error here as expected and return a synthetic acknowledgement.
    if (!resp.ok) {
      return { status: "shutting down" };
    }
    return resp.json().catch(() => ({ status: "shutting down" })) as Promise<{ status: string }>;
  }
}
