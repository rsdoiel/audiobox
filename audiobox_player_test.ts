import { assertEquals } from "@std/assert";
import { DOMParser } from "deno-dom";

// Pure utility functions — no DOM required
import {
  buildBrowseQuery,
  formatArtists,
  formatDuration,
  parseDurationSecs,
  PLAYER_TEMPLATE,
} from "./audiobox_player.ts";

// ---------------------------------------------------------------------------
// parseDurationSecs
// ---------------------------------------------------------------------------

Deno.test("parseDurationSecs - empty string returns 0", () => {
  assertEquals(parseDurationSecs(""), 0);
});

Deno.test("parseDurationSecs - invalid string returns 0", () => {
  assertEquals(parseDurationSecs("abc"), 0);
});

Deno.test("parseDurationSecs - PT45S returns 45", () => {
  assertEquals(parseDurationSecs("PT45S"), 45);
});

Deno.test("parseDurationSecs - PT3M45S returns 225", () => {
  assertEquals(parseDurationSecs("PT3M45S"), 225);
});

Deno.test("parseDurationSecs - PT1H2M3S returns 3723", () => {
  assertEquals(parseDurationSecs("PT1H2M3S"), 3723);
});

// ---------------------------------------------------------------------------
// formatArtists
// ---------------------------------------------------------------------------

Deno.test("formatArtists - empty array returns empty string", () => {
  assertEquals(formatArtists([]), "");
});

Deno.test("formatArtists - undefined/null-like coerced to empty string", () => {
  // deno-lint-ignore no-explicit-any
  assertEquals(formatArtists(null as any), "");
});

Deno.test("formatArtists - single artist", () => {
  assertEquals(formatArtists([{ type: "Person", name: "Glenn Gould" }]), "Glenn Gould");
});

Deno.test("formatArtists - multiple artists joined with comma", () => {
  assertEquals(
    formatArtists([
      { type: "Person", name: "Glenn Gould" },
      { type: "Organization", name: "Columbia Records" },
    ]),
    "Glenn Gould, Columbia Records",
  );
});

// ---------------------------------------------------------------------------
// formatDuration
// ---------------------------------------------------------------------------

Deno.test("formatDuration - empty string returns empty string", () => {
  assertEquals(formatDuration(""), "");
});

Deno.test("formatDuration - invalid string returns empty string", () => {
  assertEquals(formatDuration("not-a-duration"), "");
});

Deno.test("formatDuration - PT45S (seconds only)", () => {
  assertEquals(formatDuration("PT45S"), "0:45");
});

Deno.test("formatDuration - PT3M45S (minutes and seconds)", () => {
  assertEquals(formatDuration("PT3M45S"), "3:45");
});

Deno.test("formatDuration - PT10M00S (zero seconds)", () => {
  assertEquals(formatDuration("PT10M00S"), "10:00");
});

Deno.test("formatDuration - PT1H2M3S (hours minutes seconds)", () => {
  assertEquals(formatDuration("PT1H2M3S"), "1:02:03");
});

Deno.test("formatDuration - PT1H0M0S (only hours)", () => {
  assertEquals(formatDuration("PT1H0M0S"), "1:00:00");
});

// ---------------------------------------------------------------------------
// buildBrowseQuery
// ---------------------------------------------------------------------------

Deno.test("buildBrowseQuery - albums tab", () => {
  assertEquals(buildBrowseQuery("albums", "Goldberg Variations"), 'album:"Goldberg Variations"');
});

Deno.test("buildBrowseQuery - artists tab", () => {
  assertEquals(buildBrowseQuery("artists", "Glenn Gould"), 'artist:"Glenn Gould"');
});

Deno.test("buildBrowseQuery - titles tab", () => {
  assertEquals(
    buildBrowseQuery("titles", "Air on a G String"),
    'title:"Air on a G String"',
  );
});

Deno.test("buildBrowseQuery - unknown tab returns item as-is", () => {
  assertEquals(buildBrowseQuery("other", "something"), "something");
});

