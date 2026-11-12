package moe

import (
	"image"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"maket/internal/render"
)

type fallbackFace struct {
	font.Face
	buffer   sfnt.Buffer
	source   *opentype.Font
	fallback font.Face
}

func newFace(f, fallback *opentype.Font, size float64) font.Face {
	primary := render.Face(f, size)
	if fallback == nil {
		return primary
	}
	return &fallbackFace{Face: primary, source: f, fallback: render.Face(fallback, size)}
}

func (f *fallbackFace) face(r rune) font.Face {
	i, err := f.source.GlyphIndex(&f.buffer, r)
	if err != nil || i == 0 {
		return f.fallback
	}
	return f.Face
}

func (f *fallbackFace) Glyph(dot fixed.Point26_6, r rune) (image.Rectangle, image.Image, image.Point, fixed.Int26_6, bool) {
	return f.face(r).Glyph(dot, r)
}

func (f *fallbackFace) GlyphBounds(r rune) (fixed.Rectangle26_6, fixed.Int26_6, bool) {
	return f.face(r).GlyphBounds(r)
}

func (f *fallbackFace) GlyphAdvance(r rune) (fixed.Int26_6, bool) {
	return f.face(r).GlyphAdvance(r)
}

func (f *fallbackFace) Kern(a, b rune) fixed.Int26_6 {
	first, second := f.face(a), f.face(b)
	if first != second {
		return 0
	}
	return first.Kern(a, b)
}

func (f *fallbackFace) Close() error {
	err := f.Face.Close()
	other := f.fallback.Close()
	if err != nil {
		return err
	}
	return other
}
