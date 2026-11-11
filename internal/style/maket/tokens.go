package maket

import (
	"image/color"
	"math"
)

const (
	blockSize = 1.8
	blockH    = 1.2
	slotSize  = 2.0
	slotH     = 0.45
	tokenSize = 1.0
	cylinderR = 1.0
	cylinderH = 1.4
	labelGap  = 34.0
	riseDur   = 0.45
	sendDur   = 0.7
	sendArc   = 1.6
	shiftArc  = 0.8
	shakeDur  = 0.45
	shakePX   = 7.0
	focusDur  = 0.6
	focusDim  = 0.4
	driftZoom = 0.03
	focusZoom = 0.03
	swayPX    = 8.0
)

var (
	ground    = color.RGBA{220, 227, 232, 255}
	grid      = color.RGBA{211, 219, 225, 255}
	shadow    = color.RGBA{202, 210, 216, 255}
	ink       = color.RGBA{31, 41, 51, 255}
	muted     = color.RGBA{84, 95, 106, 255}
	errorText = color.RGBA{176, 38, 46, 255}
	card      = color.RGBA{250, 251, 252, 255}
	roles     = map[string]color.RGBA{
		"producer": {30, 158, 140, 255},
		"consumer": {124, 92, 230, 255},
		"buffer":   {240, 162, 2, 255},
		"storage":  {47, 111, 219, 255},
		"external": {91, 107, 122, 255},
		"error":    {229, 72, 77, 255},
		"ok":       {46, 160, 67, 255},
	}
)

func clamp(x float64) float64 { return math.Max(0, math.Min(1, x)) }

func ease(x float64) float64 {
	x = clamp(x)
	if x < 0.5 {
		return 4 * x * x * x
	}
	return 1 - math.Pow(-2*x+2, 3)/2
}

func rise(x float64) float64 {
	x = clamp(x) - 1
	return 1 + 2.5*x*x*x + 1.5*x*x
}

func prog(t, a, b float64) float64 {
	if b <= a {
		if t >= a {
			return 1
		}
		return 0
	}
	return ease((t - a) / (b - a))
}

func lerp(a, b, k float64) float64 { return a + (b-a)*k }

func mix(c, base color.RGBA, k float64) color.RGBA {
	return color.RGBA{
		uint8(lerp(float64(base.R), float64(c.R), k)),
		uint8(lerp(float64(base.G), float64(c.G), k)),
		uint8(lerp(float64(base.B), float64(c.B), k)), 255,
	}
}

func shade(c color.RGBA, k float64) color.RGBA {
	return mix(c, color.RGBA{A: 255}, k)
}
