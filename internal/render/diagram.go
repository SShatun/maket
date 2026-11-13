package render

import (
	"fmt"
	"math"

	"maket/internal/scene"
)

func (o overlay) diagram() {
	d := o.sc.Diagram
	if d == nil {
		return
	}
	nodes := map[string]scene.DiagramNode{}
	for _, n := range d.Nodes {
		nodes[n.ID] = n
	}
	results := map[string]string{}
	visited := map[string]bool{}
	walked := map[[2]string]bool{}
	active := -1
	for i, step := range d.Steps {
		if o.t < o.tl.MustResolve(step.At) {
			break
		}
		active = i
		visited[step.Target] = true
		if step.Result != "" {
			results[step.Target] = step.Result
		}
		if i > 0 {
			walked[[2]string{d.Steps[i-1].Target, step.Target}] = true
		}
	}
	for _, link := range d.Links {
		a, b := diagramEnds(nodes[link.From], nodes[link.To])
		col := o.th.CodeDim
		if walked[[2]string{link.From, link.To}] {
			col = o.th.OK
		}
		o.dc.SetColor(col)
		o.dc.SetLineWidth(5)
		o.dc.DrawLine(a[0], a[1], b[0], b[1])
		o.dc.Stroke()
		angle := math.Atan2(b[1]-a[1], b[0]-a[0])
		for _, side := range []float64{-1, 1} {
			o.dc.DrawLine(b[0], b[1], b[0]-18*math.Cos(angle)+side*10*math.Sin(angle), b[1]-18*math.Sin(angle)-side*10*math.Cos(angle))
		}
		o.dc.Stroke()
	}
	for _, n := range d.Nodes {
		col := o.th.CodeDim
		if visited[n.ID] {
			col = o.th.OK
		}
		bw := 4.0
		if active >= 0 && d.Steps[active].Target == n.ID {
			col, bw = o.th.Accent, 8
		}
		x0, y0, x1, y1 := n.Box[0], n.Box[1], n.Box[2], n.Box[3]
		FrameBox(o.dc, x0, y0, x1, y1, col, o.th.Card, bw)
		face := o.th.Fonts.Semi
		if o.th.Pixel {
			face = o.th.Fonts.Pixel
		}
		size := numberSize(face, 48, x1-x0-32, n.Title, o.th.Pixel)
		o.text(Face(face, size), n.Title, (x0+x1)/2, y0+(y1-y0)*0.28, o.th.Ink)
		if result := results[n.ID]; result != "" {
			size := numberSize(o.th.Fonts.Mono, 36, x1-x0-32, result, false)
			o.text(Face(o.th.Fonts.Mono, size), result, (x0+x1)/2, y0+(y1-y0)*0.7, o.th.OK)
		}
	}
	if active > 0 && d.Steps[active-1].Target != d.Steps[active].Target {
		at := o.tl.MustResolve(d.Steps[active].At)
		end := at + 0.45
		if active+1 < len(d.Steps) {
			end = math.Min(end, o.tl.MustResolve(d.Steps[active+1].At))
		}
		if o.t < end {
			a, b := diagramEnds(nodes[d.Steps[active-1].Target], nodes[d.Steps[active].Target])
			u := prog(o.t, at, end)
			o.dc.SetColor(o.th.Accent)
			o.dc.DrawCircle(a[0]+(b[0]-a[0])*u, a[1]+(b[1]-a[1])*u, 11)
			o.dc.Fill()
		}
	}
	if d.Axis != nil {
		o.diagramAxis(d, active)
	}
}

func diagramEnds(from, to scene.DiagramNode) ([2]float64, [2]float64) {
	ax, ay := (from.Box[0]+from.Box[2])/2, (from.Box[1]+from.Box[3])/2
	bx, by := (to.Box[0]+to.Box[2])/2, (to.Box[1]+to.Box[3])/2
	edge := func(n scene.DiagramNode, x, y, dx, dy float64) [2]float64 {
		if dx == 0 && dy == 0 {
			return [2]float64{x, y}
		}
		u := math.Min((n.Box[2]-n.Box[0])/2/math.Abs(dx), (n.Box[3]-n.Box[1])/2/math.Abs(dy))
		return [2]float64{x + dx*u, y + dy*u}
	}
	return edge(from, ax, ay, bx-ax, by-ay), edge(to, bx, by, ax-bx, ay-by)
}

func (o overlay) diagramAxis(d *scene.Diagram, active int) {
	a := d.Axis
	first := o.tl.MustResolve(d.Steps[0].At)
	last := o.tl.MustResolve(d.Steps[len(d.Steps)-1].At)
	xAt := func(t float64) float64 {
		if first == last {
			return a.At[0]
		}
		return a.At[0] + a.Width*prog(t, first, last)
	}
	y := a.At[1]
	o.dc.SetColor(o.th.CodeDim)
	o.dc.SetLineWidth(4)
	o.dc.DrawLine(a.At[0], y, a.At[0]+a.Width, y)
	o.dc.Stroke()
	if active >= 0 {
		o.dc.SetColor(o.th.Accent)
		o.dc.DrawLine(a.At[0], y, xAt(o.t), y)
		o.dc.Stroke()
		o.dc.DrawCircle(xAt(o.t), y, 10)
		o.dc.Fill()
	}
	for i, step := range d.Steps {
		at := o.tl.MustResolve(step.At)
		x := xAt(at)
		col := o.th.CodeDim
		if i <= active {
			col = o.th.OK
		}
		o.dc.SetColor(col)
		o.dc.DrawLine(x, y-14, x, y+14)
		o.dc.Stroke()
		o.text(Face(o.th.Fonts.Mono, 24), fmt.Sprintf("%.1f с", at-first), x, y+40, col)
		if step.Tick != "" {
			width := a.Width
			if i > 0 {
				width = math.Min(width, x-xAt(o.tl.MustResolve(d.Steps[i-1].At)))
			}
			if i+1 < len(d.Steps) {
				width = math.Min(width, xAt(o.tl.MustResolve(d.Steps[i+1].At))-x)
			}
			size := numberSize(o.th.Fonts.Mono, 28, math.Max(1, width-16), step.Tick, false)
			o.text(Face(o.th.Fonts.Mono, size), step.Tick, x, y+80, col)
		}
	}
}
