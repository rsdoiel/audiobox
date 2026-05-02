//go:build cgo

// player.go — terminal UI audio player built on termlib and AudioEngine.
// Copyright (C) 2025 R. S. Doiel
package audiobox

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rsdoiel/termlib"
)

// viewMode controls what the browse panel displays.
type viewMode int

const (
	viewBrowse      viewMode = iota // browsing albums / artists / titles
	viewSearchInput                 // user is typing a search query
	viewSearchResult                // search results are displayed
)

// tabMode selects which browse category is active.
type tabMode int

const (
	tabAlbums tabMode = iota
	tabArtists
	tabTitles
)

// panelFocus tracks which panel currently receives keyboard navigation.
type panelFocus int

const (
	focusBrowse panelFocus = iota
	focusQueue
)

// playerLayout holds computed row/column positions for the current terminal size.
type playerLayout struct {
	width     int
	height    int
	browseTop int // first content row of the browse list
	browseBot int // last content row of the browse list
	sep1      int // separator between browse and now-playing
	nowRow1   int // track title
	nowRow2   int // artist · album
	nowRow3   int // progress bar
	nowRow4   int // controls + volume
	sep2      int // separator between now-playing and queue
	queueTop  int // first row of queue (header)
	queueBot  int // last row of queue
	hintsRow  int // key hints / search input (bottom row)
}

// playerState holds all mutable TUI state.
type playerState struct {
	view              viewMode
	tab               tabMode
	browseList        []string    // items shown in browse panel
	browseAlbums      []Album     // parallel to browseList when tab == tabAlbums
	browseIdx         int         // cursor position in browseList
	browseOff         int         // scroll offset
	visibleBrowseRows int         // updated each draw
	results           []AudioInfo // tracks from last search
	queue             []AudioInfo
	queueIdx          int        // index of the currently playing track
	queueCursor       int        // navigation cursor in queue panel
	queueOff          int        // scroll offset in queue
	visibleQueueRows  int        // updated each draw
	focus             panelFocus // which panel has keyboard focus
	searchQuery       string     // query being typed (viewSearchInput)
	elapsed           time.Duration
	total             time.Duration
	paused            bool
	volume            int    // 0–100
	statusMsg         string // ephemeral one-shot message
}

/** RunPlayer opens a full-screen TUI audio player for the given collection.
 * The terminal is placed in raw mode for the duration. The player returns
 * when the user presses q or Ctrl+C.
 *
 * Parameters:
 *   coll (*Collection) — open collection to browse and play.
 *
 * Returns:
 *   error — non-nil if the audio engine or terminal cannot be initialised.
 *
 * Example:
 *   col, _ := audiobox.LoadCollection("music.yaml")
 *   defer col.Close()
 *   if err := audiobox.RunPlayer(col); err != nil {
 *       log.Fatal(err)
 *   }
 */
func RunPlayer(coll *Collection) error {
	engine, err := NewAudioEngine()
	if err != nil {
		return fmt.Errorf("audio engine: %w", err)
	}
	defer engine.Close()

	restore, err := termlib.EnterRawMode(os.Stdin)
	if err != nil {
		return fmt.Errorf("raw mode: %w", err)
	}
	defer restore()

	term := termlib.New(os.Stdout)
	term.Clear()

	keys := termlib.KeyReader(os.Stdin)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	state := &playerState{
		tab:    tabAlbums,
		volume: engine.Volume(),
	}
	if albums, err := coll.GetAlbumEntries(); err == nil {
		state.browseAlbums = albums
		names := make([]string, len(albums))
		for i, a := range albums {
			names[i] = a.DisplayName
		}
		state.browseList = names
	}

	lay := computeLayout(term)
	drawAll(term, lay, state, coll.Config().Name)

	for {
		select {
		case k, ok := <-keys:
			if !ok {
				return nil
			}
			if handleKey(k, state, engine, coll, lay) {
				term.Clear()
				term.Move(1, 1)
				return nil
			}
			lay = computeLayout(term)
			drawAll(term, lay, state, coll.Config().Name)

		case <-engine.Done():
			if !advanceQueue(state, engine) {
				engine.Idle() // queue exhausted; stop Done() from firing continuously
			}
			lay = computeLayout(term)
			drawAll(term, lay, state, coll.Config().Name)

		case <-ticker.C:
			newElapsed, newTotal := engine.Position()
			newPaused := engine.IsPaused()
			newVolume := engine.Volume()
			// Only redraw when something visible changed. Elapsed is compared at
			// second granularity so a playing track redraws at most once per second.
			if newElapsed.Truncate(time.Second) == state.elapsed.Truncate(time.Second) &&
				newTotal == state.total &&
				newPaused == state.paused &&
				newVolume == state.volume {
				break
			}
			state.elapsed = newElapsed
			state.total = newTotal
			state.paused = newPaused
			state.volume = newVolume
			drawNowPlaying(term, lay, state)
			term.Refresh()
		}
	}
}

