package output

import (
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	startupWord          = "HACKCTL"
	startupCharset       = "abcdefghijklmnopqrstuvwxyz0123456789@#$%&*"
	startupDuration      = 1200 * time.Millisecond
	startupFrameInterval = 50 * time.Millisecond
	startupEncryptRatio  = 0.3
)

type StartupAnimationTickMsg time.Time

type StartupAnimation struct {
	word         string
	duration     time.Duration
	lockDuration time.Duration
	interval     time.Duration
	startedAt    time.Time
	frame        string
	rng          *rand.Rand
	active       bool
}

func NewStartupAnimation() StartupAnimation {
	anim := StartupAnimation{
		word:         startupWord,
		duration:     startupDuration,
		lockDuration: time.Duration(float64(startupDuration) * (1 - startupEncryptRatio)),
		interval:     startupFrameInterval,
		startedAt:    time.Now(),
		active:       shouldAnimateStartup(),
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	if anim.active {
		anim.frame = anim.word
	}

	return anim
}

func (a StartupAnimation) Init() tea.Cmd {
	if !a.active {
		return nil
	}

	return tickStartupAnimation(a.interval)
}

func (a StartupAnimation) Update(msg tea.Msg) (StartupAnimation, tea.Cmd) {
	tick, ok := msg.(StartupAnimationTickMsg)
	if !ok || !a.active {
		return a, nil
	}

	elapsed := time.Time(tick).Sub(a.startedAt)
	if elapsed >= a.duration {
		a.active = false
		a.frame = a.word
		return a, nil
	}

	encryptDuration := a.duration - a.lockDuration
	if elapsed < encryptDuration {
		encrypted := transitioningCharacters(len(a.word), elapsed, encryptDuration, true)
		a.frame = renderEncryptedFrame(a.word, encrypted, startupCharset, a.rng)
	} else {
		decryptElapsed := elapsed - encryptDuration
		locked := lockedCharacters(len(a.word), decryptElapsed, a.lockDuration)
		a.frame = renderDecryptedFrame(a.word, locked, startupCharset, a.rng)
	}

	return a, tickStartupAnimation(a.interval)
}

func (a StartupAnimation) Active() bool {
	return a.active
}

func (a StartupAnimation) Visible() bool {
	return a.frame != ""
}

func (a StartupAnimation) ResolvedView() string {
	return a.word
}

func (a StartupAnimation) View() string {
	return a.frame
}

func tickStartupAnimation(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return StartupAnimationTickMsg(t)
	})
}

func lockedCharacters(length int, elapsed time.Duration, duration time.Duration) int {
	return transitioningCharacters(length, elapsed, duration, false)
}

func transitioningCharacters(length int, elapsed time.Duration, duration time.Duration, roundUp bool) int {
	if length <= 0 {
		return 0
	}

	if duration <= 0 {
		return length
	}

	progress := float64(elapsed) / float64(duration)
	if progress <= 0 {
		return 0
	}

	if progress >= 1 {
		return length
	}

	count := 0
	if roundUp {
		count = int(math.Ceil(progress * float64(length)))
	} else {
		count = int(math.Floor(progress * float64(length)))
	}

	if count < 0 {
		return 0
	}
	if count > length {
		return length
	}

	return count
}

func renderEncryptedFrame(word string, encrypted int, charset string, rng *rand.Rand) string {
	if encrypted < 0 {
		encrypted = 0
	}
	if encrypted > len(word) {
		encrypted = len(word)
	}

	var builder strings.Builder
	builder.Grow(len(word))

	for i := 0; i < encrypted; i++ {
		builder.WriteByte(charset[rng.Intn(len(charset))])
	}

	builder.WriteString(word[encrypted:])

	return builder.String()
}

func renderDecryptedFrame(word string, locked int, charset string, rng *rand.Rand) string {
	if locked < 0 {
		locked = 0
	}
	if locked > len(word) {
		locked = len(word)
	}

	var builder strings.Builder
	builder.Grow(len(word))
	builder.WriteString(word[:locked])

	for i := locked; i < len(word); i++ {
		builder.WriteByte(charset[rng.Intn(len(charset))])
	}

	return builder.String()
}

func shouldAnimateStartup() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	if info.Mode()&os.ModeCharDevice == 0 {
		return false
	}

	return os.Getenv("TERM") != "dumb"
}
