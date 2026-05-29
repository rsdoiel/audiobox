# audiobox Design Decisions

A log of significant design choices, the alternatives considered, and the
reasons for each decision. Newest entries at the top.

---

## 2026-05-29 — Network sharing: single listener restart, not dual listeners

**Decision:** Share mode is implemented as a single `http.Server` goroutine
that can be stopped and restarted on a different bind address, rather than
running two concurrent listeners on separate ports.

**Alternatives considered:**
- *Two concurrent listeners* — one on `127.0.0.1:port` (full access) and one
  on `0.0.0.0:port+1` (read-only). Rejected because two listeners on
  different ports adds user-visible complexity (two URLs, two port numbers to
  know about) and complicates the shutdown path, which must now drain both
  servers.
- *Mode-based single restart (bind to specific IP)* — bind to the user-chosen
  LAN IP directly rather than `0.0.0.0`. Rejected because binding to a
  specific IP means `127.0.0.1` connections no longer work, which would break
  the owner's existing browser tab and make the loopback-based access control
  check unreliable.

**Rationale:** Binding to `0.0.0.0` on restart covers all interfaces
including loopback, so the owner's `http://127.0.0.1:8010` tab stays valid
throughout. The chosen LAN IP address is used only for display and for saving
to config — not as the actual bind address. The listener restart is fast
(graceful shutdown with a 5 s timeout) and the browser UI polls `127.0.0.1`
during the brief downtime, so the owner experiences no interruption.

---

## 2026-05-29 — Network sharing: IP-based access control, not mode-based

**Decision:** When the server is bound to `0.0.0.0` (share mode), access
control is enforced per-request by checking `r.RemoteAddr`. Connections from
`127.0.0.1` or `::1` receive full access; all other source IPs are restricted
to read-only (GET/HEAD only — scan, sweep, init, delete, shutdown, and share
toggle are blocked with 403).

**Alternatives considered:**
- *Mode-based (everyone read-only in share mode)* — simpler middleware, but
  forces the owner to use the CLI for scan/sweep/shutdown while sharing.
  Rejected as too inconvenient for the common case where the owner wants to
  manage their collection while guests are listening.

**Rationale:** The owner connects via `127.0.0.1` throughout (the browser is
never redirected away from that address), so the loopback check reliably
identifies the owner. LAN guests connect from a routable IP and naturally
receive the read-only view.

---

## 2026-05-29 — Network sharing: no browser redirect on share toggle

**Decision:** When share mode is toggled on/off via the UI, the owner's
browser is not redirected. The UI polls `http://127.0.0.1:8010/api/share/status`
and updates the Share button in place once the listener is back up.

**Alternatives considered:**
- *Redirect owner to LAN IP after enabling share* — would visually confirm
  the LAN URL works, but moves the owner off the loopback address, stripping
  their admin access (they become a read-only LAN client). Rejected.
- *Open LAN URL in a new tab* — avoids stripping admin access but creates
  a confusing second tab. Rejected.

**Rationale:** Since `0.0.0.0` covers loopback, `127.0.0.1:8010` continues
to work in share mode. The Share button shows the LAN URL with a Copy button;
the owner pastes it into a message to share with others. No redirect needed.

---

## 2026-05-29 — Network sharing: CLI syntax `audiobox server share [IP]`

**Decision:** Share mode is activated by passing `share` as a positional
sub-argument to the `server` action: `audiobox server share` or
`audiobox server share 192.168.1.5`.

**Alternatives considered:**
- *Flag: `audiobox server --share [--share-addr 192.168.1.5]`* — idiomatic
  for Go's `flag` package, but `audiobox server share` reads more naturally
  as a subcommand and is consistent with the existing pattern of positional
  action arguments (`audiobox list albums`, `audiobox help server`, etc.).

**Rationale:** Positional style is consistent with the rest of the CLI. The
first run requires an explicit IP address (`audiobox server share 192.168.1.5`),
which writes `shareAddress` to `audio.yaml`. Subsequent runs can omit the
address (`audiobox server share`) and the saved value is used, avoiding the
need to look up the IP each time.

---

## 2026-05-29 — Browse drill-down: clicking a browse item never auto-plays or modifies the queue

**Decision:** Clicking an album, artist, title, or folder row in the browse
panel opens a track detail view in the list panel. The queue is not modified
and playback does not start. Only the explicit `⊕` (add) button on a track
row or the `⊕ Add All` button on the detail view header adds content to the
queue.

**Alternatives considered:**
- *Click to replace queue and auto-play* — the original behaviour. Simple but
  destructive: a mis-click wipes a carefully assembled queue. Rejected.
- *Click to append to queue and auto-play* — less destructive but still
  starts unexpected playback. Rejected.