// handleKey processes one keystroke. Returns true if the player should exit.
func handleKey(k termlib.Key, state *playerState, engine *AudioEngine, coll *Collection, lay playerLayout) bool {

	// Search input mode intercepts all keys.
	if state.view == viewSearchInput {
		switch k {
		case termlib.Key(0x1b): // Esc — cancel search
			state.view = viewBrowse
			state.searchQuery = ""
		case termlib.Key('\r'), termlib.Key('\n'): // Enter — execute search
			q := strings.TrimSpace(state.searchQuery)
			if q != "" {
				results, _ := coll.SearchAudioFiles(q)
				state.results = results
				state.browseList = audioInfoNames(results)
				state.browseIdx = 0
				state.browseOff = 0
				state.view = viewSearchResult
			} else {
				state.view = viewBrowse
			}
			state.searchQuery = ""
		case termlib.Key(0x7f), termlib.Key(0x08): // Backspace
			if r := []rune(state.searchQuery); len(r) > 0 {
				state.searchQuery = string(r[:len(r)-1])
			}
		default:
			if k >= 32 && k < 127 { // printable ASCII
				state.searchQuery += string(rune(k))
			}
		}
		return false
	}

	// Normal mode.
	switch k {
	case termlib.Key('q'), termlib.Key('Q'), termlib.Key(0x03): // quit
		return true

	case termlib.Key(' '): // play/pause
		engine.Toggle()
		state.paused = engine.IsPaused()

	case termlib.Key('n'): // next
		advanceQueue(state, engine)

	case termlib.Key('p'): // previous
		retreatQueue(state, engine)

	case termlib.Key('+'), termlib.Key('='): // volume up
		engine.SetVolume(engine.Volume() + 5)
		state.volume = engine.Volume()

	case termlib.Key('-'): // volume down
		engine.SetVolume(engine.Volume() - 5)
		state.volume = engine.Volume()

	case termlib.Key('\t'): // Tab — toggle panel focus
		if state.focus == focusBrowse {
			state.focus = focusQueue
		} else {
			state.focus = focusBrowse
		}

	case termlib.KeyLeft: // cycle browse tabs backward (browse panel only)
		if state.focus == focusBrowse {
			cycleBrowseTab(state, coll, -1)
		}

	case termlib.KeyRight: // cycle browse tabs forward (browse panel only)
		if state.focus == focusBrowse {
			cycleBrowseTab(state, coll, +1)
		}

	case termlib.Key('/'): // enter search input mode
		state.view = viewSearchInput
		state.searchQuery = ""

	case termlib.Key(0x1b): // Esc — return to browse
		state.view = viewBrowse
		state.browseIdx = 0
		state.browseOff = 0
		reloadBrowseTab(state, coll)

	case termlib.KeyUp:
		switch state.focus {
		case focusBrowse:
			if state.browseIdx > 0 {
				state.browseIdx--
				if state.browseIdx < state.browseOff {
					state.browseOff = state.browseIdx
				}
			}
		case focusQueue:
			if state.queueCursor > 0 {
				state.queueCursor--
				if state.queueCursor < state.queueOff {
					state.queueOff = state.queueCursor
				}
			}
		}

	case termlib.KeyDown:
		switch state.focus {
		case focusBrowse:
			if state.browseIdx < len(state.browseList)-1 {
				state.browseIdx++
				vis := state.visibleBrowseRows
				if vis < 1 {
					vis = 1
				}
				if state.browseIdx >= state.browseOff+vis {
					state.browseOff = state.browseIdx - vis + 1
				}
			}
		case focusQueue:
			if state.queueCursor < len(state.queue)-1 {
				state.queueCursor++
				vis := state.visibleQueueRows
				if vis < 1 {
					vis = 1
				}
				if state.queueCursor >= state.queueOff+vis {
					state.queueOff = state.queueCursor - vis + 1
				}
			}
		}

	case termlib.KeyPageUp:
		switch state.focus {
		case focusBrowse:
			vis := state.visibleBrowseRows
			if vis < 1 {
				vis = 1
			}
			state.browseIdx -= vis
			if state.browseIdx < 0 {
				state.browseIdx = 0
			}
			state.browseOff -= vis
			if state.browseOff < 0 {
				state.browseOff = 0
			}
			if state.browseIdx < state.browseOff {
				state.browseOff = state.browseIdx
			}
		case focusQueue:
			vis := state.visibleQueueRows
			if vis < 1 {
				vis = 1
			}
			state.queueCursor -= vis
			if state.queueCursor < 0 {
				state.queueCursor = 0
			}
			state.queueOff -= vis
			if state.queueOff < 0 {
				state.queueOff = 0
			}
			if state.queueCursor < state.queueOff {
				state.queueOff = state.queueCursor
			}
		}

	case termlib.KeyPageDown:
		switch state.focus {
		case focusBrowse:
			vis := state.visibleBrowseRows
			if vis < 1 {
				vis = 1
			}
			n := len(state.browseList)
			state.browseIdx += vis
			if state.browseIdx >= n {
				state.browseIdx = n - 1
			}
			if state.browseIdx < 0 {
				state.browseIdx = 0
			}
			if state.browseIdx >= state.browseOff+vis {
				state.browseOff = state.browseIdx - vis + 1
			}
		case focusQueue:
			vis := state.visibleQueueRows
			if vis < 1 {
				vis = 1
			}
			n := len(state.queue)
			state.queueCursor += vis
			if state.queueCursor >= n {
				state.queueCursor = n - 1
			}
			if state.queueCursor < 0 {
				state.queueCursor = 0
			}
			if state.queueCursor >= state.queueOff+vis {
				state.queueOff = state.queueCursor - vis + 1
			}
		}

	case termlib.KeyHome:
		switch state.focus {
		case focusBrowse:
			state.browseIdx = 0
			state.browseOff = 0
		case focusQueue:
			state.queueCursor = 0
			state.queueOff = 0
		}

	case termlib.KeyEnd:
		switch state.focus {
		case focusBrowse:
			n := len(state.browseList)
			if n > 0 {
				state.browseIdx = n - 1
			}
			vis := state.visibleBrowseRows
			if vis < 1 {
				vis = 1
			}
			state.browseOff = state.browseIdx - vis + 1
			if state.browseOff < 0 {
				state.browseOff = 0
			}
		case focusQueue:
			n := len(state.queue)
			if n > 0 {
				state.queueCursor = n - 1
			}
			vis := state.visibleQueueRows
			if vis < 1 {
				vis = 1
			}
			state.queueOff = state.queueCursor - vis + 1
			if state.queueOff < 0 {
				state.queueOff = 0
			}
		}

	case termlib.Key('\r'), termlib.Key('\n'): // Enter
		switch state.focus {
		case focusBrowse:
			playSelected(state, engine, coll)
		case focusQueue:
			if len(state.queue) > 0 && state.queueCursor < len(state.queue) {
				state.queueIdx = state.queueCursor
				engine.Play(state.queue[state.queueIdx])
			}
		}

	case termlib.Key('a'): // append selected to queue without starting playback
		appendSelected(state, coll)
		state.statusMsg = "Added to queue"
	}
	return false
}

