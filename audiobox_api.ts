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

/** ScanStatus describes the current state of an async collection scan. */
export interface ScanStatus {
  status: "idle" | "running" | "completed" | "error";
  started_at?: string;
  completed_at?: string;
  error?: string;
}

/** ScanStarted is returned when POST /api/scan accepts the request. */
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
}
