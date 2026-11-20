package park

import (
	"image"
	"math"
	"sort"

	"maket/internal/timeline"
)

type point [2]float64

func (p point) pixel() image.Point {
	return image.Pt(int(math.Round(p[0]/scale)), int(math.Round(p[1]/scale)))
}
func (p point) distance(q point) float64 { return math.Hypot(q[0]-p[0], q[1]-p[1]) }
func (p point) mix(q point, u float64) point {
	return point{p[0] + (q[0]-p[0])*u, p[1] + (q[1]-p[1])*u}
}
func smooth(u float64) float64 { u = min(1, max(0, u)); return u * u * (3 - 2*u) }

type leg struct {
	at, end  float64
	from, to point
	view     string
	flip     bool
}

func (l leg) position(t float64) point {
	if t >= l.end {
		return l.to
	}
	return l.from.mix(l.to, max(0, (t-l.at)/(l.end-l.at)))
}

type mark struct {
	at, end float64
	value   string
}

func valueAt(marks []mark, t float64) string {
	for i := len(marks) - 1; i >= 0; i-- {
		m := marks[i]
		if m.at <= t && t < m.end {
			return m.value
		}
	}
	return ""
}

type bubbleMark struct {
	mark
	offset point
}

type person struct {
	kind                        string
	phase                       float64
	legs                        []leg
	hidden, poses, items, icons []mark
	bubbles                     []bubbleMark
}

func (p *person) pose(t float64) (leg, point) {
	i := sort.Search(len(p.legs), func(i int) bool { return p.legs[i].at > t }) - 1
	l := p.legs[max(i, 0)]
	return l, l.position(t)
}

func direction(from, to point) (string, bool) {
	dx, dy := to[0]-from[0], to[1]-from[1]
	if math.Abs(dx) > math.Abs(dy) {
		return "side", dx < 0
	}
	if dy < 0 {
		return "back", false
	}
	return "front", false
}

func (p *person) move(t, end float64, from, to point, view string, flip bool) {
	i := sort.Search(len(p.legs), func(i int) bool { return p.legs[i].at >= t })
	if view == "" {
		view, flip = direction(from, to)
	}
	p.legs = append(p.legs[:i], leg{t, end, from, to, view, flip})
}

func (a *art) nearest(p point) string {
	best := ""
	dist := math.Inf(1)
	for _, id := range a.nodes {
		if d := p.distance(a.paths.Nodes[id]); d < dist {
			dist, best = d, id
		}
	}
	return best
}

func (a *art) route(from, to string) []point {
	dist := map[string]float64{from: 0}
	prev := map[string]string{}
	visited := map[string]bool{}
	for {
		best := ""
		lowest := math.Inf(1)
		for _, id := range a.nodes {
			if d, ok := dist[id]; ok && !visited[id] && d < lowest {
				lowest, best = d, id
			}
		}
		if best == "" {
			return nil
		}
		if best == to {
			break
		}
		visited[best] = true
		for _, edge := range a.paths.Edges {
			next := edge[0]
			if best == next {
				next = edge[1]
			} else if best != edge[1] {
				continue
			}
			d := lowest + a.paths.Nodes[best].distance(a.paths.Nodes[next])
			if old, ok := dist[next]; !ok || d < old {
				dist[next], prev[next] = d, best
			}
		}
	}
	var route []point
	for id := to; id != ""; id = prev[id] {
		route = append(route, a.paths.Nodes[id])
	}
	for i, j := 0, len(route)-1; i < j; i, j = i+1, j-1 {
		route[i], route[j] = route[j], route[i]
	}
	return route
}

func (o *object) runningSpans(start, end float64) []timeline.Interval {
	times := []float64{start, end}
	for _, m := range o.states {
		times = append(times, m.at)
	}
	if o.power != nil {
		for _, m := range o.power.states {
			times = append(times, m.at)
		}
	}
	sort.Float64s(times)
	var spans []timeline.Interval
	for i, from := range times[:len(times)-1] {
		to := times[i+1]
		if from < to && o.state(from) == "running" {
			spans = append(spans, timeline.Interval{From: from, To: to})
		}
	}
	return spans
}

func (o *object) runningTime(t float64) float64 {
	elapsed := 0.0
	for _, span := range o.running {
		if t <= span.From {
			break
		}
		elapsed += min(t, span.To) - span.From
	}
	return elapsed
}

func (a *art) prepareRail() {
	pts := a.paths.Coaster.Points
	a.railTimes = []float64{0}
	for i, p := range pts {
		q := pts[(i+1)%len(pts)]
		dx, dy := q[0]-p[0], q[1]-p[1]
		d := math.Hypot(dx, dy)
		speed := 210 + 100*dy/max(1, d)
		a.railTimes = append(a.railTimes, a.railTimes[i]+d/speed)
	}
}

func (a *art) railPhase(t float64) float64 {
	n := len(a.railTimes) - 1
	t = math.Mod(t, a.railTimes[n])
	i := sort.Search(n, func(i int) bool { return a.railTimes[i+1] > t })
	u := (t - a.railTimes[i]) / (a.railTimes[i+1] - a.railTimes[i])
	return (float64(i) + u) / float64(n)
}

func (a *art) railPoint(phase float64) point {
	pts := a.paths.Coaster.Points
	phase -= math.Floor(phase)
	i := sort.Search(len(pts), func(i int) bool { return pts[i][2] > phase }) - 1
	p, q := pts[i], pts[(i+1)%len(pts)]
	end := q[2]
	if end == 0 {
		end = 1
	}
	return (point{p[0], p[1]}).mix(point{q[0], q[1]}, (phase-p[2])/(end-p[2]))
}