// computeLayout calculates row positions from the current terminal dimensions.
func computeLayout(term *termlib.Terminal) playerLayout {
	term.UpdateTerminalSize()
	w := term.GetTerminalWidth()
	h := term.GetTerminalHeight()

	queueH := h / 4
	if queueH < 3 {
		queueH = 3
	}
	const nowH = 4
	browseH := h - 1 - nowH - 2 - queueH // 1=tabrow, 2=separators
	if browseH < 2 {
		browseH = 2
	}

	tabRow := 1
	browseTop := tabRow + 1
	browseBot := browseTop + browseH - 1
	sep1 := browseBot + 1
	nowRow1 := sep1 + 1
	nowRow2 := nowRow1 + 1
	nowRow3 := nowRow2 + 1
	nowRow4 := nowRow3 + 1
	sep2 := nowRow4 + 1
	queueTop := sep2 + 1
	queueBot := queueTop + queueH - 1
	hintsRow := h

	return playerLayout{
		width: w, height: h,
		browseTop: browseTop, browseBot: browseBot,
		sep1:    sep1,
		nowRow1: nowRow1, nowRow2: nowRow2, nowRow3: nowRow3, nowRow4: nowRow4,
		sep2:     sep2,
		queueTop: queueTop, queueBot: queueBot,
		hintsRow: hintsRow,
	}
}

