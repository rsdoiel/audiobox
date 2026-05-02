//go:build cgo

// audio.go — beep-based audio engine for the TUI player.
// Copyright (C) 2025 R. S. Doiel
package audiobox

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
)

const audioSampleRate = beep.SampleRate(44100)

/** AudioEngine manages single-track audio playback via the beep library.
 * Formats MP3, FLAC, OGG Vorbis, and WAV are decoded natively. M4A, WMA,
 * and AAC are transcoded through ffmpeg (if available) before playback.
 *
 * Example:
 *   engine, err := audiobox.NewAudioEngine()
 *   if err != nil { log.Fatal(err) }
 *   defer engine.Close()
 *
 *   if err := engine.Play(track); err != nil { log.Println(err) }
 *   <-engine.Done() // block until track ends
 */
type AudioEngine struct {
	mu         sync.Mutex
	ctrl       *beep.Ctrl
	vol        *effects.Volume
	current    beep.StreamSeekCloser // pre-resample streamer for Position/Len
	currentFmt beep.Format
	done       chan struct{} // closed when current track ends naturally
	volumePct  int          // 0–100
}

/** NewAudioEngine initialises the speaker at 44.1 kHz with a 100 ms buffer
 * and returns a ready AudioEngine. Call Close when the player exits.
 *
 * Returns:
 *   *AudioEngine — ready for Play calls.
 *   error        — non-nil if the audio device cannot be opened.
 *
 * Example:
 *   engine, err := audiobox.NewAudioEngine()
 */
func NewAudioEngine() (*AudioEngine, error) {
	bufSize := audioSampleRate.N(time.Second / 10)
	if err := speaker.Init(audioSampleRate, bufSize); err != nil {
		return nil, fmt.Errorf("speaker init: %w", err)
	}
	return &AudioEngine{
		volumePct: 80,
		done:      nil, // nil channel blocks forever; Done() is inert until Play is called
	}, nil
}

/** Play stops any current track and begins playing the audio file referenced
 * by info.ContentURL. The returned error covers file-open and decode
 * failures; playback errors after start are available via Done and Err on
 * the underlying streamer.
 *
 * Parameters:
 *   info (AudioInfo) — track to play; ContentURL must point to a readable file.
 *
 * Returns:
 *   error — non-nil if the file cannot be opened or decoded.
 *
 * Example:
 *   if err := engine.Play(track); err != nil { log.Println(err) }
 */
func (e *AudioEngine) Play(info AudioInfo) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stopCurrentLocked()

	streamer, format, err := openAudio(info.ContentURL)
	if err != nil {
		return err
	}

	e.current = streamer
	e.currentFmt = format
	e.done = make(chan struct{})
	done := e.done // capture for Callback closure

	var src beep.Streamer = streamer
	if format.SampleRate != audioSampleRate {
		src = beep.Resample(4, format.SampleRate, audioSampleRate, streamer)
	}

	e.ctrl = &beep.Ctrl{Streamer: src}
	e.vol = &effects.Volume{
		Streamer: e.ctrl,
		Base:     2,
		Volume:   pctToVolume(e.volumePct),
	}

	speaker.Play(beep.Seq(e.vol, beep.Callback(func() {
		close(done)
	})))

	return nil
}

/** Toggle switches playback between paused and playing.
 *
 * Example:
 *   engine.Toggle() // pause if playing, resume if paused
 */
func (e *AudioEngine) Toggle() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ctrl == nil {
		return
	}
	speaker.Lock()
	e.ctrl.Paused = !e.ctrl.Paused
	speaker.Unlock()
}

/** IsPaused reports whether playback is currently paused. Returns true when
 * no track is loaded.
 *
 * Returns:
 *   bool — true if paused or idle.
 *
 * Example:
 *   if engine.IsPaused() { fmt.Print("⏸") }
 */
func (e *AudioEngine) IsPaused() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ctrl == nil {
		return true
	}
	speaker.Lock()
	p := e.ctrl.Paused
	speaker.Unlock()
	return p
}

/** SetVolume sets the playback volume. pct is clamped to [0, 100].
 * 100 is unity gain; 0 is near-silent (not a hard mute).
 *
 * Parameters:
 *   pct (int) — desired volume, 0–100.
 *
 * Example:
 *   engine.SetVolume(75)
 */
func (e *AudioEngine) SetVolume(pct int) {
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.volumePct = pct
	if e.vol == nil {
		return
	}
	speaker.Lock()
	e.vol.Volume = pctToVolume(pct)
	speaker.Unlock()
}

/** Volume returns the current volume as a percentage in [0, 100].
 *
 * Returns:
 *   int — current volume percentage.
 *
 * Example:
 *   fmt.Printf("Vol: %d%%", engine.Volume())
 */
