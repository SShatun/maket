package audio

import (
	"math"
	"math/rand"
)

const (
	fxLevel     = 0.18
	fxPeak      = 0.85
	duckTo      = 0.25
	duckAttack  = 0.005
	duckRelease = 0.25
	speechLevel = 0.02
	roomDelay   = 0.09
	roomMix     = 0.18
	limit       = 0.8
	ceiling     = 0.891
)

var baseGain = map[string]float64{
	Rise: 0.35, Whoosh: 0.18, Land: 0.55, Thud: 0.75, Buzz: 0.35, Pop: 0.5, Chime: 0.45,
	PadSunset: 0.12, PadNight: 0.12, PadIdea: 0.12, PadSakura: 0.12, Drone: 0.14, DroneLow: 0.14,
	MusicBox: 0.35, Meow: 0.45, Heartbeat: 0.6, Purr: 0.25, Stab: 0.6, Blip: 0.3, Sparkle: 0.35,
	Pluck: 0.4, Drums: 0.55, Horn: 0.4, Crackle: 0.25, Clunk: 0.6,
}

var pads = map[string][]float64{
	PadSunset: {220, 277, 330, 440},
	PadNight:  {196, 233, 294},
	PadIdea:   {262, 330, 392, 523},
	PadSakura: {294, 370, 440, 587},
	Drone:     {146.8, 220, 293.7},
	DroneLow:  {73.4, 110, 146.8},
}

func Mix(voiceTrack string, cues []Cue, duration float64, out string) error {
	var voice []float32
	if voiceTrack != "" {
		v, err := Read(voiceTrack, "loudnorm=I=-16:TP=-1.5:LRA=11")
		if err != nil {
			return err
		}
		voice = v
	}
	n := max(int(duration*Rate), len(voice))
	fx := make([]float64, n)
	rng := rand.New(rand.NewSource(7))
	for _, c := range cues {
		sig := synth(c.Kind, rng)
		if sig == nil {
			sig = long(c.Kind, c.Dur, rng)
		}
		g := c.Gain
		if b, ok := baseGain[c.Kind]; ok {
			g *= b
		}
		at := int(c.T * Rate)
		for i, v := range sig {
			if at+i >= 0 && at+i < n {
				fx[at+i] += v * g
			}
		}
	}
	room(fx)
	peak := 0.0
	for _, v := range fx {
		peak = math.Max(peak, math.Abs(v))
	}
	if peak > 0 {
		k := fxPeak / peak * fxLevel
		for i := range fx {
			fx[i] *= k
		}
	}
	duck := envelope(voice, n)
	mix := make([]float32, n)
	for i := range mix {
		v := fx[i] * duck[i]
		if i < len(voice) {
			v += float64(voice[i])
		}
		mix[i] = float32(soft(v))
	}
	return Write(out, mix)
}

func envelope(voice []float32, n int) []float64 {
	out := make([]float64, n)
	att := 1 - math.Exp(-1/(duckAttack*Rate))
	rel := 1 - math.Exp(-1/(duckRelease*Rate))
	env, g := 0.0, 1.0
	for i := range out {
		x := 0.0
		if i < len(voice) {
			x = math.Abs(float64(voice[i]))
		}
		if x > env {
			env += att * (x - env)
		} else {
			env += rel * (x - env)
		}
		target := 1.0
		if env > speechLevel {
			target = duckTo
		}
		if target < g {
			g += att * (target - g)
		} else {
			g += rel * (target - g)
		}
		out[i] = g
	}
	return out
}

func room(x []float64) {
	d := int(roomDelay * Rate)
	for i := len(x) - 1; i >= d; i-- {
		x[i] += x[i-d] * roomMix
	}
}

func soft(v float64) float64 {
	a := math.Abs(v)
	if a <= limit {
		return v
	}
	s := limit + (ceiling-limit)*math.Tanh((a-limit)/(ceiling-limit))
	return math.Copysign(s, v)
}

func samples(dur float64) int { return int(dur * Rate) }

func env(n int, a, d float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		t := float64(i) / Rate
		out[i] = math.Min(1, t/a) * math.Exp(-t/d)
	}
	return out
}

func tone(f0, f1, dur, d, a float64) []float64 {
	n := samples(dur)
	e := env(n, a, d)
	out := make([]float64, n)
	ph := 0.0
	for i := range out {
		f := f0 + (f1-f0)*float64(i)/float64(max(1, n-1))
		ph += 2 * math.Pi * f / Rate
		out[i] = math.Sin(ph) * e[i]
	}
	return out
}

func lowpass(x []float64, k float64) []float64 {
	out := make([]float64, len(x))
	acc := 0.0
	for i, v := range x {
		acc += k * (v - acc)
		out[i] = acc
	}
	return out
}

func noise(n int, rng *rand.Rand) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = rng.NormFloat64()
	}
	return out
}

