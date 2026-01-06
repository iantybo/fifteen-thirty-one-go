package cribbage

// Rules captures configurable cribbage rules for 2-4 players.
type Rules struct {
	MaxPlayers int `json:"max_players"` // 2-4
}

func DefaultRules(players int) Rules {
	if players < 2 {
		players = 2
	}
	if players > 4 {
		players = 4
	}
	return Rules{MaxPlayers: players}
}

func (r Rules) HandSize() int {
	switch r.MaxPlayers {
	case 2:
		return 6
	case 3, 4:
		return 5
	default:
		return 6
	}
}

func (r Rules) DiscardCount() int {
	switch r.MaxPlayers {
	case 2:
		return 2
	case 3, 4:
		return 1
	default:
		return 2
	}
}

func (r Rules) CribSize() int {
	return 4
}
