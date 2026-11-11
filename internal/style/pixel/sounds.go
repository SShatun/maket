package pixel

import (
	"sort"

	"maket/internal/audio"
	"maket/internal/scene"
	"maket/internal/timeline"
)

func (p *Pixel) sounds(sc *scene.Scene, tl *timeline.Timeline) []audio.Cue {
	var c []audio.Cue
	add := func(t float64, kind string) { c = append(c, audio.Cue{T: t, Kind: kind, Gain: 1}) }
	if sc.Guess != nil {
		add(0.05, audio.ChipTitle)
	}
	for _, x := range p.ents {
		add(x.appear, audio.ChipAppear)
		for _, s := range x.states {
			if s.v == "happy" {
				add(s.t, audio.ChipOK)
			}
		}
		for _, r := range x.roles[1:] {
			switch r.v {
			case "error":
				add(r.t, audio.ChipBlock)
			case "ok":
				add(r.t, audio.ChipOK)
			}
		}
	}
	for _, m := range p.moves {
		add(m.t, audio.ChipMove)
	}
	for _, s := range p.says {
		add(s.t, audio.ChipSay)
	}
	for _, ev := range sc.Events {
		if ev.Do == "number" {
			add(tl.MustResolve(ev.At), audio.ChipQuestion)
		}
	}
	if sc.Code != nil {
		for _, l := range sc.Code.Lines {
			if l.Appear != nil {
				add(tl.MustResolve(*l.Appear), audio.ChipTick)
			}
		}
	}
	if p.focus != nil {
		add(p.focus.t, audio.ChipTick)
	}
	if a := sc.Answer; a != nil && sc.Guess != nil {
		t := tl.MustResolve(a.At)
		add(t+0.25, audio.ChipStrike)
		card := t + 0.7
		if a.CardAt != nil {
			card = tl.MustResolve(*a.CardAt)
		}
		add(card, audio.ChipSay)
	}
	sort.SliceStable(c, func(i, j int) bool { return c[i].T < c[j].T })
	return c
}
