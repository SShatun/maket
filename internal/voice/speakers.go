package voice

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"maket/internal/scene"
)

func phraseVoices(sc *scene.Scene, fallback string) ([]string, error) {
	voices := map[string]string{}
	roles := map[string]string{}
	if sc.Style == "paper" || sc.Style == "noir" {
		entities := append([]scene.Entity(nil), sc.Entities...)
		for _, shot := range sc.Shots {
			for _, cast := range shot.Cast {
				entities = append(entities, cast.Entity)
			}
		}
		for _, entity := range entities {
			if entity.Kind != "" {
				roles[entity.ID] = entity.Kind
			}
			if entity.Voice == "" {
				continue
			}
			if previous := voices[entity.ID]; previous != "" && previous != entity.Voice {
				return nil, fmt.Errorf("%s: speaker %q has conflicting voices %q and %q", sc.Style, entity.ID, previous, entity.Voice)
			}
			voices[entity.ID] = entity.Voice
		}
	}
	names := make([]string, len(sc.Phrases))
	for i, phrase := range sc.Phrases {
		names[i] = fallback
		if phrase.Speaker == "" {
			continue
		}
		if voice := voices[phrase.Speaker]; voice != "" {
			names[i] = voice
		} else if sc.Style == "noir" {
			names[i] = "noir-suspect"
			if cmp.Or(roles[phrase.Speaker], phrase.Speaker) == "detective" {
				names[i] = "noir-detective"
			}
		}
	}
	return names, nil
}

func checkLibraryVoices(root string, cfg Config, sc *scene.Scene, name string) error {
	if cfg.Backend == "fish" {
		return nil
	}
	if name == "" {
		name = cfg.Voice
	}
	names, err := phraseVoices(sc, name)
	if err != nil {
		return err
	}
	for _, name := range names {
		b, err := os.ReadFile(filepath.Join(root, "voices", name, "voice.json"))
		if err != nil {
			continue
		}
		var spec Spec
		if json.Unmarshal(b, &spec) == nil && spec.ReferenceID != "" && spec.RefAudio == "" {
			return fmt.Errorf("voice %q exists only in Fish; select the fish voice backend", name)
		}
	}
	return nil
}
