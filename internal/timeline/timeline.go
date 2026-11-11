package timeline

import (
	"fmt"
	"sort"

	"maket/internal/scene"
)

type Timeline struct {
	Phrases  []PhraseTime
	Duration float64
}

type PhraseTime struct {
	Start, End float64
	Words      []WordTime
}

type WordTime struct {
	Text string
	S, E float64
}

type Gap struct {
	From, To float64
}

const (
	lead          = 0.2
	charsPerSec   = 14.0
	phraseGap     = 0.35
	quietLimitSec = 3.0
)

func Estimate(sc *scene.Scene) *Timeline {
	var phrases []PhraseTime
	t := lead
	for _, p := range sc.Phrases {
		t += p.Pause
		pt := PhraseTime{Start: t}
		for _, w := range scene.Words(p.Text) {
			d := float64(len([]rune(w))+2) / charsPerSec
			pt.Words = append(pt.Words, WordTime{Text: w, S: t, E: t + d})
			t += d
		}
		pt.End = t
		phrases = append(phrases, pt)
		t += phraseGap
	}
	return New(sc, phrases)
}

func New(sc *scene.Scene, phrases []PhraseTime) *Timeline {
	end := 0.0
	if n := len(phrases); n > 0 {
		end = phrases[n-1].End
	}
	return &Timeline{Phrases: phrases, Duration: end + sc.Hold}
}

func (tl *Timeline) Resolve(a scene.At) (float64, error) {
	if a.T != nil {
		return *a.T + a.Offset, nil
	}
	if a.Phrase < 1 || a.Phrase > len(tl.Phrases) {
		return 0, fmt.Errorf("moment %s: no phrase %d", a, a.Phrase)
	}
	p := tl.Phrases[a.Phrase-1]
	if a.Word == "" {
		if a.End {
			return p.End + a.Offset, nil
		}
		return p.Start + a.Offset, nil
	}
	seen := 0
	for _, w := range p.Words {
		if scene.Fold(w.Text) == scene.Fold(a.Word) {
			seen++
			if seen == a.Nth {
				return w.S + a.Offset, nil
			}
		}
	}
	return 0, fmt.Errorf("moment %s: phrase %d has no word %q #%d", a, a.Phrase, a.Word, a.Nth)
}

func (tl *Timeline) MustResolve(a scene.At) float64 {
	t, err := tl.Resolve(a)
	if err != nil {
		panic(err)
	}
	return t
}

func (tl *Timeline) Moments(sc *scene.Scene) ([]float64, error) {
	var out []float64
	add := func(a scene.At) error {
		t, err := tl.Resolve(a)
		if err != nil {
			return err
		}
		out = append(out, t)
		return nil
	}
	for i, ev := range sc.Events {
		if err := add(ev.At); err != nil {
			return nil, fmt.Errorf("event %d: %w", i+1, err)
		}
		if ev.Until != nil {
			if err := add(*ev.Until); err != nil {
				return nil, fmt.Errorf("event %d: %w", i+1, err)
			}
		}
	}
	for _, e := range sc.Entities {
		for _, l := range [][]scene.Timed[string]{e.Roles, e.Status, e.States, e.Badges} {
			for _, x := range l {
				if err := add(x.At); err != nil {
					return nil, fmt.Errorf("entity %s: %w", e.ID, err)
				}
			}
		}
		if e.Appear != nil {
			if err := add(*e.Appear); err != nil {
				return nil, fmt.Errorf("entity %s: %w", e.ID, err)
			}
		}
	}
	if c := sc.Code; c != nil {
		for _, h := range c.Highlight {
			if err := add(h.At); err != nil {
				return nil, fmt.Errorf("code highlight: %w", err)
			}
		}
		for _, l := range c.Lines {
			if l.Appear != nil {
				if err := add(*l.Appear); err != nil {
					return nil, fmt.Errorf("code line: %w", err)
				}
			}
		}
	}
	if a := sc.Answer; a != nil {
		if err := add(a.At); err != nil {
			return nil, fmt.Errorf("answer: %w", err)
		}
		if a.CardAt != nil {
			if err := add(*a.CardAt); err != nil {
				return nil, fmt.Errorf("answer: %w", err)
			}
		}
	}
	sort.Float64s(out)
	return out, nil
}

func (tl *Timeline) Quiet(sc *scene.Scene) []Gap {
	moments, err := tl.Moments(sc)
	if err != nil || len(tl.Phrases) == 0 {
		return nil
	}
	var gaps []Gap
	prev := tl.Phrases[0].Start
	for _, t := range append(moments, tl.Duration) {
		if t-prev > quietLimitSec {
			gaps = append(gaps, Gap{From: prev, To: t})
		}
		if t > prev {
			prev = t
		}
	}
	return gaps
}
