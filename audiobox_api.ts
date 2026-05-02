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

  /** listAlbums returns all distinct album names in the collection.
   *
   * Returns:
   *   Promise<string[]> — sorted album names
   *
   * Example:
   *   const albums = await api.listAlbums();
   */
  async listAlbums(): Promise<string[]> {
    return this.getJSON<string[]>("/api/list/albums");
  }

  /** listArtists returns all distinct artist names in the collection.
   *
   * Returns:
   *   Promise<string[]> — sorted artist names
   *
   * Example:
   *   const artists = await api.listArtists();
   */
  async listArtists(): Promise<string[]> {
    return this.getJSON<string[]>("/api/list/artists");
  }

  /** listTitles returns all distinct recording titles in the collection.
   *
   * Returns:
   *   Promise<string[]> — sorted titles
   *
   * Example:
   *   const titles = await api.listTitles();
   */
  async listTitles(): Promise<string[]> {
    return this.getJSON<string[]>("/api/list/titles");
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
