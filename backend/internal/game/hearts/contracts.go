package hearts

// Variant identifies Hearts scoring rule sets supported by the future Rust engine.
type Variant string

const (
	VariantStandard Variant = "standard"
	VariantOmnibus  Variant = "omnibus"
)

// RoundPlayer captures round scoring inputs in seat order.
type RoundPlayer struct {
	HeartsTaken    int  `json:"hearts_taken"`
	QueenOfSpades  bool `json:"queen_of_spades"`
	JackOfDiamonds bool `json:"jack_of_diamonds"`
}

// ScoreRoundRequest is the boundary contract for Go -> Hearts engine scoring.
type ScoreRoundRequest struct {
	Variant Variant       `json:"variant"`
	Players []RoundPlayer `json:"players"`
}

// ScoreRoundResponse is the boundary contract for Hearts score results.
type ScoreRoundResponse struct {
	Scores       []int `json:"scores"`
	ShooterIndex *int  `json:"shooter_index,omitempty"`
}

// Scorer defines the future integration seam (FFI/service) for Hearts logic.
type Scorer interface {
	ScoreRound(req ScoreRoundRequest) (ScoreRoundResponse, error)
}
