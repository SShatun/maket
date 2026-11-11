package audio

type Cue struct {
	T    float64
	Kind string
	Gain float64
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
)
