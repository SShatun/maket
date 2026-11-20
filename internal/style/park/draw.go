package park

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"
	"strconv"
	"strings"

	xdraw "golang.org/x/image/draw"

	"maket/internal/render"
)

type layer struct {
	y   float64
	pos image.Point
	im  image.Image
}

type layers struct{ world, top []layer }

func (l *layers) anchor(im *image.RGBA, pos point) {
	p := pos.pixel()
	p = p.Sub(image.Pt(im.Bounds().Dx()/2, im.Bounds().Dy()-1))
	l.world = append(l.world, layer{pos[1], p, im})
}

func blit(dst *image.RGBA, cam image.Point, layers []layer) {
	for _, l := range layers {
		origin := l.pos.Sub(cam)
		if src, ok := l.im.(*image.NRGBA); ok {
			blendStatic(dst, origin, src)
		} else {
			draw.Draw(dst, l.im.Bounds().Add(origin), l.im, image.Point{}, draw.Over)
		}
	}
}

func blendStatic(dst *image.RGBA, origin image.Point, src *image.NRGBA) {
	r := src.Bounds().Add(origin).Intersect(dst.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		s := src.PixOffset(r.Min.X-origin.X, y-origin.Y)
		d := dst.PixOffset(r.Min.X, y)
		for x := r.Min.X; x < r.Max.X; x++ {
			alpha := int(src.Pix[s+3])
			if alpha == 255 {
				copy(dst.Pix[d:d+3], src.Pix[s:s+3])
			} else if alpha > 0 {
				for c := 0; c < 3; c++ {
					dst.Pix[d+c] = uint8((int(src.Pix[s+c])*alpha + int(dst.Pix[d+c])*(255-alpha) + 127) / 255)
				}
			}
			s += 4
			d += 4
		}
	}
}

func (s *Style) Draw(dst *image.RGBA, t float64) {
	i := sort.Search(len(s.shots), func(i int) bool { return s.shots[i].at > t }) - 1
	sh := s.shots[max(i, 0)]
	cam := sh.cameraAt(t).pixel()
	frame := image.NewRGBA(image.Rect(0, 0, render.W/scale, render.H/scale))
	draw.Draw(frame, frame.Bounds(), s.images[s.world.Ground], cam, draw.Src)
	l := layers{world: append([]layer(nil), s.static...)}
	for _, o := range sh.objects {
		sh.drawObject(&l, o, t)
	}
	for _, p := range sh.people {
		if valueAt(p.hidden, t) == "" {
			sh.drawPerson(&l, p, t)
		}
	}
	for _, signal := range sh.signals {
		if t >= signal.at && t < signal.at+.25 {
			l.top = append(l.top, layer{signal.gate[1], signal.gate.pixel().Sub(image.Pt(12, 34)), s.images["fx/icon_ok_0.png"]})
		}
	}
	sort.SliceStable(l.world, func(i, j int) bool { return l.world[i].y < l.world[j].y })
	blit(frame, cam, l.world)
	blit(frame, cam, l.top)
	xdraw.NearestNeighbor.Scale(dst, dst.Bounds(), frame, frame.Bounds(), draw.Src, nil)
	for _, title := range sh.titles {
		if t >= title.at && t <= title.until {
			y := -300 + 399*min(1, (t-title.at)/.3)
			draw.Draw(dst, title.im.Bounds().Add(image.Pt(90, scale*int(math.Round(y/scale)))), title.im, image.Point{}, draw.Over)
		}
	}
	for _, c := range s.captions {
		if t >= c.at && t < c.end {
			c.draw(dst, t)
			break
		}
	}
	for _, c := range sh.cuts {
		if t >= c.at && t < c.at+c.dur {
			fade(dst, 1-smooth((t-c.at)/c.dur), c.white)
		}
	}
	if t > s.duration-.5 {
		fade(dst, smooth((t-(s.duration-.5))/.5), false)
	}
}

func fade(dst *image.RGBA, k float64, white bool) {
	v := uint8(0)
	if white {
		v = 255
	}
	col := color.NRGBA{v, v, v, uint8(math.Round(255 * k))}
	draw.Draw(dst, dst.Bounds(), image.NewUniform(col), image.Point{}, draw.Over)
}