// drawAll redraws every section of the screen.
// The cursor stays hidden except in search input mode, where drawHints shows it.
func drawAll(term *termlib.Terminal, lay playerLayout, state *playerState, collName string) {
	term.HideCursor()
	drawTabBar(term, lay, state, collName)
	drawBrowseList(term, lay, state)
	drawSeparator(term, lay.sep1, lay.width)
	drawNowPlaying(term, lay, state)
	drawSeparator(term, lay.sep2, lay.width)
	drawQueue(term, lay, state)
	drawHints(term, lay, state)
	term.Refresh()
}

func drawTabBar(term *termlib.Terminal, lay playerLayout, state *playerState, collName string) {
	term.Move(1, 1)
	term.ClrToEOL()
	term.SetBold()
	term.Print(termlib.Truncate("audiobox — "+collName, lay.width/2))
	term.ResetStyle()

	labels := []string{"Albums", "Artists", "Titles"}
	col := lay.width/2 + 2
	for i, label := range labels {
		term.Move(1, col)
		if tabMode(i) == state.tab {
			term.SetFgColor(termlib.Black)
			term.SetBgColor(termlib.CyanBg)
		}
		term.Print("[" + label + "]")
		term.ResetStyle()
		col += len(label) + 3
	}
}

func drawBrowseList(term *termlib.Terminal, lay playerLayout, state *playerState) {
	visible := lay.browseBot - lay.browseTop + 1
	state.visibleBrowseRows = visible
	focused := state.focus == focusBrowse
	for i := 0; i < visible; i++ {
		row := lay.browseTop + i
		term.Move(row, 1)
		term.ClrToEOL()
		idx := state.browseOff + i
		if idx >= len(state.browseList) {
			continue
		}
		label := termlib.PadRight(state.browseList[idx], lay.width-2)
		isCursor := idx == state.browseIdx
		switch {
		case isCursor && focused:
			term.SetFgColor(termlib.Black)
			term.SetBgColor(termlib.CyanBg)
			term.Print(" " + label)
			term.ResetStyle()
		case isCursor:
			// dim indicator when panel focus is elsewhere
			term.SetFgColor(termlib.Cyan)
			term.Print(" " + label)
			term.ResetStyle()
		default:
			term.Print(" " + label)
		}
	}
}

func drawSeparator(term *termlib.Terminal, row, width int) {
	term.Move(row, 1)
	term.Print(strings.Repeat("─", width))
}

func drawNowPlaying(term *termlib.Terminal, lay playerLayout, state *playerState) {
	w := lay.width

	// Row 1: play/pause icon + track title
	term.Move(lay.nowRow1, 1)
	term.ClrToEOL()
	title := "(nothing playing)"
	if len(state.queue) > 0 && state.queueIdx < len(state.queue) {
		title = state.queue[state.queueIdx].Name
	}
	icon := "▶ "
	if state.paused {
		icon = "⏸ "
	}
	term.SetBold()
	term.Print(termlib.Truncate(icon+title, w-1))
	term.ResetStyle()

	// Row 2: artist · album
	term.Move(lay.nowRow2, 1)
	term.ClrToEOL()
	if len(state.queue) > 0 && state.queueIdx < len(state.queue) {
		track := state.queue[state.queueIdx]
		artist := ""
		if len(track.ByArtist) > 0 {
			artist = track.ByArtist[0].Name
		}
		term.SetFgColor(termlib.Cyan)
		term.Print(" " + termlib.Truncate(artist+" · "+track.InAlbum, w-2))
		term.ResetStyle()
	}

	// Row 3: elapsed [bar] total
	term.Move(lay.nowRow3, 1)
	term.ClrToEOL()
	elStr := termlib.FormatDuration(state.elapsed)
	totStr := termlib.FormatDuration(state.total)
	barW := w - len(elStr) - len(totStr) - 3
	if barW < 4 {
		barW = 4
	}
	term.Print(elStr + " ")
	termlib.DrawProgressBar(term, lay.nowRow3, len(elStr)+2, barW,
		state.elapsed.Seconds(), state.total.Seconds())
	term.Move(lay.nowRow3, len(elStr)+2+barW)
	term.Print(" " + totStr)

	// Row 4: controls + volume bar
	term.Move(lay.nowRow4, 1)
	term.ClrToEOL()
	volBarW := 10
	volStr := fmt.Sprintf(" ⏮ %s ⏭   Vol:", pauseIcon(state.paused))
	term.Print(volStr)
	col := len(volStr) + 1
	termlib.DrawProgressBar(term, lay.nowRow4, col, volBarW,
		float64(state.volume), 100)
	term.Move(lay.nowRow4, col+volBarW)
	term.Print(fmt.Sprintf(" %d%%", state.volume))
}

