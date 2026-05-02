//go:build !cgo

// player_nocgo.go — stub RunPlayer for builds without CGO (e.g. cross-compilation).
// The terminal player depends on AudioEngine which requires CGO via beep/oto.
// Copyright (C) 2025 R. S. Doiel
package audiobox

import "fmt"

/** RunPlayer starts the terminal UI audio player.
 * This stub is compiled when CGO is unavailable and always returns an error
 * directing the user to use the web UI instead.
 *
 * Parameters:
 *   col (*Collection) — the open audio collection (unused in stub)
 *
 * Returns:
 *   error — always returns a "not available" error in this stub
 *
 * Example:
 *   err := RunPlayer(col)
 */
func RunPlayer(_ *Collection) error {
	return fmt.Errorf("the terminal player requires CGO (audio hardware support); use 'audiobox' to start the web UI instead")
}
