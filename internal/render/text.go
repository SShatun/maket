package render

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

type Fonts struct {
	Bold, Semi, Mono, MonoBold, Pixel *opentype.Font
}

func LoadFonts(dir string) (*Fonts, error) {
	var f Fonts
	for _, x := range []struct {
		dst  **opentype.Font
		file string
	}{
		{&f.Bold, "Inter-Bold.otf"},
		{&f.Semi, "Inter-SemiBold.otf"},
		{&f.Mono, "jbm500.ttf"},
		{&f.MonoBold, "jbm700.ttf"},
		{&f.Pixel, "PressStart2P-Regular.ttf"},
	} {
		ft, err := LoadFont(filepath.Join(dir, x.file))
		if err != nil {
			return nil, err
		}
		*x.dst = ft
	}
	return &f, nil
}

func LoadFont(path string) (*opentype.Font, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := opentype.Parse(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return f, nil
}

func Face(f *opentype.Font, size float64) font.Face {
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		panic(err)
	}
	return face
}