func drawQueue(term *termlib.Terminal, lay playerLayout, state *playerState) {
	focused := state.focus == focusQueue
	term.Move(lay.queueTop, 1)
	term.ClrToEOL()
	term.SetBold()
	if focused {
		term.SetFgColor(termlib.Black)
		term.SetBgColor(termlib.CyanBg)
	}
	term.Print(fmt.Sprintf("Queue (%d tracks)", len(state.queue)))
	term.ResetStyle()

	visible := lay.queueBot - lay.queueTop // header takes one row
	state.visibleQueueRows = visible
	for i := 0; i < visible; i++ {
		row := lay.queueTop + 1 + i
		term.Move(row, 1)
		term.ClrToEOL()
		idx := state.queueOff + i
		if idx >= len(state.queue) {
			continue
		}
		track := state.queue[idx]
		isPlaying := idx == state.queueIdx
		isCursor := focused && idx == state.queueCursor
		prefix := "  "
		if isPlaying {
			prefix = "▶ "
		}
		label := termlib.PadRight(fmt.Sprintf("%d. %s", idx+1, track.Name), lay.width-4)
		switch {
		case isCursor:
			term.SetFgColor(termlib.Black)
			term.SetBgColor(termlib.CyanBg)
		case isPlaying:
			term.SetFgColor(termlib.Cyan)
		}
		term.Print(prefix + label)
		if isCursor || isPlaying {
			term.ResetStyle()
		}
	}
}

func drawHints(term *termlib.Terminal, lay playerLayout, state *playerState) {
	term.Move(lay.hintsRow, 1)
	term.ClrToEOL()

	if state.view == viewSearchInput {
		term.SetFgColor(termlib.Yellow)
		term.Print("Search: " + state.searchQuery)
		term.ResetStyle()
		term.ShowCursor()
		return
	}
	if state.statusMsg != "" {
		term.SetFgColor(termlib.Green)
		term.Print(state.statusMsg)
		term.ResetStyle()
		state.statusMsg = ""
		return
	}
	term.SetFgColor(termlib.Cyan)
	switch state.focus {
	case focusBrowse:
		term.Print("[↑↓] move  [PgUp/Dn] page  [Home/End]  [←→] tabs  [Enter] play  [a] add  [Tab]→queue  [/] search  [q] quit")
	case focusQueue:
		term.Print("[↑↓] move  [PgUp/Dn] page  [Home/End]  [Enter] jump  [Tab]→browse  [Spc] pause  [n/p] next/prev  [q] quit")
	}
	term.ResetStyle()
}

// advanceQueue moves to the next track and starts playing it.
// Returns true if a new track was started, false if the queue is exhausted.
func advanceQueue(state *playerState, engine *AudioEngine) bool {
	if len(state.queue) == 0 {
		return false
	}
	next := state.queueIdx + 1
	if next >= len(state.queue) {
		return false
	}
	state.queueIdx = next
	scrollQueueToVisible(state)
	engine.Play(state.queue[state.queueIdx]) //nolint — playback errors shown by beep
	return true
}

// retreatQueue moves to the previous track and starts playing it.
func retreatQueue(state *playerState, engine *AudioEngine) {
	if state.queueIdx <= 0 {
		return
	}
	state.queueIdx--
	scrollQueueToVisible(state)
	engine.Play(state.queue[state.queueIdx])
}

