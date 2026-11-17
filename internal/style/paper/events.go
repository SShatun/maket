package paper

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"maket/internal/audio"
	"maket/internal/scene"
	"maket/internal/style/sprites"
)

func (s *Style) event(sh *shot, ev scene.Event) error {
	t, err := s.resolve(ev.At, sh.at, sh.until)
	if err != nil {
		return err
	}
	until := sh.until
	if ev.Until != nil {
		until, err = s.tl.Resolve(*ev.Until)
		if err != nil {
			return err
		}
		if until <= t {
			return fmt.Errorf("until must follow at")
		}
		until = math.Min(until, sh.until)
	}
	var x struct {
		Name    string  `json:"name"`
		Side    string  `json:"side"`
		Color   string  `json:"color"`
		Episode int     `json:"episode"`
		CPS     float64 `json:"cps"`
		Sound   *bool   `json:"sound"`
	}
	if err := json.Unmarshal(ev.Raw, &x); err != nil {
		return err
	}
	audible := x.Sound == nil || *x.Sound
	cue := func(kind string, dur float64) {
		if audible {
			s.cues = append(s.cues, audio.Cue{T: t, Kind: kind, Gain: 1, Dur: dur})
		}
	}
	target := func() (*actor, error) {
		if ev.Target == nil || sh.byID[ev.Target.ID] == nil {
			return nil, fmt.Errorf("%s needs a cast target", ev.Do)
		}
		return sh.byID[ev.Target.ID], nil
	}
	switch ev.Do {
	case "state":
		a, err := target()
		if err != nil {
			return err
		}
		a.states = append(a.states, sprites.Step{T: t, V: ev.State, Hold: ev.Hold})
		if a.kind == "dialog" && ev.State == "closed" {
			cue(audio.Pop, 0)
		}
		if ev.State == "pushed" || a.kind == "deploy" && ev.State == "work" {
			cue(audio.Whoosh, 0)
			cue(audio.Rise, 0)
		}
		if ev.State == "happy" {
			cue(audio.Chime, 0)
		}
	case "role":
		a, err := target()
		if err != nil {
			return err
		}
		a.roles = append(a.roles, role{t, ev.Role})
	case "walk", "enter":
		a, err := target()
		if err != nil {
			return err
		}
		px, py := a.position(t)
		from := [2]float64{px, py}
		to := a.home
		dur := ev.Dur
		if dur == 0 {
			dur = 0.8
		}
		if dur < 0 {
			return fmt.Errorf("dur must be positive")
		}
		if ev.Do == "enter" {
			from, err = s.edge(a, x.Side)
			if err != nil {
				return err
			}
		} else {
			if ev.To == nil || len(ev.To.Coords) != 2 {
				return fmt.Errorf("walk needs to: [x, y]")
			}
			to = [2]float64{ev.To.Coords[0], ev.To.Coords[1]}
		}
		a.moves = append(a.moves, motion{t, dur, from, to})
	case "bg":
		if err := s.loadBackground(x.Name); err != nil {
			return err
		}
		sh.backgrounds = append(sh.backgrounds, background{t, x.Name})
		for i := range sh.chalk {
			sh.chalk[i].until = math.Min(sh.chalk[i].until, t)
		}
		dur, white := 0.3, false
		if x.Name == "classroom" {
			dur, white = 0.15, true
		}
		if ev.Dur != 0 {
			dur = ev.Dur
		}
		if dur <= 0 || dur > 2 {
			return fmt.Errorf("bg dur must be in (0, 2]")
		}
		s.cut(transition{t, dur, white})
		if audible {
			s.cues = append(s.cues, audio.Cue{T: t, Kind: audio.Strum, Gain: 1, Chord: len(sh.backgrounds) - 1})
		}
	case "chalk":
		if sh.backgroundAt(t) != "classroom" {
			return fmt.Errorf("chalk requires classroom")
		}
		lines, err := plainLines(ev)
		if err != nil {
			return err
		}
		cps := x.CPS
		if cps == 0 {
			cps = 18
		}
		if cps <= 0 {
			return fmt.Errorf("chalk cps must be positive")
		}
		if err := s.checkChalk(lines); err != nil {
			return err
		}
		for i := range sh.chalk {
			sh.chalk[i].until = math.Min(sh.chalk[i].until, t)
		}
		sh.chalk = append(sh.chalk, lettering{at: t, until: until, cps: cps, lines: lines, sound: audible})
	case "title":
		lines, err := plainLines(ev)
		if err != nil {
			return err
		}
		im, err := s.titleImage(x.Episode, lines)
		if err != nil {
			return err
		}
		sh.titles = append(sh.titles, lettering{at: t, until: until, image: im})
	case "text":
		size := ev.Size
		if size == 0 {
			size = 44
		}
		col := s.theme.Ink
		switch ev.Tone {
		case "error":
			col = s.theme.Error
		case "ok":
			col = s.theme.OK
		case "accent":
			col = s.theme.Error
		}
		im, err := s.textImage(ev.Text, size, col)
		if err != nil {
			return err
		}
		sh.labels = append(sh.labels, lettering{at: t, until: until, image: im, pos: [2]float64{ev.Pos[0], ev.Pos[1]}})
	case "cut", "flash":
		white := x.Color == "white" || ev.Do == "flash"
		if x.Color != "" && x.Color != "black" && x.Color != "white" {
			return fmt.Errorf("cut color must be black or white")
		}
		dur := ev.Dur
		if dur == 0 {
			dur = 0.3
			if white {
				dur = 0.15
			}
		}
		if dur <= 0 || dur > 2 {
			return fmt.Errorf("cut dur must be in (0, 2]")
		}
		if ev.Do == "flash" {
			s.cuts = append(s.cuts, transition{t + dur/2, dur, true})
		} else {
			s.cut(transition{t, dur, white})
		}
	case "sfx":
		switch x.Name {
		case audio.Strum, audio.Babble, audio.Chalk, audio.Pop, audio.Whoosh, audio.Rise, audio.Land, audio.Thud, audio.Buzz, audio.Chime:
		default:
			return fmt.Errorf("unknown paper sfx %q", x.Name)
		}
		if ev.Dur < 0 {
			return fmt.Errorf("sfx dur must be nonnegative")
		}
		dur := ev.Dur
		if dur == 0 {
			dur = until - t
		}
		cue(x.Name, math.Min(dur, until-t))
	default:
		return fmt.Errorf("paper does not support %q", ev.Do)
	}
	return nil
}

func plainLines(ev scene.Event) ([]string, error) {
	var lines []string
	for _, l := range ev.Lines {
		if l.Appear != nil || len(l.Timed) > 0 || l.Field != "" {
			return nil, fmt.Errorf("%s lines must be plain text", ev.Do)
		}
		lines = append(lines, l.Text)
	}
	if len(lines) == 0 && ev.Text != "" {
		lines = strings.Split(ev.Text, "\n")
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("%s needs lines or text", ev.Do)
	}
	return lines, nil
}

func (s *Style) cut(c transition) {
	for i := range s.cuts {
		if math.Abs(s.cuts[i].at-c.at) < 1e-8 {
			s.cuts[i] = c
			return
		}
	}
	s.cuts = append(s.cuts, c)
}
