import { assertEquals, assertRejects } from "@std/assert";
import { AudioInfoAPI, type AlbumEntry, type AudioInfo, type ScanStatus } from "./audiobox_api.ts";

type FetchFn = typeof globalThis.fetch;
const originalFetch: FetchFn = globalThis.fetch;

function mockFetch(data: unknown, status = 200): void {
  globalThis.fetch = () =>
    Promise.resolve(
      new Response(JSON.stringify(data), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
    );
}

function restoreFetch(): void {
  globalThis.fetch = originalFetch;
}

Deno.test("listAlbums - returns AlbumEntry array", async () => {
  const fixture: AlbumEntry[] = [
    { name: "Bach Partitas", displayName: "Bach Partitas", dir: "/music/Bach-Partitas" },
    { name: "Goldberg Variations", displayName: "Goldberg Variations", dir: "/music/Goldberg-Variations" },
  ];
  mockFetch(fixture);
  try {
    const api = new AudioInfoAPI("http://localhost:8010");
    const albums = await api.listAlbums();
    assertEquals(albums.length, 2);
    assertEquals(albums[0].name, "Bach Partitas");
    assertEquals(albums[1].name, "Goldberg Variations");
  } finally {
    restoreFetch();
  }
});

Deno.test("listArtists - returns string array", async () => {
  mockFetch(["Glenn Gould", "Johann Sebastian Bach"]);
  try {
    const api = new AudioInfoAPI();
    assertEquals(await api.listArtists(), ["Glenn Gould", "Johann Sebastian Bach"]);
  } finally {
    restoreFetch();
  }
});

Deno.test("listTitles - returns string array", async () => {
  mockFetch(["Goldberg Variations BWV 988", "Partita No. 1"]);
  try {
    const api = new AudioInfoAPI();
    assertEquals(await api.listTitles(), ["Goldberg Variations BWV 988", "Partita No. 1"]);
  } finally {
    restoreFetch();
  }
});

Deno.test("listAlbumTracks - requests the exact album-tracks endpoint by dir, not search", async () => {
  let capturedUrl = "";
  globalThis.fetch = (input: RequestInfo | URL) => {
    capturedUrl = String(input);
    return Promise.resolve(
      new Response(JSON.stringify([]), { status: 200 }),
    );
  };
  try {
    const api = new AudioInfoAPI("http://localhost:8010");
    await api.listAlbumTracks("Travels");
    assertEquals(
      capturedUrl,
      "http://localhost:8010/api/list/album-tracks?dir=Travels",
    );
  } finally {
    restoreFetch();
  }
});

Deno.test("listAlbumTracks - returns AudioInfo array", async () => {
  const fixture = [{ ID: "abc", Name: "Departure Suite" }];
  mockFetch(fixture);
  try {
    const api = new AudioInfoAPI();
    const results = await api.listAlbumTracks("Travels");
    assertEquals(results.length, 1);
    assertEquals(results[0].Name, "Departure Suite");
  } finally {
    restoreFetch();
  }
});

Deno.test("search - encodes query in URL", async () => {
  let capturedUrl = "";
  globalThis.fetch = (input: RequestInfo | URL) => {
    capturedUrl = String(input);
    return Promise.resolve(
      new Response(JSON.stringify([]), { status: 200 }),
    );
  };
  try {
    const api = new AudioInfoAPI("http://localhost:8010");
    await api.search("artist:Glenn Gould");
    assertEquals(
      capturedUrl,
      "http://localhost:8010/api/search?q=artist%3AGlenn%20Gould",
    );
  } finally {
    restoreFetch();
  }
});

Deno.test("search - returns AudioInfo array", async () => {
  const fixture = [{ ID: "abc", Name: "Goldberg Variations" }];
  mockFetch(fixture);
  try {
    const api = new AudioInfoAPI();
    const results = await api.search("Bach");
    assertEquals(results.length, 1);
    assertEquals(results[0].Name, "Goldberg Variations");
  } finally {
    restoreFetch();
  }
});

Deno.test("show - returns AudioInfo on success", async () => {
  const fixture: Partial<AudioInfo> = {
    ID: "test-uuid",
    Name: "Goldberg Variations",
    InAlbum: "Bach: Goldberg Variations",
  };
  mockFetch(fixture);
  try {
    const api = new AudioInfoAPI();
    const info = await api.show("test-uuid");
    assertEquals(info.ID, "test-uuid");
    assertEquals(info.Name, "Goldberg Variations");
  } finally {
    restoreFetch();
  }
});

Deno.test("show - throws on 404", async () => {
  mockFetch({ error: "record not found" }, 404);
  try {
    const api = new AudioInfoAPI();
    await assertRejects(() => api.show("bad-id"), Error, "record not found");
  } finally {
    restoreFetch();
  }
});

Deno.test("startScan - returns started status on 202", async () => {
  mockFetch({ status: "started", started_at: "2024-01-01T00:00:00Z" }, 202);
  try {
    const api = new AudioInfoAPI();
    const result = await api.startScan();
    assertEquals(result.status, "started");
  } finally {
    restoreFetch();
  }
});

Deno.test("startScan - throws on 409 conflict", async () => {
  mockFetch({ error: "scan already in progress" }, 409);
  try {
    const api = new AudioInfoAPI();
    await assertRejects(() => api.startScan(), Error, "scan already in progress");
  } finally {
    restoreFetch();
  }
});

Deno.test("scanStatus - returns idle status", async () => {
  const fixture: ScanStatus = { status: "idle" };
  mockFetch(fixture);
  try {
    const api = new AudioInfoAPI();
    const s = await api.scanStatus();
    assertEquals(s.status, "idle");
  } finally {
    restoreFetch();
  }
});

Deno.test("scanStatus - returns running with timestamps", async () => {
  const fixture: ScanStatus = { status: "running", started_at: "2024-01-01T00:00:00Z" };
  mockFetch(fixture);
  try {
    const api = new AudioInfoAPI();
    const s = await api.scanStatus();
    assertEquals(s.status, "running");
    assertEquals(s.started_at, "2024-01-01T00:00:00Z");
  } finally {
    restoreFetch();
  }
});

Deno.test("audioUrl - constructs URL without fetch", () => {
  const api = new AudioInfoAPI("http://localhost:8010");
  assertEquals(
    api.audioUrl("550e8400-e29b-41d4-a716-446655440000"),
    "http://localhost:8010/api/audio/550e8400-e29b-41d4-a716-446655440000",
  );
});

Deno.test("audioUrl - strips trailing slash from baseUrl", () => {
  const api = new AudioInfoAPI("http://localhost:8010/");
  assertEquals(
    api.audioUrl("abc"),
    "http://localhost:8010/api/audio/abc",
  );
});

Deno.test("audioUrl - works with empty baseUrl (same-origin)", () => {
  const api = new AudioInfoAPI();
  assertEquals(api.audioUrl("xyz"), "/api/audio/xyz");
});

Deno.test("opmlExportUrl - constructs URL without fetch", () => {
  const api = new AudioInfoAPI("http://localhost:8010");
  assertEquals(
    api.opmlExportUrl("550e8400-e29b-41d4-a716-446655440000"),
    "http://localhost:8010/api/playlists/550e8400-e29b-41d4-a716-446655440000/opml",
  );
});

Deno.test("importPlaylistOPML - posts multipart form with file and optional name", async () => {
  let capturedUrl = "";
  let capturedInit: RequestInit | undefined;
  globalThis.fetch = (input: RequestInfo | URL, init?: RequestInit) => {
    capturedUrl = String(input);
    capturedInit = init;
    return Promise.resolve(
      new Response(
        JSON.stringify({ id: "x", name: "My Mix", trackCount: 1, imported: 1, skipped: 0 }),
        { status: 201 },
      ),
    );
  };
  try {
    const api = new AudioInfoAPI("http://localhost:8010");
    const file = new File(["<opml></opml>"], "playlist.opml", { type: "text/x-opml+xml" });
    const result = await api.importPlaylistOPML(file, "My Mix");
    assertEquals(capturedUrl, "http://localhost:8010/api/playlists/import-opml");
    assertEquals(capturedInit?.method, "POST");
    const form = capturedInit?.body as FormData;
    assertEquals(form.get("file") instanceof File, true);
    assertEquals(form.get("name"), "My Mix");
    assertEquals(result.imported, 1);
    assertEquals(result.name, "My Mix");
  } finally {
    restoreFetch();
  }
});

Deno.test("importPlaylistOPML - omits name field when not provided", async () => {
  let capturedInit: RequestInit | undefined;
  globalThis.fetch = (_input: RequestInfo | URL, init?: RequestInit) => {
    capturedInit = init;
    return Promise.resolve(
      new Response(
        JSON.stringify({ id: "x", name: "Imported Playlist", trackCount: 0, imported: 0, skipped: 0 }),
        { status: 201 },
      ),
    );
  };
  try {
    const api = new AudioInfoAPI();
    const file = new File(["<opml></opml>"], "playlist.opml");
    await api.importPlaylistOPML(file);
    const form = capturedInit?.body as FormData;
    assertEquals(form.get("name"), null);
  } finally {
    restoreFetch();
  }
});

Deno.test("importPlaylistOPML - throws on error response", async () => {
  mockFetch({ error: "bad opml" }, 400);
  try {
    const api = new AudioInfoAPI();
    const file = new File(["not xml"], "bad.opml");
    await assertRejects(() => api.importPlaylistOPML(file), Error, "bad opml");
  } finally {
    restoreFetch();
  }
});
