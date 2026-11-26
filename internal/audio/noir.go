package audio

import (
	"math"
	"math/rand"
)

type noirSound struct {
	gain, duration float64
	play           func(float64, *rand.Rand) []float64
}

var noirSounds = map[string]noirSound{
	"rain":          {.32, 0, noirRain},
	"type-key":      {.35, .075, typeKey},
	"click":         {.18, .14, func(_ float64, r *rand.Rand) []float64 { return parkClick(r) }},
	"carriage-bell": {.16, .55, func(d float64, _ *rand.Rand) []float64 { return bell(1280, d) }},
	"yarn":          {.29, 0, yarnStretch},
	"pin":           {.49, .14, corkPin},
	"culprit":       {.58, .85, culpritChord},
	"shutter":       {.42, .16, shutter},
	"neon-hum":      {.06, 0, electricHum(100, .34)},
	"neon-crackle":  {.20, .08, neonCrackle},
	"watch-tick":    {.22, .045, watchTick},
	"heel":          {.38, .20, wetHeel},
	"lamp-hum":      {.075, 0, electricHum(50, .18)},
	"shock":         {.58, .40, shockHit},
	"jazz-end":      {.32, 1.1, closingChord},
}

func NoirCue(name string) (Cue, bool) {
	s, ok := noirSounds[name]
	return Cue{Kind: "noir-" + name, Gain: s.gain, Dur: s.duration}, ok
}

func noirSignal(name string, dur float64, rng *rand.Rand) []float64 {
	s, ok := noirSounds[name]
	if !ok || dur <= 0 {
		return nil
	}
	x := s.play(dur, rng)
	x = x[:min(len(x), samples(dur))]
	fade := min(len(x), samples(.012))
	for i := range fade {
		x[len(x)-1-i] *= float64(i) / float64(fade)
	}
	return x
}

func noirRain(dur float64, rng *rand.Rand) []float64 {
	x := make([]float64, samples(dur))
	var near, far, drop, pitch, phase float64
	for i := range x {
		t := float64(i) / Rate
		w := rng.NormFloat64()
		near += .46 * (w - near)
		far += .012 * (w - far)
		if rng.Float64() < 24.0/Rate {
			drop = .14 + .35*rng.Float64()
			pitch = 850 + 1500*rng.Float64()
		}
		phase += 2 * math.Pi * pitch / Rate
		drop *= .995
		wind := .86 + .10*math.Sin(.71*t) + .04*math.Sin(2.3*t)
		x[i] = ((near-far)*.78 + far*1.4 + drop*math.Sin(phase)) * wind * min(1., t/.18)
	}
	return x
}

func noiseStroke(dur, filter, decay float64, rng *rand.Rand) []float64 {
	x := lowpass(noise(samples(dur), rng), filter)
	for i := range x {
		t := float64(i) / Rate
		x[i] *= min(1., t/.001) * math.Exp(-t/decay)
	}
	return x
}

func typeKey(d float64, rng *rand.Rand) []float64 {
	f := 790 + 310*rng.Float64()
	x := noiseStroke(d, .53, .007, rng)
	x = add(x, tone(f, .72*f, d, .009, .001), .49)
	return at(x, .023, noiseStroke(.033, .19, .011, rng))
}

func yarnStretch(d float64, rng *rand.Rand) []float64 {
	x := make([]float64, samples(d))
	f := 220 + 70*rng.Float64()
	var phase, grit float64
	for i := range x {
		u := float64(i) / float64(len(x))
		t := float64(i) / Rate
		phase += 2 * math.Pi * f * (.7 + .8*u + .045*math.Sin(51*t)) / Rate
		grit += .22 * (rng.NormFloat64() - grit)
		pull := .7 + .3*math.Sin(2*math.Pi*(18+8*u)*t)
		x[i] = (.55*math.Sin(phase) + .22*math.Sin(phase*3) + .40*grit) * pull * math.Sin(math.Pi*u)
	}
	return x
}

func corkPin(d float64, rng *rand.Rand) []float64 {
	x := tone(210, 90, d, .031, .002)
	return add(x, noiseStroke(d, .17, .014, rng), .76)
}

func shockHit(d float64, rng *rand.Rand) []float64 {
	return add(tone(112, 46, d, .11, .004), noiseStroke(d, .11, .041, rng), .8)
}

func culpritChord(d float64, rng *rand.Rand) []float64 {
	x := shockHit(d, rng)
	for _, f := range [...]float64{146.83, 174.61, 207.65} {
		x = add(x, tone(f, f, d, .24, .018), .32)
	}
	return x
}

func shutter(d float64, rng *rand.Rand) []float64 {
	x := noiseStroke(d, .65, .010, rng)
	return at(x, .067, noiseStroke(.065, .34, .014, rng))
}

func electricHum(f, overtone float64) func(float64, *rand.Rand) []float64 {
	return func(d float64, _ *rand.Rand) []float64 {
		x := make([]float64, samples(d))
		for i := range x {
			t := float64(i) / Rate
			p := 2 * math.Pi * f * t
			x[i] = (math.Sin(p) + overtone*math.Sin(3*p)) * (.9 + .1*math.Sin(6*t)) * min(1., t/.02)
		}
		return x
	}
}

func neonCrackle(d float64, rng *rand.Rand) []float64 {
	x := make([]float64, samples(d))
	for t := 0.; t < d; t += .005 + .016*rng.Float64() {
		x = at(x, t, noiseStroke(.006, .88, .0015, rng))
	}
	return x
}

func watchTick(d float64, rng *rand.Rand) []float64 {
	f := 1850 + 460*rng.Float64()
	return add(tone(f, f*.88, d, .004, .0007), noiseStroke(d, .8, .003, rng), .6)
}

func wetHeel(d float64, rng *rand.Rand) []float64 {
	f := 340 + 90*rng.Float64()
	x := add(tone(f, f*.52, d, .021, .001), tone(130, 68, d, .047, .005), .65)
	return at(x, .018, noiseStroke(.16, .47, .036, rng))
}

func closingChord(d float64, _ *rand.Rand) []float64 {
	var x []float64
	for i, f := range [...]float64{73.42, 146.83, 174.61, 220, 246.94} {
		x = at(x, float64(i)*.023, add(tone(f, f, d, .28, .009), tone(2*f, 2*f, d, .19, .006), .22))
	}
	return x
}