func add(dst []float64, src []float64, k float64) []float64 {
	if len(src) > len(dst) {
		dst = append(dst, make([]float64, len(src)-len(dst))...)
	}
	for i, v := range src {
		dst[i] += v * k
	}
	return dst
}

func square(f0, f1, dur, vol float64) []float64 {
	n := samples(dur)
	out := make([]float64, n)
	ph := 0.0
	for i := range out {
		f := f0 + (f1-f0)*float64(i)/float64(max(1, n-1))
		ph += f / Rate
		v := 1.0
		if math.Sin(2*math.Pi*ph) < 0 {
			v = -1
		}
		out[i] = v * vol * math.Min(1, float64(n-i)/(Rate*0.02))
	}
	return out
}

func fading(n int, vol float64, rng *rand.Rand) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = (rng.Float64()*2 - 1) * vol * (1 - float64(i)/float64(n))
	}
	return out
}

func at(sig []float64, t float64, part []float64) []float64 {
	off := samples(t)
	if need := off + len(part); need > len(sig) {
		sig = append(sig, make([]float64, need-len(sig))...)
	}
	for i, v := range part {
		sig[off+i] += v
	}
	return sig
}

func synth(kind string, rng *rand.Rand) []float64 {
	switch kind {
	case Whoosh:
		n := samples(0.5)
		x := lowpass(noise(n, rng), 0.06)
		for i := range x {
			s := math.Sin(math.Pi * float64(i) / float64(n))
			x[i] *= s * s * 1.6
		}
		return x
	case Land:
		n := samples(0.12)
		x := tone(900, 520, 0.12, 0.035, 0.003)
		x = add(x, tone(1800, 1500, 0.05, 0.012, 0.003), 0.5)
		cl := lowpass(noise(samples(0.03), rng), 0.4)
		e := env(len(cl), 0.001, 0.008)
		for i := range cl {
			cl[i] *= e[i]
		}
		return add(x[:n], cl, 0.3)
	case Rise:
		return add(nil, tone(300, 420, 0.18, 0.06, 0.003), 0.5)
	case Thud:
		x := add(nil, tone(140, 55, 0.45, 0.16, 0.003), 1.3)
		b := lowpass(noise(samples(0.2), rng), 0.08)
		e := env(len(b), 0.002, 0.05)
		for i := range b {
			b[i] *= e[i] * 1.2
		}
		return add(x, b, 1)
	case Buzz:
		n := samples(0.22)
		x := make([]float64, n)
		for i := range x {
			t := float64(i) / Rate
			x[i] = sgn(math.Sin(2*math.Pi*110*t))*0.25 + sgn(math.Sin(2*math.Pi*116*t))*0.25
		}
		x = lowpass(x, 0.15)
		e := env(n, 0.005, 0.12)
		for i := range x {
			x[i] *= e[i]
		}
		return x
	case Pop:
		return add(nil, tone(420, 980, 0.16, 0.06, 0.003), 0.9)
	case Chime:
		x := tone(660, 660, 1.6, 0.5, 0.01)
		x = add(x, tone(990, 990, 1.6, 0.4, 0.01), 0.6)
		x = add(x, tone(1320, 1320, 1.2, 0.3, 0.01), 0.3)
		for i := range x {
			x[i] *= 0.5
		}
		return x
	case ChipTitle:
		return at(square(523, 523, 0.08, 0.25), 0.09, square(784, 784, 0.12, 0.25))
	case ChipAppear:
		return square(660, 660, 0.05, 0.12)
	case ChipSay:
		return at(square(880, 880, 0.05, 0.15), 0.06, square(1175, 1175, 0.06, 0.15))
	case ChipMove:
		return square(300, 900, 0.18, 0.12)
	case ChipBlock:
		return add(square(110, 110, 0.3, 0.3), fading(samples(0.25), 0.15, rng), 1)
	case ChipQuestion:
		return at(square(440, 440, 0.07, 0.15), 0.08, square(660, 660, 0.1, 0.15))
	case ChipTick:
		return square(1400, 1400, 0.03, 0.1)
	case ChipOK:
		var x []float64
		for i, f := range []float64{523, 659, 784, 1047} {
			x = at(x, float64(i)*0.07, square(f, f, 0.09, 0.18))
		}
		return x
	case ChipStrike:
		return add(fading(samples(0.2), 0.2, rng), square(200, 90, 0.15, 0.15), 1)
	case MusicBox:
		var x []float64
		for i, f := range []float64{1047, 1319, 1568, 1319, 1175} {
			x = at(x, float64(i)*0.2, bell(f, 0.9))
		}
		return x
	case Meow:
		n := samples(0.42)
		x := make([]float64, n)
		ph := 0.0
		for i := range x {
			t := float64(i) / Rate
			u := t / 0.42
			f := 520 + 520*math.Sin(math.Pi*u)
			f *= 1 + 0.02*math.Sin(2*math.Pi*7*t)
			ph += 2 * math.Pi * f / Rate
			v := 0.0
			for h := 1.0; h <= 4; h++ {
				v += math.Sin(ph*h) / h
			}
			x[i] = v * math.Sin(math.Pi*u) * 0.6
		}
		return x
	case Stab:
		n := samples(0.5)
		x := make([]float64, n)
		for i := range x {
			t := float64(i) / Rate
			x[i] = (sgn(math.Sin(2*math.Pi*98*t)) + sgn(math.Sin(2*math.Pi*104*t)) + sgn(math.Sin(2*math.Pi*147*t))) / 3 * math.Exp(-t/0.15)
		}
		return add(lowpass(x, 0.12), synth(Thud, rng), 0.8)
	case Blip:
		return at(tone(1200, 1200, 0.08, 0.05, 0.002), 0.08, tone(1500, 1500, 0.08, 0.05, 0.002))
	case Sparkle:
		x := tone(1568, 1568, 0.5, 0.18, 0.003)
		x = add(x, tone(2093, 2093, 0.5, 0.15, 0.003), 0.7)
		return add(x, tone(2637, 2637, 0.5, 0.12, 0.003), 0.5)
	case Pluck:
		var x []float64
		for i, f := range []float64{294, 330, 392, 440, 523} {
			x = at(x, float64(i)*0.32, pluck(f))
		}
		return x
	case Horn:
		n := samples(1.7)
		x := make([]float64, n)
		for i := range x {
			t := float64(i) / Rate
			v := 1 + 0.01*math.Sin(2*math.Pi*5*t)
			saw := func(f float64) float64 { p := f * v * t; return 2 * (p - math.Floor(p+0.5)) }
			x[i] = (saw(110) + saw(98)) * 0.5 * math.Min(1, t/0.15) * math.Min(1, (1.7-t)/0.3)
		}
		return lowpass(x, 0.05)
	case Clunk:
		return add(synth(Thud, rng), tone(260, 180, 0.2, 0.08, 0.003), 0.6)
	}
	return nil
}