// playSelected replaces the queue with tracks for the highlighted browse item
// and starts playback.
func playSelected(state *playerState, engine *AudioEngine, coll *Collection) {
	if len(state.browseList) == 0 || state.browseIdx >= len(state.browseList) {
		return
	}
	if state.view == viewSearchResult {
		if state.browseIdx < len(state.results) {
			state.queue = []AudioInfo{state.results[state.browseIdx]}
			state.queueIdx = 0
			state.queueCursor = 0
			state.queueOff = 0
			engine.Play(state.queue[0])
		}
		return
	}
	results := albumTracksOrSearch(state, coll)
	if len(results) == 0 {
		return
	}
	state.queue = results
	state.queueIdx = 0
	state.queueCursor = 0
	state.queueOff = 0
	engine.Play(state.queue[0])
}

// appendSelected appends tracks for the highlighted item to the queue.
func appendSelected(state *playerState, coll *Collection) {
	if len(state.browseList) == 0 || state.browseIdx >= len(state.browseList) {
		return
	}
	if state.view == viewSearchResult && state.browseIdx < len(state.results) {
		state.queue = append(state.queue, state.results[state.browseIdx])
		return
	}
	results := albumTracksOrSearch(state, coll)
	state.queue = append(state.queue, results...)
}

// albumTracksOrSearch fetches tracks for the highlighted browse item.
// For the Albums tab it uses GetTracksByAlbum (directory-scoped) so that
// same-named releases in different folders stay independent.
// For other tabs it falls back to SearchAudioFiles.
func albumTracksOrSearch(state *playerState, coll *Collection) []AudioInfo {
	if state.tab == tabAlbums && state.browseIdx < len(state.browseAlbums) {
		tracks, err := coll.GetTracksByAlbum(state.browseAlbums[state.browseIdx])
		if err != nil {
			return nil
		}
		return tracks
	}
	results, err := coll.SearchAudioFiles(browseQuery(state))
	if err != nil {
		return nil
	}
	return results
}

// browseQuery builds a search query for the currently highlighted browse item.
func browseQuery(state *playerState) string {
	if state.browseIdx >= len(state.browseList) {
		return ""
	}
	name := state.browseList[state.browseIdx]
	switch state.tab {
	case tabAlbums:
		return `album:"` + name + `"`
	case tabArtists:
		return `artist:"` + name + `"`
	default:
		return `"` + name + `"`
	}
}

// cycleBrowseTab advances the browse tab by delta (+1 or -1) and loads its list.
func cycleBrowseTab(state *playerState, coll *Collection, delta int) {
	state.tab = tabMode((int(state.tab) + delta + 3) % 3)
	state.browseIdx = 0
	state.browseOff = 0
	state.view = viewBrowse
	reloadBrowseTab(state, coll)
}

// reloadBrowseTab refreshes browseList for the current tab without changing it.
func reloadBrowseTab(state *playerState, coll *Collection) {
	switch state.tab {
	case tabAlbums:
		state.browseAlbums = nil
		if albums, err := coll.GetAlbumEntries(); err == nil {
			state.browseAlbums = albums
			names := make([]string, len(albums))
			for i, a := range albums {
				names[i] = a.DisplayName
			}
			state.browseList = names
		}
	case tabArtists:
		state.browseAlbums = nil
		state.browseList, _ = coll.GetArtists()
	case tabTitles:
		state.browseAlbums = nil
		state.browseList, _ = coll.GetTitles()
	}
}

// scrollQueueToVisible adjusts queueOff so that queueIdx (the playing track) is in view.
func scrollQueueToVisible(state *playerState) {
	vis := state.visibleQueueRows
	if vis < 1 {
		vis = 1
	}
	if state.queueIdx < state.queueOff {
		state.queueOff = state.queueIdx
	}
	if state.queueIdx >= state.queueOff+vis {
		state.queueOff = state.queueIdx - vis + 1
	}
}

// audioInfoNames returns the Name field of each AudioInfo.
func audioInfoNames(tracks []AudioInfo) []string {
	names := make([]string, len(tracks))
	for i, t := range tracks {
		names[i] = t.Name
	}
	return names
}

func pauseIcon(paused bool) string {
	if paused {
		return "⏸"
	}
	return "▶"
}
