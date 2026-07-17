import { assertEquals } from "@std/assert";
import { DOMParser } from "deno-dom";

// Pure utility functions — no DOM required
import {
  buildBrowseQuery,
  buildFolderTree,
  dirOfContentURL,
  formatArtists,
  formatDuration,
  isFieldScopedQuery,
  jumpLetter,
  jumpLetters,
  librarianSortKey,
  literalJumpLetter,
  parseDurationSecs,
  PLAYER_TEMPLATE,
  renderJumpBar,
  shuffleQueue,
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
// buildFolderTree
// ---------------------------------------------------------------------------

Deno.test("buildFolderTree - flat single-level folders", () => {
  const tree = buildFolderTree([
    { path: "Travels", name: "Travels", trackCount: 20 },
    { path: "Peace-Love-Ukulele", name: "Peace-Love-Ukulele", trackCount: 12 },
  ]);
  assertEquals(tree.map((n) => n.path).sort(), ["Peace-Love-Ukulele", "Travels"]);
  const travels = tree.find((n) => n.path === "Travels")!;
  assertEquals(travels.depth, 0);
  assertEquals(travels.ownCount, 20);
  assertEquals(travels.totalCount, 20);
  assertEquals(travels.hasChildren, false);
});

Deno.test("buildFolderTree - reproduces a folder nested 3+ levels deep (bug: could not be drilled into)", () => {
  // Music/Albums/SomeArtist/SomeAlbum/SomeSubfolder holds tracks directly;
  // every ancestor must get its own navigable node, not just the first two levels.
  const tree = buildFolderTree([
    { path: "Music/Albums/SomeArtist/SomeAlbum/SomeSubfolder", name: "SomeSubfolder", trackCount: 5 },
  ]);
  const paths = tree.map((n) => n.path).sort();
  assertEquals(paths, [
    "Music",
    "Music/Albums",
    "Music/Albums/SomeArtist",
    "Music/Albums/SomeArtist/SomeAlbum",
    "Music/Albums/SomeArtist/SomeAlbum/SomeSubfolder",
  ]);

  const leaf = tree.find((n) => n.path === "Music/Albums/SomeArtist/SomeAlbum/SomeSubfolder")!;
  assertEquals(leaf.depth, 4);
  assertEquals(leaf.ownCount, 5);
  assertEquals(leaf.totalCount, 5);
  assertEquals(leaf.hasChildren, false);

  const midAncestor = tree.find((n) => n.path === "Music/Albums/SomeArtist")!;
  assertEquals(midAncestor.depth, 2);
  assertEquals(midAncestor.ownCount, 0, "an ancestor with no tracks of its own has ownCount 0");
  assertEquals(midAncestor.totalCount, 5, "ancestors aggregate descendant track counts");
  assertEquals(midAncestor.hasChildren, true);

  const root = tree.find((n) => n.path === "Music")!;
  assertEquals(root.depth, 0);
  assertEquals(root.name, "Music");
});

Deno.test("buildFolderTree - a directory with tracks of its own AND subfolders sums both into totalCount", () => {
  const tree = buildFolderTree([
    { path: "Jazz/MilesDavis", name: "MilesDavis", trackCount: 3 },
    { path: "Jazz/MilesDavis/Live", name: "Live", trackCount: 7 },
  ]);
  const milesDavis = tree.find((n) => n.path === "Jazz/MilesDavis")!;
  assertEquals(milesDavis.ownCount, 3);
  assertEquals(milesDavis.totalCount, 10);
  assertEquals(milesDavis.hasChildren, true);
});

Deno.test("buildFolderTree - sibling directories sharing a name prefix stay distinct", () => {
  const tree = buildFolderTree([
    { path: "Travels", name: "Travels", trackCount: 20 },
    { path: "Travels with Jack", name: "Travels with Jack", trackCount: 16 },
  ]);
  const travels = tree.find((n) => n.path === "Travels")!;
  const travelsWithJack = tree.find((n) => n.path === "Travels with Jack")!;
  assertEquals(travels.totalCount, 20);
  assertEquals(travelsWithJack.totalCount, 16);
});

// ---------------------------------------------------------------------------
// isFieldScopedQuery / dirOfContentURL — support artist-filtered Albums-tab
// search (typing "Shimabukuro" while browsing Albums should filter the
// album list to his albums, without needing an explicit artist: prefix).
// ---------------------------------------------------------------------------

Deno.test("isFieldScopedQuery - plain artist name is not field-scoped", () => {
  assertEquals(isFieldScopedQuery("Shimabukuro"), false);
});

Deno.test("isFieldScopedQuery - explicit artist: prefix is field-scoped", () => {
  assertEquals(isFieldScopedQuery("artist:Shimabukuro"), true);
});

Deno.test("isFieldScopedQuery - explicit album: prefix is field-scoped", () => {
  assertEquals(isFieldScopedQuery("album:Travel"), true);
});

Deno.test("isFieldScopedQuery - leading whitespace is tolerated", () => {
  assertEquals(isFieldScopedQuery("  title:Goldberg"), true);
});

Deno.test("isFieldScopedQuery - unknown alias is treated as plain text (matches server parseQuery)", () => {
  assertEquals(isFieldScopedQuery("unknownfield:foo"), false);
});

Deno.test("dirOfContentURL - single-level album path", () => {
  assertEquals(dirOfContentURL("Travels/01-departure.wav"), "Travels");
});

Deno.test("dirOfContentURL - nested album path", () => {
  assertEquals(dirOfContentURL("Jazz/MilesDavis/Live/track.mp3"), "Jazz/MilesDavis/Live");
});

Deno.test("dirOfContentURL - root-level file has no directory", () => {
  assertEquals(dirOfContentURL("track.mp3"), "");
});

Deno.test("dirOfContentURL - normalizes Windows-style separators", () => {
  assertEquals(dirOfContentURL("Some\\Windows\\Path\\track.mp3"), "Some/Windows/Path");
});

// ---------------------------------------------------------------------------
// librarianSortKey / jumpLetter / renderJumpBar — A-Z jump bar support.
// Must mirror audiobox.go's librarianSortKey exactly so client-side letter
// bucketing lines up with the server-sorted list order.
// ---------------------------------------------------------------------------

Deno.test("librarianSortKey - strips a leading standalone article", () => {
  assertEquals(librarianSortKey("The Dave Matthews Band"), "dave matthews band");
  assertEquals(librarianSortKey("the dave matthews band"), "dave matthews band");
  assertEquals(librarianSortKey("A Perfect Circle"), "perfect circle");
  assertEquals(librarianSortKey("An Cafe"), "cafe");
});

Deno.test("librarianSortKey - leaves non-article words starting with the same letters alone", () => {
  assertEquals(librarianSortKey("Theatre of Tragedy"), "theatre of tragedy");
  assertEquals(librarianSortKey("Anaconda"), "anaconda");
  assertEquals(librarianSortKey("A-ha"), "a-ha");
});

Deno.test("librarianSortKey - an article with nothing after it is left as-is", () => {
  assertEquals(librarianSortKey("The"), "the");
  assertEquals(librarianSortKey("A"), "a");
});

Deno.test("jumpLetter - buckets by the librarian sort key's first letter", () => {
  assertEquals(jumpLetter("The Dave Matthews Band"), "D");
  assertEquals(jumpLetter("Eagles"), "E");
  assertEquals(jumpLetter("eagles"), "E");
});

Deno.test("jumpLetter - non-alphabetic sort keys bucket under #", () => {
  assertEquals(jumpLetter("3 Doors Down"), "#");
  assertEquals(jumpLetter(""), "#");
});

Deno.test("literalJumpLetter - uses the name's first letter as-written, articles included", () => {
  assertEquals(literalJumpLetter("The Dave Matthews Band"), "T");
  assertEquals(literalJumpLetter("Eagles"), "E");
  assertEquals(literalJumpLetter("3 Doors Down"), "#");
});

Deno.test("jumpLetters - returns both the sorted and literal letters when they differ", () => {
  assertEquals(jumpLetters("The Dave Matthews Band"), ["D", "T"]);
});

Deno.test("jumpLetters - returns a single letter when sorted and literal agree", () => {
  assertEquals(jumpLetters("Eagles"), ["E"]);
  assertEquals(jumpLetters("3 Doors Down"), ["#"]);
});

Deno.test("renderJumpBar - enables only letters present in the given set", () => {
  const doc = new DOMParser().parseFromString(
    `<html><body>${renderJumpBar(new Set(["D", "E"]))}</body></html>`,
    "text/html",
  )!;
  const d = doc.querySelector('[data-jump-to="D"]');
  const e = doc.querySelector('[data-jump-to="E"]');
  const a = doc.querySelector('[data-jump-to="A"]');
  const hash = doc.querySelector('[data-jump-to="#"]');
  assertEquals(d?.hasAttribute("disabled"), false);
  assertEquals(e?.hasAttribute("disabled"), false);
  assertEquals(a?.hasAttribute("disabled"), true);
  assertEquals(hash?.hasAttribute("disabled"), true);
});

// ---------------------------------------------------------------------------
// shuffleQueue — TODO.md: shuffle must never move the currently-playing
// track or anything already played. But "currently playing" means playback
// has actually started (preserveCurrent) — a queue that's freshly built and
// primed to index 0 but never actually played must be fully shuffleable
// (including whatever sits at index 0), so building a queue and shuffling
// it doesn't always leave the same track stuck at the front.
// ---------------------------------------------------------------------------

Deno.test("shuffleQueue - preserveCurrent=true preserves the current track and everything already played", () => {
  const queue = ["a", "b", "c", "d", "e"];
  // currentIndex 1 ("b") — "a" and "b" must never move.
  const result = shuffleQueue(queue, 1, true);
  assertEquals(result[0], "a");
  assertEquals(result[1], "b");
  // The remaining tail must be exactly the same set of tracks, just reordered.
  assertEquals([...result.slice(2)].sort(), ["c", "d", "e"]);
  assertEquals(result.length, 5);
});

Deno.test("shuffleQueue - deterministic permutation with an injected rng", () => {
  const queue = ["x0", "x1", "x2", "x3"];
  // preserveCurrent=false: whole queue is the shuffle pool. rng() => 0 makes
  // every Fisher-Yates swap target index 0, producing a fixed, checkable result.
  const result = shuffleQueue(queue, -1, false, () => 0);
  assertEquals(result, ["x1", "x2", "x3", "x0"]);
});

Deno.test("shuffleQueue - preserveCurrent=false shuffles everything, including whatever sits at currentIndex", () => {
  // This is the "queue was built and primed to index 0 but playback never
  // started" case: the track at index 0 must be just as shuffleable as the
  // rest, not pinned in place the way an actively-playing track would be.
  // Same queue, same currentIndex, same rng — only preserveCurrent differs.
  const queue = ["a", "b", "c"];
  const shuffled = shuffleQueue(queue, 0, false, () => 0);
  const pinned = shuffleQueue(queue, 0, true, () => 0);
  assertEquals(shuffled, ["b", "c", "a"]); // "a" moved away from index 0
  assertEquals(pinned, ["a", "c", "b"]); // "a" stayed pinned at index 0
});

Deno.test("shuffleQueue - empty queue does not crash", () => {
  assertEquals(shuffleQueue([], -1, false), []);
  assertEquals(shuffleQueue([], -1, true), []);
});

Deno.test("shuffleQueue - preserveCurrent=true with current track at the end of the queue is a no-op", () => {
  const queue = ["a", "b", "c"];
  const result = shuffleQueue(queue, 2, true);
  assertEquals(result, ["a", "b", "c"]);
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

Deno.test("PLAYER_TEMPLATE - search input documents field-prefix syntax", () => {
  const doc = new DOMParser().parseFromString(
    `<html><body>${PLAYER_TEMPLATE}</body></html>`,
    "text/html",
  )!;
  const input = doc.querySelector(".search-bar input")!;
  const hint = (input.getAttribute("title") ?? "") + (input.getAttribute("placeholder") ?? "");
  // The hint must mention at least the album:/artist:/title: prefixes so a
  // user isn't left guessing (TODO.md: "do I need a query prefix like
  // artist:Shimabukuro?").
  assertEquals(/album:/i.test(hint), true);
  assertEquals(/artist:/i.test(hint), true);
  assertEquals(/title:/i.test(hint), true);
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

Deno.test("PLAYER_TEMPLATE - has an OPML playlist import control (hidden file input behind a button)", () => {
  const doc = new DOMParser().parseFromString(
    `<html><body>${PLAYER_TEMPLATE}</body></html>`,
    "text/html",
  )!;
  const btn = doc.querySelector(".import-opml-btn");
  const input = doc.querySelector<HTMLInputElement>(".opml-file-input");
  assertEquals(btn !== null, true);
  assertEquals(input !== null, true);
  assertEquals(input?.getAttribute("type"), "file");
  assertEquals(input?.hasAttribute("hidden"), true);
});

Deno.test("PLAYER_TEMPLATE - has a Build Playlist control", () => {
  const doc = new DOMParser().parseFromString(
    `<html><body>${PLAYER_TEMPLATE}</body></html>`,
    "text/html",
  )!;
  const btn = doc.querySelector(".build-playlist-btn");
  const status = doc.querySelector(".build-playlist-status");
  assertEquals(btn !== null, true);
  assertEquals(status !== null, true);
});
