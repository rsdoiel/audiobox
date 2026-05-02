//go:build !cgo

// audio_nocgo.go — stub AudioEngine for builds without CGO (e.g. cross-compilation).
// Real audio playback requires CGO via the beep/oto libraries.
// Copyright (C) 2025 R. S. Doiel
package audiobox

import (
	"fmt"
	"time"
)

/** AudioEngine is a stub that satisfies the AudioEngine API on platforms or
 * build configurations where CGO is unavailable.  All methods that require
 * actual audio hardware return a "not available" error.
 *
 * Example:
 *   e := &AudioEngine{}
 *   err := e.Play("/path/to/file.mp3") // returns errNoCGO
 */
type AudioEngine struct{}

var errNoCGO = fmt.Errorf("audio playback requires CGO; rebuild with CGO_ENABLED=1")

/** Play attempts to start playback of the given file.
 *
 * Parameters:
 *   path (string) — path to the audio file (unused in stub)
 *
 * Returns:
 *   error — always returns a "not available" error in this stub
 *
 * Example:
 *   err := engine.Play("/path/to/file.mp3")
 */
func (e *AudioEngine) Play(_ string) error { return errNoCGO }

/** Toggle pauses or resumes playback.
 *
 * Returns:
 *   error — always returns a "not available" error in this stub
 *
 * Example:
 *   err := engine.Toggle()
 */
func (e *AudioEngine) Toggle() error { return errNoCGO }

/** SetVolume sets the playback volume (0–100).
 *
 * Parameters:
 *   pct (int) — volume percentage (unused in stub)
 *
 * Example:
 *   engine.SetVolume(75)
 */
func (e *AudioEngine) SetVolume(_ int) {}

/** Position returns the current elapsed and total playback duration.
 *
 * Returns:
 *   elapsed (time.Duration) — always zero in this stub
 *   total   (time.Duration) — always zero in this stub
 *
 * Example:
 *   elapsed, total := engine.Position()
 */
func (e *AudioEngine) Position() (time.Duration, time.Duration) { return 0, 0 }

/** Done returns a channel that is closed when the current track ends.
 * The stub returns a channel that is immediately closed.
 *
 * Returns:
 *   <-chan struct{} — closed channel
 *
 * Example:
 *   <-engine.Done()
 */
func (e *AudioEngine) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

/** Close releases any resources held by the engine (no-op in stub).
 *
 * Example:
 *   engine.Close()
 */
func (e *AudioEngine) Close() {}
