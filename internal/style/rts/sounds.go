package rts

import (
	"math"
	"sort"

	"maket/internal/audio"
)

func (s *Style) sounds() []audio.Cue {
	var c []audio.Cue
	add := func(t float64, kind string, dur float64) {
		c = append(c, audio.Cue{T: math.Max(0, t), Kind: kind, Gain: 1, Dur: dur})
	}
	for i, sh := range s.shots {
		fires := fires(sh)
		horde := !math.IsInf(sh.horde, 1) && sh.horde < sh.until
		drone := audio.Drone
		if horde || len(fires) > 0 {
			drone = audio.DroneLow
		}
		add(sh.at-0.25, drone, sh.until-sh.at+0.5)
		if horde {
			add(sh.horde, audio.Drums, math.Min(4, sh.until-sh.horde))
		}
		if sh.horn < sh.until {
			add(sh.horn, audio.Horn, 0)
		}
		if !horde && len(fires) == 0 {
			for t := sh.at + 0.3; t < sh.until-1; t += 4 {
				add(t, audio.Pluck, 0)
			}
			if i == len(s.shots)-1 {
				add(sh.at+0.3, audio.Chime, 0)
			}
		}
		for _, f := range fires {
			add(f[0], audio.Thud, 0)
			add(f[0], audio.Crackle, f[1]-f[0])
		}
		for _, p := range sh.passes {
			if p < sh.until {
				add(p, audio.Clunk, 0)
			}
		}
		for _, d := range sh.denies {
			add(d, audio.Buzz, 0)
		}
		for _, x := range sh.shocks {
			add(x.t, audio.Thud, 0)
		}
		c = append(c, sh.sfx...)
	}
	sort.SliceStable(c, func(i, j int) bool { return c[i].T < c[j].T })
	return c
}

func fires(sh *shot) [][2]float64 {
	var out [][2]float64
	for _, a := range sh.actors {
		if a.Unit {
			continue
		}
		for i, st := range a.Steps {
			if st.V != "bad" || (i > 0 && a.Steps[i-1].V == "bad") {
				continue
			}
			end := sh.until
			for _, nx := range a.Steps[i+1:] {
				if nx.V != "bad" {
					end = math.Min(end, nx.T)
					break
				}
			}
			if end > st.T {
				out = append(out, [2]float64{st.T, end})
			}
		}
	}
	return out
}
