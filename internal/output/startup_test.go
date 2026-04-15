package output

import (
	"math/rand"
	"strings"
	"testing"
	"time"
)

func TestLockedCharacters(t *testing.T) {
	tests := []struct {
		name     string
		length   int
		elapsed  time.Duration
		duration time.Duration
		want     int
	}{
		{name: "empty word", length: 0, elapsed: 500 * time.Millisecond, duration: time.Second, want: 0},
		{name: "before start", length: len(startupWord), elapsed: -100 * time.Millisecond, duration: time.Second, want: 0},
		{name: "halfway", length: len(startupWord), elapsed: 700 * time.Millisecond, duration: 1400 * time.Millisecond, want: 3},
		{name: "complete", length: len(startupWord), elapsed: 2 * time.Second, duration: 1400 * time.Millisecond, want: len(startupWord)},
		{name: "zero duration", length: len(startupWord), elapsed: 0, duration: 0, want: len(startupWord)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lockedCharacters(tt.length, tt.elapsed, tt.duration); got != tt.want {
				t.Fatalf("lockedCharacters() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestTransitioningCharactersRoundUp(t *testing.T) {
	got := transitioningCharacters(len(startupWord), 100*time.Millisecond, 300*time.Millisecond, true)
	if got != 3 {
		t.Fatalf("transitioningCharacters() = %d, want %d", got, 3)
	}
}

func TestRenderDecryptedFrameKeepsLockedPrefix(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	frame := renderDecryptedFrame(startupWord, 4, startupCharset, rng)

	if len(frame) != len(startupWord) {
		t.Fatalf("frame length = %d, want %d", len(frame), len(startupWord))
	}

	if !strings.HasPrefix(frame, startupWord[:4]) {
		t.Fatalf("frame %q does not keep locked prefix %q", frame, startupWord[:4])
	}

	for _, ch := range frame[4:] {
		if !strings.ContainsRune(startupCharset, ch) {
			t.Fatalf("frame %q contains unexpected character %q", frame, ch)
		}
	}
}

func TestRenderEncryptedFrameKeepsOriginalSuffix(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	frame := renderEncryptedFrame(startupWord, 3, startupCharset, rng)

	if len(frame) != len(startupWord) {
		t.Fatalf("frame length = %d, want %d", len(frame), len(startupWord))
	}

	if !strings.HasSuffix(frame, startupWord[3:]) {
		t.Fatalf("frame %q does not keep original suffix %q", frame, startupWord[3:])
	}

	for _, ch := range frame[:3] {
		if !strings.ContainsRune(startupCharset, ch) {
			t.Fatalf("frame %q contains unexpected character %q", frame, ch)
		}
	}
}

func TestStartupAnimationFinishesOnFinalWord(t *testing.T) {
	start := time.Unix(0, 0)
	anim := StartupAnimation{
		word:         startupWord,
		duration:     startupDuration,
		lockDuration: time.Duration(float64(startupDuration) * (1 - startupEncryptRatio)),
		interval:     startupFrameInterval,
		startedAt:    start,
		frame:        startupWord,
		rng:          rand.New(rand.NewSource(2)),
		active:       true,
	}

	anim, cmd := anim.Update(StartupAnimationTickMsg(start.Add(startupDuration)))
	if cmd != nil {
		t.Fatal("expected no follow-up tick command after animation completes")
	}

	if anim.Active() {
		t.Fatal("expected animation to be inactive after completion")
	}

	if anim.View() != startupWord {
		t.Fatalf("final frame = %q, want %q", anim.View(), startupWord)
	}
}
