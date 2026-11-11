package maket

import "math"

type mesh struct {
	top, right, left, shadow []point
	bands                    [][]point
	height                   float64
}

func cube(w, h float64) mesh {
	r := w / 2
	return mesh{
		top:    []point{{-r, -r, h}, {r, -r, h}, {r, r, h}, {-r, r, h}},
		right:  []point{{r, -r, 0}, {r, r, 0}, {r, r, h}, {r, -r, h}},
		left:   []point{{-r, r, 0}, {r, r, 0}, {r, r, h}, {-r, r, h}},
		shadow: footprint(w),
		height: h,
	}
}

func footprint(w float64) []point {
	r := w / 2
	return []point{{-r + 0.3, -r + 0.3, 0}, {r + 0.3, -r + 0.3, 0}, {r + 0.3, r + 0.3, 0}, {-r + 0.3, r + 0.3, 0}}
}

func cylinder(r, h float64) mesh {
	const steps = 64
	p := func(i int, z float64) point {
		theta := float64(i) * 2 * math.Pi / steps
		x, y := 1.2247/0.866*r*math.Cos(theta), 2*0.7071*r*math.Sin(theta)
		return point{(y + x) / 2, (y - x) / 2, z}
	}
	m := mesh{height: h, shadow: footprint(2 * r)}
	for i := 0; i < steps; i++ {
		m.top = append(m.top, p(i, h))
	}
	for _, side := range []struct {
		start, end int
		points     *[]point
	}{{0, steps / 4, &m.right}, {steps / 4, steps / 2, &m.left}} {
		for i := side.start; i <= side.end; i++ {
			*side.points = append(*side.points, p(i, 0))
		}
		for i := side.end; i >= side.start; i-- {
			*side.points = append(*side.points, p(i, h))
		}
	}
	for _, k := range []float64{1.0 / 3, 2.0 / 3} {
		var band []point
		for i := 0; i <= steps/2; i++ {
			band = append(band, p(i, h*k))
		}
		m.bands = append(m.bands, band)
	}
	return m
}