func (sh *shot) drawPerson(l *layers, p *person, t float64) {
	leg, pos := p.pose(t)
	feet := pos.pixel()
	pose := valueAt(p.poses, t)
	if pose == "" {
		pose = "stand"
	}
	if t < leg.end && leg.from != leg.to {
		pose = "walk"
	}
	idx := 0
	switch pose {
	case "walk":
		idx = int((t+p.phase)*8) % 4
	case "jump", "cry":
		idx = int((t+p.phase)*4) % 2
	}
	frames := sh.sprites.People[p.kind].Frames
	f := frames[fmt.Sprintf("%s_%s_%d", leg.view, pose, idx)]
	if f.PNG == "" {
		f = frames[leg.view+"_stand_0"]
	}
	im := sh.images[f.PNG]
	anchor := func(p point) image.Point {
		if leg.flip {
			p[0] = float64(im.Bounds().Dx()*scale-1) - p[0]
		}
		return p.pixel()
	}
	origin := feet.Sub(anchor(f.Feet))
	shadow := sh.images["fx/shadow_person.png"]
	l.world = append(l.world, layer{pos[1], feet.Sub(shadow.Bounds().Size().Div(2)), shadow})
	if leg.flip {
		im = sh.mirrors[f.PNG]
	}
	l.world = append(l.world, layer{pos[1], origin, im})
	if name := valueAt(p.items, t); name != "" {
		item := sh.sprites.Items[name]
		l.world = append(l.world, layer{pos[1], origin.Add(anchor(f.Hand)).Add(item.Offset.pixel()), sh.images[item.PNG]})
	}
	head := origin.Add(anchor(f.Head))
	if icon := valueAt(p.icons, t); icon != "" {
		n := 1
		if icon == "wait" {
			n = 2
		} else if icon == "dots" {
			n = 3
		}
		im := sh.images[fmt.Sprintf("fx/icon_%s_%d.png", icon, int((t+p.phase)*4)%n)]
		l.top = append(l.top, layer{pos[1], head.Sub(image.Pt(1, im.Bounds().Dy())), im})
	}
	for _, bubble := range p.bubbles {
		if t < bubble.at || t >= bubble.end {
			continue
		}
		im := sh.bubble(bubble.value)
		l.top = append(l.top, layer{pos[1], head.Add(bubble.offset.pixel()).Sub(image.Pt(2, im.Bounds().Dy())), im})
	}
}

func (sh *shot) drawObject(l *layers, o *object, t float64) {
	kind, state := o.kind, o.state(t)
	n := o.occupied(t, false)
	if o.seats > 0 {
		spec := sh.sprites.Rides[kind]
		phase := 0
		if state == "running" && spec.Phases > 0 {
			phase = int(t*spec.FPS) % spec.Phases
		}
		switch kind {
		case "carousel":
			l.anchor(sh.images[fmt.Sprintf("rides/carousel/phase%d_riders%d.png", phase, n)], o.at)
		case "ferris":
			l.anchor(sh.images[fmt.Sprintf("rides/ferris/phase%d_occ%d.png", phase, 3*((n+1)/3))], o.at)
		case "coaster":
			u := sh.railPhase(o.runningTime(t))
			for i := 0; i < 3; i++ {
				load := "empty"
				if n > i*2 {
					load = "full"
				}
				l.anchor(sh.images[fmt.Sprintf("rides/coaster/car%d_%s.png", i, load)], sh.railPoint(u-float64(i)*spec.Gap))
			}
		}
		if state == "broken" {
			l.anchor(sh.images[fmt.Sprintf("fx/smoke_%d.png", int(t*4)%3)], point{o.at[0] + 36, o.at[1]})
			l.anchor(sh.images[fmt.Sprintf("fx/spark_%d.png", int(t*6)%2)], point{o.at[0] + 30, o.at[1] + 3})
			l.anchor(sh.images["fx/plate_РЕМОНТ.png"], point{o.at[0] - 60, o.at[1] + 6})
		}
		if state == "broken" || state == "closed" {
			l.anchor(sh.images["fx/chain_closed.png"], point{o.at[0], o.at[1] + 54})
		}
		return
	}
	value := valueAt(o.values, t)
	tone := state
	if o.link != nil && value == "" {
		taken := o.link.occupied(t, false)
		free := o.link.seats - taken
		switch kind {
		case "free":
			value = strconv.Itoa(free)
			if tone == "" {
				tone = freeTone(free)
			}
		case "seats6", "seats8":
			value = strconv.Itoa(taken)
		case "light":
			if tone == "" {
				state = "green"
				if free == 0 {
					state = "red"
				}
			}
		}
	}
	var im *image.RGBA
	switch kind {
	case "free", "wait", "inpark", "parking", "ready":
		if tone == "" {
			tone = "ok"
		}
		im = sh.labels[label{kind, value, tone}]
	case "seats6", "seats8":
		if value == "" {
			value = "0"
		}
		im = sh.images["boards/"+kind+"_"+value+".png"]
	case "light", "validator", "junction", "speaker":
		if state == "" {
			state = map[string]string{"light": "green", "validator": "ok", "junction": "on", "speaker": "off"}[kind]
		}
		if kind == "speaker" && state == "on" {
			state = fmt.Sprintf("on_%d", int(t*4)%3)
		}
		im = sh.images["boards/"+kind+"_"+state+".png"]
	case "gate":
		return
	default:
		key := kind
		if strings.HasPrefix(key, "smoke_") {
			key = fmt.Sprintf("smoke_%d", (int(t*4)+int(key[len(key)-1]-'0'))%3)
		}
		if strings.HasPrefix(key, "spark_") {
			key = fmt.Sprintf("spark_%d", int(t*6)%2)
		}
		im = sh.images["fx/"+key+".png"]
	}
	if im != nil {
		l.anchor(im, o.at)
	}
}

func freeTone(free int) string {
	if free == 0 {
		return "bad"
	}
	if free <= 2 {
		return "warn"
	}
	return "ok"
}
