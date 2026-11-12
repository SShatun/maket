package charkit

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
)

var themeKeys = map[string]bool{"bg": true, "ink": true, "card": true, "muted": true, "accent": true, "error": true, "ok": true, "grid": true, "dot": true}

type Look struct {
	About     string
	Voice     string
	Theme     map[string]color.RGBA
	Roles     map[string]color.RGBA
	Palette   map[string]color.RGBA
	Backdrops map[string]Backdrop
}

type Backdrop struct {
	File  string   `json:"file"`
	Floor *float64 `json:"floor"`
}

func ReadLook(dir string) (Look, error) {
	lk := Look{Theme: map[string]color.RGBA{}, Roles: map[string]color.RGBA{}}
	path := filepath.Join(dir, "look.json")
	if _, err := os.Stat(path); err != nil {
		return lk, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Look{}, err
	}
	var f struct {
		About     string              `json:"about"`
		Voice     string              `json:"voice"`
		Theme     map[string][3]uint8 `json:"theme"`
		Roles     map[string][3]uint8 `json:"roles"`
		Palette   map[string][3]uint8 `json:"palette"`
		Backdrops map[string]Backdrop `json:"backdrops"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return Look{}, fmt.Errorf("%s/look.json: %w", dir, err)
	}
	for k, v := range f.Theme {
		if !themeKeys[k] {
			return Look{}, fmt.Errorf("%s/look.json: unknown theme colour %q", dir, k)
		}
		lk.Theme[k] = color.RGBA{v[0], v[1], v[2], 255}
	}
	for k, v := range f.Roles {
		lk.Roles[k] = color.RGBA{v[0], v[1], v[2], 255}
	}
	lk.About, lk.Voice, lk.Backdrops = f.About, f.Voice, f.Backdrops
	lk.Palette = map[string]color.RGBA{}
	for k, v := range f.Palette {
		lk.Palette[k] = color.RGBA{v[0], v[1], v[2], 255}
	}
	return lk, nil
}
