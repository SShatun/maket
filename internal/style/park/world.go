package park

import (
	"encoding/json"
	"fmt"
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	xdraw "golang.org/x/image/draw"

	"maket/internal/charkit"
	"maket/internal/style/sprites"
)

const scale = 3

type frame struct {
	PNG        string
	Feet, Head point
	Hand       point `json:"hand_r"`
}

type boardSpec struct {
	PNG, Title string
	Centre     point `json:"value_centre"`
	Colors     map[string][3]uint8
}

type slot struct {
	X, Y float64
	View string
	Flip bool
}

type line struct {
	Slots []slot
	Gate  *point
}

type reference struct {
	Kind   string
	Anchor point
	Flip   bool
	Call   [3]json.RawMessage
}

type art struct {
	world struct {
		Ground  string
		Objects []struct {
			Sprite string
			Pos    point
			SortY  float64 `json:"sort_y"`
		}
		Reference []reference `json:"dynamic_in_reference"`
	}
	cameras struct{ Presets map[string]point }
	queues  map[string]*line
	paths   struct {
		Nodes   map[string]point
		Edges   [][2]string
		Coaster struct{ Points [][3]float64 }
	}
	sprites struct {
		People map[string]struct {
			Frames map[string]frame
			Role   string
			Kid    bool
		}
		Items map[string]struct {
			PNG    string
			Offset point `json:"offset_from_hand"`
		}
		Boards map[string]boardSpec
		Rides  map[string]struct {
			Seats, Phases int
			FPS           float64 `json:"fps_running"`
			Gap           float64 `json:"car_gap_t"`
		}
	}
	images, mirrors map[string]*image.RGBA
	guests          []string
	guestsByAge     map[bool][]string
	nodes           []string
	railTimes       []float64
	static          []layer
}

func loadArt(dir string) (*art, error) {
	a := &art{images: map[string]*image.RGBA{}, mirrors: map[string]*image.RGBA{}}
	for _, entry := range []struct {
		name string
		dest any
	}{
		{"world", &a.world}, {"sprites", &a.sprites}, {"queues", &a.queues}, {"paths", &a.paths}, {"cameras", &a.cameras},
	} {
		b, err := os.ReadFile(filepath.Join(dir, entry.name+".json"))
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, entry.dest); err != nil {
			return nil, fmt.Errorf("%s.json: %w", entry.name, err)
		}
	}
	for id, person := range a.sprites.People {
		if person.Role == "" {
			a.guests = append(a.guests, id)
		}
	}
	sort.Strings(a.guests)
	a.guestsByAge = make(map[bool][]string)
	for _, id := range a.guests {
		kid := a.sprites.People[id].Kid
		a.guestsByAge[kid] = append(a.guestsByAge[kid], id)
	}
	statics := map[string]*image.NRGBA{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if filepath.Ext(path) != ".png" {
			return nil
		}
		key, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		key = filepath.ToSlash(key)
		if strings.HasPrefix(key, "world/") && !strings.HasPrefix(key, "world/objects/") && key != a.world.Ground {
			return nil
		}
		src, err := charkit.ReadPNG(path)
		if err != nil {
			return err
		}
		sz := src.Bounds().Size()
		bounds := image.Rect(0, 0, sz.X/scale, sz.Y/scale)
		if strings.HasPrefix(key, "world/objects/") {
			im := image.NewNRGBA(bounds)
			for y := 0; y < bounds.Dy(); y++ {
				for x := 0; x < bounds.Dx(); x++ {
					im.Set(x, y, src.At(x*scale, y*scale))
				}
			}
			statics[key] = im
			return nil
		}
		im := image.NewRGBA(bounds)
		xdraw.NearestNeighbor.Scale(im, im.Bounds(), src, src.Bounds(), xdraw.Src, nil)
		a.images[key] = im
		if strings.HasPrefix(key, "people/") {
			a.mirrors[key] = sprites.Mirror(im)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if a.images[a.world.Ground] == nil {
		return nil, fmt.Errorf("missing park ground %q", a.world.Ground)
	}
	for _, o := range a.world.Objects {
		im := statics[o.Sprite]
		if im == nil {
			return nil, fmt.Errorf("missing park object %q", o.Sprite)
		}
		a.static = append(a.static, layer{o.SortY, o.Pos.pixel(), im})
	}
	for k := range a.paths.Nodes {
		a.nodes = append(a.nodes, k)
	}
	sort.Strings(a.nodes)
	a.prepareRail()
	return a, nil
}
