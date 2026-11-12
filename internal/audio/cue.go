package audio

type Cue struct {
	T    float64
	Kind string
	Gain float64
	Dur  float64
}

const (
	Whoosh = "whoosh"
	Land   = "land"
	Rise   = "rise"
	Thud   = "thud"
	Buzz   = "buzz"
	Pop    = "pop"
	Chime  = "chime"

	ChipTitle    = "title"
	ChipAppear   = "appear"
	ChipSay      = "say"
	ChipMove     = "move"
	ChipBlock    = "block"
	ChipQuestion = "question"
	ChipTick     = "tick"
	ChipOK       = "ok"
	ChipStrike   = "strike"

	PadSunset = "pad-sunset"
	PadNight  = "pad-night"
	PadIdea   = "pad-idea"
	PadSakura = "pad-sakura"
	MusicBox  = "musicbox"
	Meow      = "meow"
	Heartbeat = "heartbeat"
	Purr      = "purr"
	Stab      = "stab"
	Blip      = "blip"
	Sparkle   = "sparkle"
	Drone     = "drone"
	DroneLow  = "drone-low"
	Pluck     = "pluck"
	Drums     = "drums"
	Horn      = "horn"
	Crackle   = "crackle"
	Clunk     = "clunk"
)