Deno.test("buildBrowseQuery - escapes double quotes in item", () => {
  assertEquals(buildBrowseQuery("albums", 'Say "Hello"'), 'album:"Say \\"Hello\\""');
});

// ---------------------------------------------------------------------------
// PLAYER_TEMPLATE structure (via deno-dom)
// ---------------------------------------------------------------------------

Deno.test("PLAYER_TEMPLATE - has five tab buttons", () => {
  const doc = new DOMParser().parseFromString(
    `<html><body>${PLAYER_TEMPLATE}</body></html>`,
    "text/html",
  )!;
  const tabs = doc.querySelectorAll(".tab");
  assertEquals(tabs.length, 5);
});

Deno.test("PLAYER_TEMPLATE - tab labels are Albums, Artists, Titles, Folders, Playlists", () => {
  const doc = new DOMParser().parseFromString(
    `<html><body>${PLAYER_TEMPLATE}</body></html>`,
    "text/html",
  )!;
  const labels = Array.from(doc.querySelectorAll(".tab")).map((el) =>
    el.textContent?.trim()
  );
  assertEquals(labels, ["Albums", "Artists", "Titles", "Folders", "Playlists"]);
});

Deno.test("PLAYER_TEMPLATE - has search input and button", () => {
  const doc = new DOMParser().parseFromString(
    `<html><body>${PLAYER_TEMPLATE}</body></html>`,
    "text/html",
  )!;
  const input = doc.querySelector(".search-bar input");
  const btn = doc.querySelector(".search-btn");
  assertEquals(input !== null, true);
  assertEquals(btn !== null, true);
});

Deno.test("PLAYER_TEMPLATE - now-playing panel is always visible", () => {
  const doc = new DOMParser().parseFromString(
    `<html><body>${PLAYER_TEMPLATE}</body></html>`,
    "text/html",
  )!;
  const np = doc.querySelector(".now-playing");
  assertEquals(np !== null, true);
  assertEquals(np?.classList.contains("hidden"), false);
});

Deno.test("PLAYER_TEMPLATE - has prev, play, next control buttons", () => {
  const doc = new DOMParser().parseFromString(
    `<html><body>${PLAYER_TEMPLATE}</body></html>`,
    "text/html",
  )!;
  assertEquals(doc.querySelector(".prev-btn") !== null, true);
  assertEquals(doc.querySelector(".play-pause-btn") !== null, true);
  assertEquals(doc.querySelector(".next-btn") !== null, true);
});

Deno.test("PLAYER_TEMPLATE - has seek bar and volume bar", () => {
  const doc = new DOMParser().parseFromString(
    `<html><body>${PLAYER_TEMPLATE}</body></html>`,
    "text/html",
  )!;
  assertEquals(doc.querySelector(".seek-bar") !== null, true);
  assertEquals(doc.querySelector(".volume-bar") !== null, true);
});

Deno.test("PLAYER_TEMPLATE - has queue panel with hide/show toggle button", () => {
  const doc = new DOMParser().parseFromString(
    `<html><body>${PLAYER_TEMPLATE}</body></html>`,
    "text/html",
  )!;
  const toggle = doc.querySelector(".toggle-queue-btn");
  assertEquals(toggle !== null, true);
  assertEquals(toggle?.textContent?.trim(), "Hide");
});

Deno.test("PLAYER_TEMPLATE - has audio element", () => {
  const doc = new DOMParser().parseFromString(
    `<html><body>${PLAYER_TEMPLATE}</body></html>`,
    "text/html",
  )!;
  assertEquals(doc.querySelector(".audio-el") !== null, true);
});

Deno.test("PLAYER_TEMPLATE - has shuffle button disabled by default", () => {
  const doc = new DOMParser().parseFromString(
    `<html><body>${PLAYER_TEMPLATE}</body></html>`,
    "text/html",
  )!;
  const btn = doc.querySelector<HTMLButtonElement>(".shuffle-btn");
  assertEquals(btn !== null, true);
  assertEquals(btn?.hasAttribute("disabled"), true);
});
