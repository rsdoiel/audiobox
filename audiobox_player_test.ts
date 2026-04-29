import { assertEquals, assertMatch } from "@std/assert";
import { DOMParser } from "deno-dom";

// Pure utility functions — no DOM required
import {
  buildBrowseQuery,
  formatArtists,
  formatDuration,
  PLAYER_TEMPLATE,
} from "./audiobox_player.ts";

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

Deno.test("PLAYER_TEMPLATE - has three tab buttons", () => {
  const doc = new DOMParser().parseFromString(
    `<html><body>${PLAYER_TEMPLATE}</body></html>`,
    "text/html",
  )!;
  const tabs = doc.querySelectorAll(".tab");
  assertEquals(tabs.length, 3);
});

Deno.test("PLAYER_TEMPLATE - tab labels are Titles, Albums, Artists", () => {
  const doc = new DOMParser().parseFromString(
    `<html><body>${PLAYER_TEMPLATE}</body></html>`,
    "text/html",
  )!;
  const labels = Array.from(doc.querySelectorAll(".tab")).map((el) =>
    el.textContent?.trim()
  );
  assertEquals(labels, ["Titles", "Albums", "Artists"]);
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

Deno.test("PLAYER_TEMPLATE - now-playing panel starts hidden", () => {
  const doc = new DOMParser().parseFromString(
    `<html><body>${PLAYER_TEMPLATE}</body></html>`,
    "text/html",
  )!;
  const np = doc.querySelector(".now-playing");
  assertMatch(np?.className ?? "", /hidden/);
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
