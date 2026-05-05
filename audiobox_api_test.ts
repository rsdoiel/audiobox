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
