package ui

import (
	"math"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/harmonica"
)

const animFPS = 60

// frameMsg drives Harmonica animation ticks.
type frameMsg time.Time

func tickAnim() tea.Cmd {
	return tea.Tick(time.Second/animFPS, func(t time.Time) tea.Msg {
		return frameMsg(t)
	})
}

// SpringPair animates a single value toward a target with spring physics.
type SpringPair struct {
	spring   harmonica.Spring
	pos      float64
	velocity float64
	target   float64
}

func newSpring(freq, damping float64) SpringPair {
	return SpringPair{
		spring: harmonica.NewSpring(harmonica.FPS(animFPS), freq, damping),
	}
}

func (s *SpringPair) setTarget(v float64) {
	s.target = v
}

func (s *SpringPair) snap(v float64) {
	s.pos = v
	s.target = v
	s.velocity = 0
}

func (s *SpringPair) update() {
	s.pos, s.velocity = s.spring.Update(s.pos, s.velocity, s.target)
}

func (s *SpringPair) settled() bool {
	return math.Abs(s.pos-s.target) < 0.15 && math.Abs(s.velocity) < 0.15
}

func (s *SpringPair) intPos() int {
	return int(math.Round(s.pos))
}

// pulse returns a 0..1 sine wave for shimmer effects.
func pulse(t time.Time, period time.Duration) float64 {
	if period <= 0 {
		return 0
	}
	phase := float64(t.UnixNano()%int64(period)) / float64(period)
	return 0.5 + 0.5*math.Sin(phase*2*math.Pi)
}
