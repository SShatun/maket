package noir

import (
	"strings"
	"unicode/utf8"

	"maket/internal/audio"
	"maket/internal/render"
)

func (s *Style) addSound(c audio.Cue, at, end float64) {
	if at >= end {
		return
	}
	c.T = at
	if c.Dur == 0 || c.Dur > end-at {
		c.Dur = end - at
	}
	s.cues = append(s.cues, c)
}

func (s *Style) play(name string, at, end float64) {
	c, _ := audio.NoirCue(name)
	s.addSound(c, at, end)
}

func (sh *shot) nextReplacement(i int) float64 {
	e := sh.actions[i]
	for _, next := range sh.actions[i+1:] {
		if next.Do == e.Do && (e.Do != "log" || next.Row == e.Row) {
			return next.at
		}
	}
	return sh.end
}

func (sh *shot) score() {
	if sh.place == "interrogation" {
		sh.play("lamp-hum", sh.start, sh.end)
	}
	bakedNeonFrom := sh.start
	for i, e := range sh.actions {
		switch e.Do {
		case "plate":
			sh.typeSounds(e, sh.nextReplacement(i), e.Title, sh.stageAt(e.at).sub(e.Sub))
		case "headline":
			sh.typeSounds(e, sh.nextReplacement(i), e.headline(), e.Time)
		case "log":
			sh.typeSounds(e, sh.nextReplacement(i), e.Text)
		case "reveal":
			sh.boardSounds(e, sh.nextReplacement(i))
		case "neon":
			if sh.Neon.Text != "" && len(e.Pos) == 0 {
				sh.play("neon-hum", bakedNeonFrom, e.at)
				bakedNeonFrom = max(bakedNeonFrom, e.end)
			}
		case "prop":
			if e.Kind == "watch" {
				for at := e.at; at < e.end; at++ {
					sh.play("watch-tick", at, e.end)
				}
			}
		case "icon":
			if e.Kind == "bang" && sh.place == "interrogation" {
				sh.play("shock", e.at, e.end)
			}
		}
	}
	if sh.Neon.Text != "" {
		sh.play("neon-hum", bakedNeonFrom, sh.end)
	}
	for _, p := range sh.people {
		for i, m := range p.moves {
			end := min(m.end, sh.end)
			if i+1 < len(p.moves) {
				end = min(end, p.moves[i+1].at)
			}
			for at := m.at; at < end; at += .5 {
				sh.play("heel", at, sh.end)
			}
		}
	}
}

func (sh *shot) typeSounds(e *action, end float64, lines ...string) {
	end = min(end, e.end)
	total := utf8.RuneCountInString(strings.Join(lines, ""))
	if total == 0 || end <= e.at {
		return
	}
	dur := e.Dur
	if dur == 0 {
		dur = min(.72, float64(total)*.045, (end-e.at)*.8)
	}
	if e.Dur > 0 {
		sh.tl.AddActivity(e.at, min(end, e.at+dur))
	}
	nextKey := e.at
	index := 0
	for _, line := range lines {
		for range line {
			index++
			t := e.at + dur*float64(index)/float64(total)
			if t >= end {
				return
			}
			if t >= nextKey {
				sh.play("type-key", t, end)
				nextKey = t + .032
			}
		}
		if line != "" {
			sh.play("carriage-bell", e.at+dur*float64(index)/float64(total), end)
		}
	}
}

func (sh *shot) boardSounds(e *action, next float64) {
	end := min(e.end, next)
	for i := range sh.board.yarns {
		at := e.at + float64(i)*e.Every
		if at >= end {
			break
		}
		sh.tl.AddActivity(at, min(at+e.Dur, end))
		sh.play("yarn", at, min(at+e.Dur, end))
		sh.play("pin", at+e.Dur, end)
	}
	marker := e.at + float64(len(sh.board.yarns))*e.Every
	if at := min(marker, e.end); sh.board.marker != nil && at < next {
		sh.play("culprit", at, next)
		sh.tl.AddActivity(at, at)
	}
	if at := min(marker+e.Every, e.end); at < next && len(sh.board.notes) > 0 {
		sh.tl.AddActivity(at, at)
	}
}

func (sh *shot) neonSounds(e *action) {
	from := e.at
	for i, frame := 0, 0; from < e.end; i++ {
		frame += blinkGap(i)
		at := e.at + float64(frame)/render.FPS
		sh.play("neon-hum", from, min(at, e.end))
		if at >= e.end {
			break
		}
		from = min(e.end, at+float64(2+i%2)/render.FPS)
		sh.play("neon-crackle", at, from)
		sh.tl.AddActivity(at, from)
	}
}