**Rationale:** Separating "browse" from "queue" gives the user full control.
They build the queue deliberately with `⊕` actions, then press ▶ when ready.
The first `⊕` action also primes the player (`audioEl.src` is set) so the
track name appears in the now-playing panel as a visual cue, without sound
starting.

---

## 2026-05-29 — Queue: played tracks are auto-removed

**Decision:** When a track finishes playing (the `ended` audio event), it is
spliced out of the queue array. The next track shifts into its slot and begins
playing automatically. The queue therefore shows only upcoming tracks (plus
the one currently playing).

**Alternatives considered:**
- *Keep played tracks in queue, advance an index* — gives a playback history
  visible in the queue and allows "Previous" to go back to already-played
  items. Rejected for this phase; the queue was conceived as a FIFO play
  list, not a history. A separate history feature can be added later.

**Rationale:** Matches the mental model of a play queue (not a playlist). The
queue shrinks as content plays, reaching empty when the session is done.
Shuffle and manual removal via `×` buttons still work correctly because they
operate on the remaining items regardless of position.

---

## 2026-05-29 — Player: always visible; delete button removed

**Decision:** The now-playing panel is always rendered, even when no track is
loaded (idle state shows "(No track loaded)" with disabled transport buttons).
The delete-record-from-collection button is removed from the player entirely.

**Alternatives considered:**
- *Hide player until first track loads* — the original behaviour. Causes
  layout shift and makes the UI feel incomplete on first load. Rejected.
- *Keep delete button but move it* — e.g. a right-click context menu on a
  queue item. Possible future work, but not part of this iteration.

**Rationale:** An always-visible player gives the UI a stable layout and
makes the transport controls predictable. The delete function is a library
management operation, not a playback operation; mixing them in the player
created accidental-deletion risk. Record deletion remains available via the
`DELETE /api/show/{id}` API endpoint and CLI.

---

## 2026-05-29 — Search: client-side fan-out for typed results, no new backend endpoint

**Decision:** Grouped search results (Albums matching…, Artists matching…,
Titles matching…) are produced by issuing up to four parallel API calls with
field-scoped queries (`album:"q"`, `artist:"q"`, `title:"q"`, plain `q`)
from the frontend. No new backend search endpoint is added.

**Alternatives considered:**
- *New `GET /api/search/typed` endpoint* — backend returns pre-grouped JSON.
  Cleaner network traffic (one call instead of four) and allows server-side
  deduplication. Deferred: the existing field-scoped query syntax already
  covers the use case, and the frontend fan-out is straightforward with
  `Promise.all`.

**Rationale:** Avoids adding backend surface area in Phase 3. The four
parallel requests complete in roughly the same time as a single request.
The backend endpoint can be added later if the fan-out proves too slow on
large collections.

---

## 2026-05-29 — Playlists stored in SQLite, not as files

**Decision:** Saved playlists are stored in two new SQLite tables
(`playlists`, `playlist_tracks`) inside the existing `audio.db` database.

**Alternatives considered:**
- *M3U or XSPF files on disk* — portable, human-readable, compatible with
  other players. Rejected for now because it requires a file-naming scheme,
  directory conventions, and a parser. Can be added as an export feature
  later.
- *Separate database file* — unnecessary isolation; the main database already
  handles migrations cleanly via `ALTER TABLE IF NOT EXISTS` patterns.

**Rationale:** Keeping playlists in the same database simplifies backup
("copy `audio.db`"), keeps foreign-key integrity with `audio_files`, and
requires no new file I/O code beyond what already exists.

---

## 2026-05-29 — Single SQLite database with FTS5, not a dedicated search index

**Decision:** Full-text search uses SQLite's built-in FTS5 virtual table
(`search_index`) with a Levenshtein fuzzy fallback implemented in Go for
queries that return no FTS5 results.

**Alternatives considered:**
- *Bleve or Tantivy (via CGo)* — dedicated search library with richer ranking.
  Rejected: adds a significant build dependency and CGo requirement for a
  local-collection use case where the corpus rarely exceeds tens of thousands
  of tracks.
- *Pure SQL LIKE queries* — simple but no fuzzy matching, poor performance on
  large collections without full-text indexing.

**Rationale:** FTS5 is bundled with SQLite and requires no external dependency.
The `unicode61 remove_diacritics 1` tokenizer handles accented characters
correctly (e.g. "Meklit" matches "Méklít"). The Levenshtein fallback handles
typos within the thresholds used by OpenSearch AUTO (edit distance 0/1/2 based
on term length).

---

## 2026-05-29 — Embedded htdocs, with override via config

**Decision:** The web UI static files are embedded in the binary via
`go:embed`. A `htdocs` field in `audio.yaml` can override this with a
filesystem path, used during frontend development.

**Rationale:** A single binary with no external file dependencies is the
primary distribution target. The override path allows iterating on the
TypeScript/HTML without rebuilding the Go binary each time.
