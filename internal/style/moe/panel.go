package moe

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"

	"maket/internal/render"
	"maket/internal/scene"
)

type panelRow struct {
	at    float64
	image *image.RGBA
}

type panel struct {
	id, title, role     string
	at, until, entrance float64
	rect                [4]float64
	blink               int
	size                float64
	lineHeight          int
	lines               []scene.PanelLine
	rows                []panelRow
	shell, dark, glow   *image.RGBA
}

type title struct {
	at, until   float64
	image, glow *image.RGBA
}

func (s *Style) makePanel(ev scene.Event, at, until float64, previous map[string]*panel) (*panel, error) {
	var opt struct {
		ID    string    `json:"id"`
		Rect  []float64 `json:"rect"`
		Blink *int      `json:"blink"`
		Title *string   `json:"title"`
		Role  *string   `json:"role"`
		Size  *float64  `json:"size"`
	}
	if err := raw(ev.Raw, &opt); err != nil {
		return nil, err
	}
	p := &panel{id: opt.ID, at: at, until: until, entrance: at, title: ev.Title, role: ev.Role, blink: -1, size: 25, lines: ev.Lines}
	rect := opt.Rect
	prev := previous[opt.ID]
	if prev != nil && opt.ID != "" {
		if len(rect) == 0 {
			rect = prev.rect[:]
		}
		if opt.Title == nil {
			p.title = prev.title
		}
		if opt.Role == nil {
			p.role = prev.role
		}
		if ev.Lines == nil {
			p.lines = prev.lines
		}
		p.blink = prev.blink
		p.size = prev.size
		if at < prev.until {
			p.entrance = prev.entrance
			prev.until = at
		}
	}
	if len(rect) != 4 || rect[0] < 0 || rect[1] < 0 || rect[2] > render.W || rect[3] > render.H || rect[2]-rect[0] < 120 || rect[3]-rect[1] < 80 {
		return nil, fmt.Errorf("holo needs rect: [x0, y0, x1, y1] within 1080x1920, at least 120x80")
	}
	copy(p.rect[:], rect)
	if p.role == "" {
		p.role = "storage"
	}
	col, ok := s.roles[p.role]
	if !ok {
		return nil, fmt.Errorf("holo: unknown role %q", p.role)
	}
	if opt.Blink != nil {
		p.blink = *opt.Blink
	}
	if p.blink < -1 || p.blink >= len(p.lines) {
		return nil, fmt.Errorf("holo blink is a zero-based line index, or -1 to stop blinking")
	}
	w, h := int(math.Ceil(rect[2]-rect[0])), int(math.Ceil(rect[3]-rect[1]))
	if opt.Size != nil {
		p.size = *opt.Size
	}
	if p.size <= 0 || p.size > float64(h) {
		return nil, fmt.Errorf("holo size must be positive and fit the panel height")
	}
	p.lineHeight = int(math.Ceil(p.size * 1.6))
	if len(p.lines) > 0 && 69+len(p.lines)*p.lineHeight > h-10 {
		return nil, fmt.Errorf("holo %q: %d lines do not fit height %d", p.id, len(p.lines), h)
	}
	const pad = 28
	p.shell = image.NewRGBA(image.Rect(0, 0, w+2*pad, h+2*pad))
	dc := gg.NewContextForRGBA(p.shell)
	dc.Translate(pad, pad)
	dc.SetRGBA255(int(col.R), int(col.G), int(col.B), 38)
	dc.DrawRoundedRectangle(0, 0, float64(w), float64(h), 14)
	dc.Fill()
	dc.SetRGBA255(int(col.R), int(col.G), int(col.B), 14)
	dc.SetLineWidth(1)
	for y := 6; y < h-6; y += 6 {
		dc.DrawLine(4, float64(y), float64(w-4), float64(y))
		dc.Stroke()
	}
	dc.SetRGBA255(int(col.R), int(col.G), int(col.B), 230)
	dc.SetLineWidth(3)
	dc.DrawRoundedRectangle(0, 0, float64(w), float64(h), 14)
	dc.Stroke()
	for _, pt := range [][2]float64{{0, 0}, {float64(w), 0}, {0, float64(h)}, {float64(w), float64(h)}} {
		dc.DrawCircle(pt[0], pt[1], 5)
		dc.Fill()
	}
	if p.title != "" {
		face, err := fitFace(s.theme.Fonts.Bold, nil, 26, float64(w-44), p.title)
		if err != nil {
			return nil, fmt.Errorf("holo title: %w", err)
		}
		dc.SetFontFace(face)
		dc.SetColor(s.theme.Accent)
		dc.DrawStringAnchored(p.title, 22, 28, 0, 0.35)
		face.Close()
		dc.SetRGBA255(int(col.R), int(col.G), int(col.B), 180)
		dc.SetLineWidth(1)
		dc.DrawLine(0, 52, float64(w), 52)
		dc.Stroke()
	}
	p.dark = image.NewRGBA(p.shell.Bounds())
	dark := gg.NewContextForRGBA(p.dark)
	dark.SetRGBA255(14, 18, 36, 200)
	dark.DrawRoundedRectangle(pad, pad, float64(w), float64(h), 14)
	dark.Fill()
	for _, line := range p.lines {
		c := col
		switch line.Tone {
		case "":
		case "ink":
			c = s.theme.Ink
		case "accent":
			c = s.theme.Accent
		case "error":
			c = s.theme.Error
		case "ok":
			c = color.RGBA{120, 255, 150, 255}
		default:
			return nil, fmt.Errorf("holo: unknown line tone %q", line.Tone)
		}
		t := at
		if line.Appear != nil {
			var err error
			t, err = s.tl.Resolve(*line.Appear)
			if err != nil {
				return nil, err
			}
		}
		im := image.NewRGBA(image.Rect(0, 0, w-44, p.lineHeight))
		row := gg.NewContextForRGBA(im)
		face, err := fitFace(s.theme.Fonts.Mono, s.theme.Fonts.Semi, p.size, float64(w-44), line.Text)
		if err != nil {
			return nil, fmt.Errorf("holo line: %w", err)
		}
		row.SetFontFace(face)
		row.SetColor(c)
		row.DrawStringAnchored(line.Text, 0, float64(p.lineHeight)/2, 0, 0.35)
		face.Close()
		p.rows = append(p.rows, panelRow{at: t, image: im})
	}
	p.glow = bloom(p.shell, 8, 0.7)
	if opt.ID != "" {
		previous[opt.ID] = p
	}
	return p, nil
}