func (e *AudioEngine) Volume() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.volumePct
}

/** Position returns the elapsed and total duration of the current track.
 * Both values are zero when no track is loaded.
 *
 * Returns:
 *   elapsed (time.Duration) — time played so far.
 *   total   (time.Duration) — full track length.
 *
 * Example:
 *   elapsed, total := engine.Position()
 *   fmt.Printf("%s / %s", termlib.FormatDuration(elapsed), termlib.FormatDuration(total))
 */
func (e *AudioEngine) Position() (elapsed, total time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.current == nil {
		return 0, 0
	}
	speaker.Lock()
	pos := e.current.Position()
	length := e.current.Len()
	speaker.Unlock()
	return e.currentFmt.SampleRate.D(pos), e.currentFmt.SampleRate.D(length)
}

/** Idle resets the engine to a quiescent state where Done() blocks indefinitely.
 * Call this when the queue is exhausted so the event loop stops spinning on
 * the closed done channel left by the last finished track.
 *
 * Example:
 *   if !advanceQueue(state, engine) {
 *       engine.Idle()
 *   }
 */
func (e *AudioEngine) Idle() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.done = nil
}

/** Done returns a channel that is closed when the current track ends
 * naturally. A fresh channel is returned after each Play call, so callers
 * should read Done() again after starting a new track.
 *
 * Returns:
 *   <-chan struct{} — closed on natural track end.
 *
 * Example:
 *   select {
 *   case <-engine.Done():
 *       advanceQueue()
 *   }
 */
func (e *AudioEngine) Done() <-chan struct{} {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.done
}

/** Close stops all playback and releases the audio device. The AudioEngine
 * must not be used after Close returns.
 *
 * Example:
 *   defer engine.Close()
 */
func (e *AudioEngine) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopCurrentLocked()
	speaker.Close()
}

// stopCurrentLocked stops the current track and frees its resources.
// Caller must hold e.mu.
func (e *AudioEngine) stopCurrentLocked() {
	if e.ctrl != nil {
		speaker.Lock()
		e.ctrl.Streamer = nil
		speaker.Unlock()
		e.ctrl = nil
	}
	if e.current != nil {
		e.current.Close()
		e.current = nil
	}
}

// openAudio selects the right decoder for path based on its extension.
func openAudio(path string) (beep.StreamSeekCloser, beep.Format, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3":
		f, err := os.Open(path)
		if err != nil {
			return nil, beep.Format{}, err
		}
		s, fmt, err := mp3.Decode(f)
		if err != nil {
			f.Close()
			return nil, beep.Format{}, err
		}
		return s, fmt, nil

	case ".flac":
		f, err := os.Open(path)
		if err != nil {
			return nil, beep.Format{}, err
		}
		s, fmt, err := flac.Decode(f)
		if err != nil {
			f.Close()
			return nil, beep.Format{}, err
		}
		return s, fmt, nil

	case ".ogg":
		f, err := os.Open(path)
		if err != nil {
			return nil, beep.Format{}, err
		}
		s, fmt, err := vorbis.Decode(f)
		if err != nil {
			f.Close()
			return nil, beep.Format{}, err
		}
		return s, fmt, nil

	case ".wav":
		f, err := os.Open(path)
		if err != nil {
			return nil, beep.Format{}, err
		}
		s, fmt, err := wav.Decode(f)
		if err != nil {
			f.Close()
			return nil, beep.Format{}, err
		}
		return s, fmt, nil

	case ".m4a", ".wma", ".aac":
		return openAudioViaFFmpeg(path)

	default:
		return nil, beep.Format{}, fmt.Errorf("unsupported audio format: %s", ext)
	}
}

// openAudioViaFFmpeg transcodes path to WAV in memory via ffmpeg and returns
// a WAV decoder over the result. Requires ffmpeg to be in PATH.
func openAudioViaFFmpeg(path string) (beep.StreamSeekCloser, beep.Format, error) {
	cmd := exec.Command("ffmpeg", "-i", path, "-f", "wav", "pipe:1", "-loglevel", "quiet")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return nil, beep.Format{}, fmt.Errorf("ffmpeg transcode %s: %w", filepath.Base(path), err)
	}
	s, format, err := wav.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return nil, beep.Format{}, fmt.Errorf("wav decode after ffmpeg: %w", err)
	}
	return s, format, nil
}

// pctToVolume converts a 0–100 volume percentage to the beep Volume exponent.
// 100% → 0 (unity gain, Base^0 = 1). 0% → -4 (near silent, 2^-4 ≈ 0.06).
func pctToVolume(pct int) float64 {
	return float64(pct-100) / 25.0
}
