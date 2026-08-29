package models

import (
	"database/sql"
	"errors"
	"time"
)

// TradingCard represents a collectible card that users can earn.
type TradingCard struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Rarity      string    `json:"rarity"`
	ArtworkURL  string    `json:"artwork_url"`
	Category    string    `json:"category"`
	CreatedAt   time.Time `json:"created_at"`
}

// UserTradingCard represents a user's ownership of a trading card.
type UserTradingCard struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	CardID     int64     `json:"card_id"`
	AcquiredAt time.Time `json:"acquired_at"`
	Quantity   int       `json:"quantity"`
}

// CardReward defines the requirements for earning a trading card.
type CardReward struct {
	ID               int64  `json:"id"`
	CardID           int64  `json:"card_id"`
	RewardType       string `json:"reward_type"`
	RequirementValue int    `json:"requirement_value"`
	RequirementData  string `json:"requirement_data,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// UserCardWithDetails combines trading card information with user ownership details.
type UserCardWithDetails struct {
	TradingCard
	Quantity   int       `json:"quantity"`
	AcquiredAt time.Time `json:"acquired_at"`
}

// CardProgress tracks a user's progress toward earning a trading card.
type CardProgress struct {
	Card            TradingCard `json:"card"`
	Unlocked        bool        `json:"unlocked"`
	Progress        int         `json:"progress"`
	RequiredValue   int         `json:"required_value"`
	RewardType      string      `json:"reward_type"`
}

// GetAllTradingCards retrieves all trading cards from the database, sorted by rarity and name.
func GetAllTradingCards(db *sql.DB) ([]TradingCard, error) {
	rows, err := db.Query(`
		SELECT id, name, description, rarity, artwork_url, category, created_at
		FROM trading_cards
		ORDER BY rarity DESC, name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []TradingCard
	for rows.Next() {
		var c TradingCard
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Rarity, &c.ArtworkURL, &c.Category, &c.CreatedAt); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cards, nil
}

// GetTradingCardByID retrieves a single trading card by its ID.
// Returns ErrNotFound if the card does not exist.
func GetTradingCardByID(db *sql.DB, cardID int64) (*TradingCard, error) {
	var c TradingCard
	err := db.QueryRow(`
		SELECT id, name, description, rarity, artwork_url, category, created_at
		FROM trading_cards
		WHERE id = ?
	`, cardID).Scan(&c.ID, &c.Name, &c.Description, &c.Rarity, &c.ArtworkURL, &c.Category, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// GetUserTradingCards retrieves all trading cards owned by a specific user.
// Returns cards with ownership details sorted by acquisition date (most recent first).
func GetUserTradingCards(db *sql.DB, userID int64) ([]UserCardWithDetails, error) {
	rows, err := db.Query(`
		SELECT tc.id, tc.name, tc.description, tc.rarity, tc.artwork_url, tc.category, tc.created_at,
		       utc.quantity, utc.acquired_at
		FROM user_trading_cards utc
		JOIN trading_cards tc ON utc.card_id = tc.id
		WHERE utc.user_id = ?
		ORDER BY utc.acquired_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []UserCardWithDetails
	for rows.Next() {
		var c UserCardWithDetails
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Description, &c.Rarity, &c.ArtworkURL, &c.Category, &c.CreatedAt,
			&c.Quantity, &c.AcquiredAt,
		); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cards, nil
}

// UserHasCard checks whether a user owns a specific trading card.
func UserHasCard(db *sql.DB, userID, cardID int64) (bool, error) {
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS(SELECT 1 FROM user_trading_cards WHERE user_id = ? AND card_id = ?)
	`, userID, cardID).Scan(&exists)
	return exists, err
}

// AddCardToUser adds a trading card to a user's collection.
// If the user already owns the card, increments the quantity.
func AddCardToUser(db *sql.DB, userID, cardID int64) error {
	_, err := db.Exec(`
		INSERT INTO user_trading_cards (user_id, card_id, quantity)
		VALUES (?, ?, 1)
		ON CONFLICT(user_id, card_id) DO UPDATE SET
			quantity = quantity + 1
	`, userID, cardID)
	return err
}

// GetCardRewards retrieves all reward conditions for a specific trading card.
func GetCardRewards(db *sql.DB, cardID int64) ([]CardReward, error) {
	rows, err := db.Query(`
		SELECT id, card_id, reward_type, requirement_value, COALESCE(requirement_data, ''), created_at
		FROM card_rewards
		WHERE card_id = ?
	`, cardID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rewards []CardReward
	for rows.Next() {
		var r CardReward
		if err := rows.Scan(&r.ID, &r.CardID, &r.RewardType, &r.RequirementValue, &r.RequirementData, &r.CreatedAt); err != nil {
			return nil, err
		}
		rewards = append(rewards, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rewards, nil
}

// GetAllCardRewards retrieves all reward conditions for all trading cards.
func GetAllCardRewards(db *sql.DB) ([]CardReward, error) {
	rows, err := db.Query(`
		SELECT id, card_id, reward_type, requirement_value, COALESCE(requirement_data, ''), created_at
		FROM card_rewards
		ORDER BY card_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rewards []CardReward
	for rows.Next() {
		var r CardReward
		if err := rows.Scan(&r.ID, &r.CardID, &r.RewardType, &r.RequirementValue, &r.RequirementData, &r.CreatedAt); err != nil {
			return nil, err
		}
		rewards = append(rewards, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rewards, nil
}

// GetUserCardProgress calculates a user's progress toward earning all trading cards.
// Returns information about each card including whether it's unlocked and current progress.
func GetUserCardProgress(db *sql.DB, userID int64) ([]CardProgress, error) {
	user, err := GetUserByID(db, userID)
	if err != nil {
		return nil, err
	}

	cards, err := GetAllTradingCards(db)
	if err != nil {
		return nil, err
	}

	allRewards, err := GetAllCardRewards(db)
	if err != nil {
		return nil, err
	}

	rewardsByCard := make(map[int64][]CardReward)
	for _, r := range allRewards {
		rewardsByCard[r.CardID] = append(rewardsByCard[r.CardID], r)
	}

	ownedCards := make(map[int64]bool)
	userCards, err := GetUserTradingCards(db, userID)
	if err != nil {
		return nil, err
	}
	for _, uc := range userCards {
		ownedCards[uc.ID] = true
	}

	var progress []CardProgress
	for _, card := range cards {
		rewards := rewardsByCard[card.ID]
		unlocked := ownedCards[card.ID]

		var prog int
		var reqValue int
		var rewardType string

		if len(rewards) > 0 {
			reward := rewards[0]
			rewardType = reward.RewardType
			reqValue = reward.RequirementValue

			switch reward.RewardType {
			case "game_win":
				prog = int(user.GamesWon)
			case "games_played":
				prog = int(user.GamesPlayed)
			case "win_streak":
				prog = 0
			case "high_score":
				prog = 0
			case "special_event":
				prog = 0
			}
		}

		progress = append(progress, CardProgress{
			Card:          card,
			Unlocked:      unlocked,
			Progress:      prog,
			RequiredValue: reqValue,
			RewardType:    rewardType,
		})
	}

	return progress, nil
}
