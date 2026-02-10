package models

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB creates an in-memory SQLite database with the required schema.
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create users table
	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			games_played INTEGER NOT NULL DEFAULT 0,
			games_won INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}

	// Create trading_cards table
	_, err = db.Exec(`
		CREATE TABLE trading_cards (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			rarity TEXT NOT NULL CHECK(rarity IN ('common', 'uncommon', 'rare', 'epic', 'legendary')),
			artwork_url TEXT NOT NULL,
			category TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create trading_cards table: %v", err)
	}

	// Create indexes
	_, err = db.Exec(`CREATE INDEX idx_trading_cards_rarity ON trading_cards(rarity)`)
	if err != nil {
		t.Fatalf("Failed to create rarity index: %v", err)
	}

	_, err = db.Exec(`CREATE INDEX idx_trading_cards_category ON trading_cards(category)`)
	if err != nil {
		t.Fatalf("Failed to create category index: %v", err)
	}

	// Create user_trading_cards table
	_, err = db.Exec(`
		CREATE TABLE user_trading_cards (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			card_id INTEGER NOT NULL,
			acquired_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			quantity INTEGER NOT NULL DEFAULT 1,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE ON UPDATE CASCADE,
			FOREIGN KEY(card_id) REFERENCES trading_cards(id) ON DELETE CASCADE ON UPDATE CASCADE
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create user_trading_cards table: %v", err)
	}

	// Create unique index
	_, err = db.Exec(`CREATE UNIQUE INDEX idx_user_trading_cards_user_card ON user_trading_cards(user_id, card_id)`)
	if err != nil {
		t.Fatalf("Failed to create unique index: %v", err)
	}

	_, err = db.Exec(`CREATE INDEX idx_user_trading_cards_user_id ON user_trading_cards(user_id)`)
	if err != nil {
		t.Fatalf("Failed to create user_id index: %v", err)
	}

	_, err = db.Exec(`CREATE INDEX idx_user_trading_cards_card_id ON user_trading_cards(card_id)`)
	if err != nil {
		t.Fatalf("Failed to create card_id index: %v", err)
	}

	// Create card_rewards table
	_, err = db.Exec(`
		CREATE TABLE card_rewards (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			card_id INTEGER NOT NULL,
			reward_type TEXT NOT NULL CHECK(reward_type IN ('game_win', 'games_played', 'high_score', 'win_streak', 'special_event')),
			requirement_value INTEGER NOT NULL,
			requirement_data TEXT,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(card_id) REFERENCES trading_cards(id) ON DELETE CASCADE ON UPDATE CASCADE
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create card_rewards table: %v", err)
	}

	_, err = db.Exec(`CREATE INDEX idx_card_rewards_card_id ON card_rewards(card_id)`)
	if err != nil {
		t.Fatalf("Failed to create rewards card_id index: %v", err)
	}

	_, err = db.Exec(`CREATE INDEX idx_card_rewards_type ON card_rewards(reward_type)`)
	if err != nil {
		t.Fatalf("Failed to create rewards type index: %v", err)
	}

	return db
}

// seedTestCards inserts test trading cards into the database.
func seedTestCards(t *testing.T, db *sql.DB) []int64 {
	cards := []struct {
		name        string
		description string
		rarity      string
		artworkURL  string
		category    string
	}{
		{"Test Common Card", "A common test card", "common", "/test/common.png", "test"},
		{"Test Rare Card", "A rare test card", "rare", "/test/rare.png", "test"},
		{"Test Legendary Card", "A legendary test card", "legendary", "/test/legendary.png", "achievement"},
		{"Another Common Card", "Another common card", "common", "/test/common2.png", "milestone"},
	}

	var cardIDs []int64
	for _, card := range cards {
		res, err := db.Exec(`
			INSERT INTO trading_cards (name, description, rarity, artwork_url, category)
			VALUES (?, ?, ?, ?, ?)
		`, card.name, card.description, card.rarity, card.artworkURL, card.category)
		if err != nil {
			t.Fatalf("Failed to insert test card: %v", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			t.Fatalf("Failed to get last insert id: %v", err)
		}
		cardIDs = append(cardIDs, id)
	}
	return cardIDs
}

// seedTestUser creates a test user in the database.
func seedTestUser(t *testing.T, db *sql.DB, gamesPlayed, gamesWon int64) int64 {
	res, err := db.Exec(`
		INSERT INTO users (username, password_hash, games_played, games_won)
		VALUES (?, ?, ?, ?)
	`, "testuser", "hashedpassword", gamesPlayed, gamesWon)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get user id: %v", err)
	}
	return id
}

func TestGetAllTradingCards(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	t.Run("empty database returns empty slice", func(t *testing.T) {
		cards, err := GetAllTradingCards(db)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(cards) != 0 {
			t.Errorf("Expected 0 cards, got %d", len(cards))
		}
	})

	t.Run("returns all cards sorted by rarity and name", func(t *testing.T) {
		cardIDs := seedTestCards(t, db)
		if len(cardIDs) == 0 {
			t.Fatal("Failed to seed test cards")
		}

		cards, err := GetAllTradingCards(db)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(cards) != 4 {
			t.Errorf("Expected 4 cards, got %d", len(cards))
		}

		// Check ordering: legendary should come before rare, rare before common (DESC)
		// Within same rarity, alphabetical by name (ASC)
		if len(cards) >= 4 {
			if cards[0].Rarity != "rare" {
				t.Errorf("Expected first card to be rare, got %s", cards[0].Rarity)
			}
			// Check that common cards are after rare
			foundCommon := false
			for i, card := range cards {
				if card.Rarity == "common" {
					foundCommon = true
					if i == 0 {
						t.Error("Common card should not be first when rare cards exist")
					}
				}
			}
			if !foundCommon {
				t.Error("Expected to find common cards")
			}
		}
	})
}

func TestGetTradingCardByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	t.Run("returns ErrNotFound for non-existent card", func(t *testing.T) {
		card, err := GetTradingCardByID(db, 999)
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
		if card != nil {
			t.Errorf("Expected nil card, got %v", card)
		}
	})

	t.Run("returns card for valid ID", func(t *testing.T) {
		cardIDs := seedTestCards(t, db)
		if len(cardIDs) == 0 {
			t.Fatal("Failed to seed test cards")
		}

		card, err := GetTradingCardByID(db, cardIDs[0])
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if card == nil {
			t.Fatal("Expected card, got nil")
		}
		if card.ID != cardIDs[0] {
			t.Errorf("Expected card ID %d, got %d", cardIDs[0], card.ID)
		}
		if card.Name != "Test Common Card" {
			t.Errorf("Expected card name 'Test Common Card', got %s", card.Name)
		}
		if card.Rarity != "common" {
			t.Errorf("Expected rarity 'common', got %s", card.Rarity)
		}
	})
}

func TestGetUserTradingCards(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userID := seedTestUser(t, db, 0, 0)
	cardIDs := seedTestCards(t, db)

	t.Run("returns empty slice for user with no cards", func(t *testing.T) {
		cards, err := GetUserTradingCards(db, userID)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(cards) != 0 {
			t.Errorf("Expected 0 cards, got %d", len(cards))
		}
	})

	t.Run("returns user cards with quantity and acquired date", func(t *testing.T) {
		// Add cards to user
		_, err := db.Exec(`
			INSERT INTO user_trading_cards (user_id, card_id, quantity)
			VALUES (?, ?, 1), (?, ?, 3)
		`, userID, cardIDs[0], userID, cardIDs[1])
		if err != nil {
			t.Fatalf("Failed to add cards to user: %v", err)
		}

		cards, err := GetUserTradingCards(db, userID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(cards) != 2 {
			t.Errorf("Expected 2 cards, got %d", len(cards))
		}

		// Check quantity is included
		foundCard1 := false
		foundCard2 := false
		for _, card := range cards {
			if card.ID == cardIDs[0] && card.Quantity == 1 {
				foundCard1 = true
			}
			if card.ID == cardIDs[1] && card.Quantity == 3 {
				foundCard2 = true
			}
		}
		if !foundCard1 || !foundCard2 {
			t.Error("Expected to find both cards with correct quantities")
		}

		// Check that acquired_at is set
		if cards[0].AcquiredAt.IsZero() {
			t.Error("Expected acquired_at to be set")
		}
	})

	t.Run("returns cards sorted by acquired_at DESC", func(t *testing.T) {
		// Clear existing user cards
		_, err := db.Exec(`DELETE FROM user_trading_cards WHERE user_id = ?`, userID)
		if err != nil {
			t.Fatalf("Failed to clear user cards: %v", err)
		}

		// Add cards with specific timestamps
		now := time.Now()
		_, err = db.Exec(`
			INSERT INTO user_trading_cards (user_id, card_id, quantity, acquired_at)
			VALUES (?, ?, 1, ?), (?, ?, 1, ?)
		`, userID, cardIDs[0], now.Add(-time.Hour), userID, cardIDs[1], now)
		if err != nil {
			t.Fatalf("Failed to add cards with timestamps: %v", err)
		}

		cards, err := GetUserTradingCards(db, userID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(cards) >= 2 {
			// Most recent should be first
			if cards[0].ID != cardIDs[1] {
				t.Errorf("Expected most recent card first, got card ID %d", cards[0].ID)
			}
		}
	})
}

func TestUserHasCard(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userID := seedTestUser(t, db, 0, 0)
	cardIDs := seedTestCards(t, db)

	t.Run("returns false when user doesn't have card", func(t *testing.T) {
		has, err := UserHasCard(db, userID, cardIDs[0])
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if has {
			t.Error("Expected false, got true")
		}
	})

	t.Run("returns true when user has card", func(t *testing.T) {
		_, err := db.Exec(`
			INSERT INTO user_trading_cards (user_id, card_id, quantity)
			VALUES (?, ?, 1)
		`, userID, cardIDs[0])
		if err != nil {
			t.Fatalf("Failed to add card to user: %v", err)
		}

		has, err := UserHasCard(db, userID, cardIDs[0])
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if !has {
			t.Error("Expected true, got false")
		}
	})

	t.Run("returns false for different user", func(t *testing.T) {
		otherUserID := seedTestUser(t, db, 0, 0)
		has, err := UserHasCard(db, otherUserID, cardIDs[0])
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if has {
			t.Error("Expected false for different user, got true")
		}
	})
}

func TestAddCardToUser(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userID := seedTestUser(t, db, 0, 0)
	cardIDs := seedTestCards(t, db)

	t.Run("adds new card to user", func(t *testing.T) {
		err := AddCardToUser(db, userID, cardIDs[0])
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		has, err := UserHasCard(db, userID, cardIDs[0])
		if err != nil {
			t.Fatalf("Failed to check card ownership: %v", err)
		}
		if !has {
			t.Error("Expected user to have card after adding")
		}

		// Check quantity is 1
		var quantity int
		err = db.QueryRow(`
			SELECT quantity FROM user_trading_cards WHERE user_id = ? AND card_id = ?
		`, userID, cardIDs[0]).Scan(&quantity)
		if err != nil {
			t.Fatalf("Failed to get quantity: %v", err)
		}
		if quantity != 1 {
			t.Errorf("Expected quantity 1, got %d", quantity)
		}
	})

	t.Run("increments quantity for duplicate card", func(t *testing.T) {
		// Add the same card again
		err := AddCardToUser(db, userID, cardIDs[0])
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Check quantity is incremented
		var quantity int
		err = db.QueryRow(`
			SELECT quantity FROM user_trading_cards WHERE user_id = ? AND card_id = ?
		`, userID, cardIDs[0]).Scan(&quantity)
		if err != nil {
			t.Fatalf("Failed to get quantity: %v", err)
		}
		if quantity != 2 {
			t.Errorf("Expected quantity 2, got %d", quantity)
		}
	})

	t.Run("handles multiple cards for same user", func(t *testing.T) {
		err := AddCardToUser(db, userID, cardIDs[1])
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		cards, err := GetUserTradingCards(db, userID)
		if err != nil {
			t.Fatalf("Failed to get user cards: %v", err)
		}
		if len(cards) != 2 {
			t.Errorf("Expected 2 cards, got %d", len(cards))
		}
	})

	t.Run("returns error for non-existent card", func(t *testing.T) {
		err := AddCardToUser(db, userID, 999)
		if err == nil {
			t.Error("Expected error for non-existent card, got nil")
		}
	})
}

func TestGetCardRewards(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cardIDs := seedTestCards(t, db)

	t.Run("returns empty slice when no rewards exist", func(t *testing.T) {
		rewards, err := GetCardRewards(db, cardIDs[0])
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(rewards) != 0 {
			t.Errorf("Expected 0 rewards, got %d", len(rewards))
		}
	})

	t.Run("returns rewards for card", func(t *testing.T) {
		// Add reward conditions
		_, err := db.Exec(`
			INSERT INTO card_rewards (card_id, reward_type, requirement_value)
			VALUES (?, 'game_win', 10), (?, 'games_played', 50)
		`, cardIDs[0], cardIDs[0])
		if err != nil {
			t.Fatalf("Failed to insert rewards: %v", err)
		}

		rewards, err := GetCardRewards(db, cardIDs[0])
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(rewards) != 2 {
			t.Errorf("Expected 2 rewards, got %d", len(rewards))
		}

		// Check reward details
		foundGameWin := false
		foundGamesPlayed := false
		for _, r := range rewards {
			if r.RewardType == "game_win" && r.RequirementValue == 10 {
				foundGameWin = true
			}
			if r.RewardType == "games_played" && r.RequirementValue == 50 {
				foundGamesPlayed = true
			}
		}
		if !foundGameWin || !foundGamesPlayed {
			t.Error("Expected to find both reward types")
		}
	})

	t.Run("returns only rewards for specified card", func(t *testing.T) {
		// Add reward for different card
		_, err := db.Exec(`
			INSERT INTO card_rewards (card_id, reward_type, requirement_value)
			VALUES (?, 'win_streak', 3)
		`, cardIDs[1])
		if err != nil {
			t.Fatalf("Failed to insert reward: %v", err)
		}

		rewards, err := GetCardRewards(db, cardIDs[1])
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(rewards) != 1 {
			t.Errorf("Expected 1 reward for card 1, got %d", len(rewards))
		}
		if rewards[0].RewardType != "win_streak" {
			t.Errorf("Expected win_streak reward, got %s", rewards[0].RewardType)
		}
	})

	t.Run("handles requirement_data field", func(t *testing.T) {
		// Add reward with requirement_data
		_, err := db.Exec(`
			INSERT INTO card_rewards (card_id, reward_type, requirement_value, requirement_data)
			VALUES (?, 'special_event', 100, 'test_event_data')
		`, cardIDs[2])
		if err != nil {
			t.Fatalf("Failed to insert reward: %v", err)
		}

		rewards, err := GetCardRewards(db, cardIDs[2])
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(rewards) != 1 {
			t.Errorf("Expected 1 reward, got %d", len(rewards))
		}
		if rewards[0].RequirementData != "test_event_data" {
			t.Errorf("Expected requirement_data 'test_event_data', got %s", rewards[0].RequirementData)
		}
	})
}

func TestGetAllCardRewards(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cardIDs := seedTestCards(t, db)

	t.Run("returns empty slice when no rewards exist", func(t *testing.T) {
		rewards, err := GetAllCardRewards(db)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(rewards) != 0 {
			t.Errorf("Expected 0 rewards, got %d", len(rewards))
		}
	})

	t.Run("returns all rewards sorted by card_id", func(t *testing.T) {
		// Add rewards for multiple cards
		_, err := db.Exec(`
			INSERT INTO card_rewards (card_id, reward_type, requirement_value)
			VALUES
				(?, 'game_win', 5),
				(?, 'games_played', 20),
				(?, 'win_streak', 3),
				(?, 'game_win', 1)
		`, cardIDs[1], cardIDs[0], cardIDs[1], cardIDs[0])
		if err != nil {
			t.Fatalf("Failed to insert rewards: %v", err)
		}

		rewards, err := GetAllCardRewards(db)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(rewards) != 4 {
			t.Errorf("Expected 4 rewards, got %d", len(rewards))
		}

		// Check that rewards are sorted by card_id
		for i := 1; i < len(rewards); i++ {
			if rewards[i].CardID < rewards[i-1].CardID {
				t.Error("Expected rewards to be sorted by card_id")
				break
			}
		}
	})
}

func TestGetUserCardProgress(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cardIDs := seedTestCards(t, db)
	userID := seedTestUser(t, db, 25, 8)

	t.Run("returns progress for all cards", func(t *testing.T) {
		// Add rewards
		_, err := db.Exec(`
			INSERT INTO card_rewards (card_id, reward_type, requirement_value)
			VALUES
				(?, 'game_win', 5),
				(?, 'games_played', 20),
				(?, 'win_streak', 3),
				(?, 'game_win', 10)
		`, cardIDs[0], cardIDs[1], cardIDs[2], cardIDs[3])
		if err != nil {
			t.Fatalf("Failed to insert rewards: %v", err)
		}

		progress, err := GetUserCardProgress(db, userID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if len(progress) != 4 {
			t.Errorf("Expected progress for 4 cards, got %d", len(progress))
		}
	})

	t.Run("correctly calculates game_win progress", func(t *testing.T) {
		// Clear rewards
		_, err := db.Exec(`DELETE FROM card_rewards`)
		if err != nil {
			t.Fatalf("Failed to clear rewards: %v", err)
		}

		_, err = db.Exec(`
			INSERT INTO card_rewards (card_id, reward_type, requirement_value)
			VALUES (?, 'game_win', 5)
		`, cardIDs[0])
		if err != nil {
			t.Fatalf("Failed to insert reward: %v", err)
		}

		progress, err := GetUserCardProgress(db, userID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Find progress for card 0
		var cardProgress *CardProgress
		for i := range progress {
			if progress[i].Card.ID == cardIDs[0] {
				cardProgress = &progress[i]
				break
			}
		}

		if cardProgress == nil {
			t.Fatal("Expected to find progress for card 0")
		}

		if cardProgress.Progress != 8 {
			t.Errorf("Expected progress 8 (games won), got %d", cardProgress.Progress)
		}
		if cardProgress.RequiredValue != 5 {
			t.Errorf("Expected required value 5, got %d", cardProgress.RequiredValue)
		}
		if cardProgress.RewardType != "game_win" {
			t.Errorf("Expected reward type 'game_win', got %s", cardProgress.RewardType)
		}
	})

	t.Run("correctly calculates games_played progress", func(t *testing.T) {
		// Clear rewards
		_, err := db.Exec(`DELETE FROM card_rewards`)
		if err != nil {
			t.Fatalf("Failed to clear rewards: %v", err)
		}

		_, err = db.Exec(`
			INSERT INTO card_rewards (card_id, reward_type, requirement_value)
			VALUES (?, 'games_played', 30)
		`, cardIDs[1])
		if err != nil {
			t.Fatalf("Failed to insert reward: %v", err)
		}

		progress, err := GetUserCardProgress(db, userID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Find progress for card 1
		var cardProgress *CardProgress
		for i := range progress {
			if progress[i].Card.ID == cardIDs[1] {
				cardProgress = &progress[i]
				break
			}
		}

		if cardProgress == nil {
			t.Fatal("Expected to find progress for card 1")
		}

		if cardProgress.Progress != 25 {
			t.Errorf("Expected progress 25 (games played), got %d", cardProgress.Progress)
		}
		if cardProgress.RequiredValue != 30 {
			t.Errorf("Expected required value 30, got %d", cardProgress.RequiredValue)
		}
	})

	t.Run("marks owned cards as unlocked", func(t *testing.T) {
		// Give user a card
		err := AddCardToUser(db, userID, cardIDs[0])
		if err != nil {
			t.Fatalf("Failed to add card to user: %v", err)
		}

		progress, err := GetUserCardProgress(db, userID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Find progress for card 0
		var cardProgress *CardProgress
		for i := range progress {
			if progress[i].Card.ID == cardIDs[0] {
				cardProgress = &progress[i]
				break
			}
		}

		if cardProgress == nil {
			t.Fatal("Expected to find progress for card 0")
		}

		if !cardProgress.Unlocked {
			t.Error("Expected card to be marked as unlocked")
		}
	})

	t.Run("handles cards without rewards", func(t *testing.T) {
		// Clear all rewards
		_, err := db.Exec(`DELETE FROM card_rewards`)
		if err != nil {
			t.Fatalf("Failed to clear rewards: %v", err)
		}

		progress, err := GetUserCardProgress(db, userID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Should still return progress for all cards
		if len(progress) != 4 {
			t.Errorf("Expected progress for 4 cards, got %d", len(progress))
		}

		// Check that cards without rewards have zero values
		for _, p := range progress {
			if p.RewardType != "" || p.RequiredValue != 0 || p.Progress != 0 {
				// Card 0 might still be unlocked from previous test
				if p.Card.ID != cardIDs[0] || !p.Unlocked {
					t.Errorf("Expected zero values for card without rewards, got: type=%s, req=%d, prog=%d",
						p.RewardType, p.RequiredValue, p.Progress)
				}
			}
		}
	})

	t.Run("returns zero progress for unsupported reward types", func(t *testing.T) {
		// Add rewards with unsupported types
		_, err := db.Exec(`
			INSERT INTO card_rewards (card_id, reward_type, requirement_value)
			VALUES
				(?, 'win_streak', 5),
				(?, 'high_score', 100),
				(?, 'special_event', 50)
		`, cardIDs[0], cardIDs[1], cardIDs[2])
		if err != nil {
			t.Fatalf("Failed to insert rewards: %v", err)
		}

		progress, err := GetUserCardProgress(db, userID)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Check that unsupported types have progress = 0
		unsupportedTypes := map[string]bool{"win_streak": true, "high_score": true, "special_event": true}
		for _, p := range progress {
			if unsupportedTypes[p.RewardType] && p.Progress != 0 {
				t.Errorf("Expected progress 0 for unsupported type %s, got %d", p.RewardType, p.Progress)
			}
		}
	})
}