func (s *Style) makeTitle(ev scene.Event, at, until float64) (title, error) {
	v := title{at: at, until: until}
	var opt struct {
		Episode *int `json:"episode"`
	}
	if err := raw(ev.Raw, &opt); err != nil {
		return v, err
	}
	if len(ev.Lines) < 1 || len(ev.Lines) > 2 {
		return v, fmt.Errorf("title needs one or two lines")
	}
	v.image = image.NewRGBA(image.Rect(0, 0, render.W, 700))
	dc := gg.NewContextForRGBA(v.image)
	dc.SetColor(s.theme.Ink)
	if opt.Episode != nil {
		episode := strconv.Itoa(*opt.Episode)
		face, err := fitFace(s.theme.Fonts.Bold, nil, 34, 900, "ЭПИЗОД "+episode)
		if err != nil {
			return v, err
		}
		dc.SetFontFace(face)
		dc.DrawStringAnchored("ЭПИЗОД "+episode, 540, 300, 0.5, 0.35)
		face.Close()
		dc.SetLineWidth(2)
		dc.DrawLine(400, 336, 680, 336)
		dc.Stroke()
	}
	size := 96.0
	for _, line := range ev.Lines {
		face := render.Face(s.theme.Fonts.Bold, size)
		width := float64(font.MeasureString(face, line.Text).Round())
		face.Close()
		if width > 920 {
			size = math.Min(size, size*920/width)
		}
		if line.Appear != nil || line.Tone != "" {
			return v, fmt.Errorf("title lines must be plain text")
		}
	}
	if size < 92 {
		return v, fmt.Errorf("title lines are too long for 92px text; split into two shorter lines")
	}
	face := render.Face(s.theme.Fonts.Bold, size)
	dc.SetFontFace(face)
	for i, line := range ev.Lines {
		dc.DrawStringAnchored(line.Text, 540, 450+115*float64(i), 0.5, 0.35)
	}
	face.Close()
	v.glow = bloom(v.image, 10, 0.5)
	return v, nil
}

func fitFace(f, fallback *opentype.Font, size, width float64, text string) (font.Face, error) {
	if strings.ContainsAny(text, "\n\r") {
		return nil, fmt.Errorf("use a separate entry for each line")
	}
	face := newFace(f, fallback, size)
	w := float64(font.MeasureString(face, text).Round())
	if w <= width {
		return face, nil
	}
	face.Close()
	size *= width / w
	if size < 18 {
		return nil, fmt.Errorf("text is too wide; shorten it or widen the panel")
	}
	return newFace(f, fallback, size), nil
}

func bloom(src *image.RGBA, sigma, gain float64) *image.RGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewRGBA(src.Bounds())
	for c := 0; c < 3; c++ {
		values := make([]float32, w*h)
		for i := range values {
			values[i] = float32(src.Pix[i*4+c])
		}
		values = blur(values, w, h, sigma)
		for i, v := range values {
			dst.Pix[i*4+c] = uint8(math.Max(0, math.Min(255, float64(v)*gain)))
		}
	}
	for i := 3; i < len(dst.Pix); i += 4 {
		dst.Pix[i] = 255
	}
	return dst
}