func long(kind string, dur float64, rng *rand.Rand) []float64 {
	if dur <= 0 {
		dur = 4
	}
	n := samples(dur)
	fade := func(i int) float64 {
		t := float64(i) / Rate
		return math.Min(1, math.Min(t/0.5, (dur-t)/0.5))
	}
	switch kind {
	case PadSunset, PadNight, PadIdea, PadSakura, Drone, DroneLow:
		fs := pads[kind]
		x := make([]float64, n)
		for i := range x {
			t := float64(i) / Rate
			v := 0.0
			for _, f := range fs {
				v += math.Sin(2 * math.Pi * f * t)
			}
			x[i] = v / float64(len(fs)) * (0.75 + 0.25*math.Sin(2*math.Pi*0.3*t)) * fade(i)
		}
		return x
	case Heartbeat:
		var x []float64
		for t := 0.0; t < dur; t += 0.42 {
			x = at(x, t, tone(70, 45, 0.12, 0.05, 0.002))
			x = at(x, t+0.16, tone(70, 45, 0.12, 0.05, 0.002))
		}
		return x
	case Purr:
		x := lowpass(noise(n, rng), 0.03)
		for i := range x {
			t := float64(i) / Rate
			x[i] *= (0.5 + 0.5*math.Sin(2*math.Pi*24*t)) * fade(i) * 2
		}
		return x
	case Drums:
		var x []float64
		gap, lo := 0.34, false
		for t := 0.0; t < dur; t += gap {
			f := 110.0
			if lo {
				f = 80
			}
			x = at(x, t, tone(f, f*0.6, 0.25, 0.09, 0.002))
			lo = !lo
			gap = math.Max(0.12, gap*0.93)
		}
		return x
	case Crackle:
		x := lowpass(noise(n, rng), 0.02)
		for i := range x {
			x[i] *= 0.4 * fade(i)
			if rng.Float64() < 120.0/Rate {
				x[i] += (rng.Float64()*2 - 1) * 0.9
			}
		}
		return x
	}
	return nil
}

func bell(f, dur float64) []float64 {
	x := tone(f, f, dur, 1/7.0, 0.002)
	x = add(x, tone(f*2.76, f*2.76, dur, 1/7.0, 0.002), 0.4)
	return add(x, tone(f*5.4, f*5.4, dur, 1/7.0, 0.002), 0.2)
}

func pluck(f float64) []float64 {
	x := tone(f, f, 0.6, 1/6.0, 0.002)
	x = add(x, tone(f*2, f*2, 0.6, 1/6.0, 0.002), 0.4)
	return add(x, tone(f*3, f*3, 0.6, 1/6.0, 0.002), 0.2)
}

func sgn(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}
