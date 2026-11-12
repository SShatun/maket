package moe

import "maket/internal/audio"

var soundNames = map[string]bool{
	audio.PadSunset: true, audio.PadNight: true, audio.PadIdea: true, audio.PadSakura: true,
	audio.MusicBox: true, audio.Meow: true, audio.Heartbeat: true, audio.Purr: true,
	audio.Stab: true, audio.Blip: true, audio.Sparkle: true, audio.Whoosh: true,
	audio.Land: true, audio.Thud: true, audio.Buzz: true, audio.Pop: true, audio.Chime: true,
}

func (s *Style) cue(t float64, kind string, gain, dur float64) {
	if dur < 0 {
		return
	}
	s.cues = append(s.cues, audio.Cue{T: t, Kind: kind, Gain: gain, Dur: dur})
}

func (s *Style) pad(start, end float64, name string) {
	if end <= start {
		return
	}
	kind := audio.PadSunset
	switch name {
	case "night", "rain", "alert":
		kind = audio.PadNight
	case "idea", "flash":
		kind = audio.PadIdea
	case "sakura", "moe":
		kind = audio.PadSakura
	}
	s.cue(start, kind, 0.55, end-start)
	if name == "moe" || name == "sakura" {
		for t := start; t < end; t += 3.2 {
			s.cue(t, audio.MusicBox, 0.35, 0)
		}
	}
}

func (s *Style) stateSound(kind, state string, start, end float64) {
	if end <= start {
		return
	}
	switch state {
	case "happy":
		if kind == "session" {
			s.cue(start, audio.Meow, 0.8, 0)
		} else {
			s.cue(start, audio.Sparkle, 0.5, 0)
		}
	case "dere", "love":
		s.cue(start, audio.Heartbeat, 0.5, end-start)
	case "sleep":
		s.cue(start, audio.Purr, 0.5, end-start)
	case "baka":
		s.cue(start, audio.Stab, 0.65, 0)
	case "give", "allow":
		s.cue(start, audio.Sparkle, 0.45, 0)
	}
}
