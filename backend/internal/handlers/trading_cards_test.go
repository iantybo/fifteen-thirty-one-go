package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"fifteen-thirty-one-go/backend/internal/models"
	"github.com/gin-gonic/gin"
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

// setupTestRouter creates a gin router in test mode.
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestGetAllCardsHandler(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	router := setupTestRouter()
	router.GET("/cards", GetAllCardsHandler(db))

	t.Run("returns empty array when no cards exist", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/cards", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string][]models.TradingCard
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if len(response["cards"]) != 0 {
			t.Errorf("Expected 0 cards, got %d", len(response["cards"]))
		}
	})

	t.Run("returns all cards when cards exist", func(t *testing.T) {
		cardIDs := seedTestCards(t, db)
		if len(cardIDs) == 0 {
			t.Fatal("Failed to seed test cards")
		}

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/cards", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string][]models.TradingCard
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		cards := response["cards"]
		if len(cards) != 3 {
			t.Errorf("Expected 3 cards, got %d", len(cards))
		}

		// Verify card data structure
		if len(cards) > 0 {
			card := cards[0]
			if card.Name == "" {
				t.Error("Expected card to have a name")
			}
			if card.Description == "" {
				t.Error("Expected card to have a description")
			}
			if card.Rarity == "" {
				t.Error("Expected card to have a rarity")
			}
			if card.ArtworkURL == "" {
				t.Error("Expected card to have an artwork URL")
			}
			if card.Category == "" {
				t.Error("Expected card to have a category")
			}
		}
	})

	t.Run("returns 500 when database query fails", func(t *testing.T) {
		// Close the database to force an error
		db.Close()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/cards", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", w.Code)
		}

		var response map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["error"] == "" {
			t.Error("Expected error message in response")
		}
	})
}

func TestGetUserCardsHandler(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	router := setupTestRouter()
	router.GET("/me/cards", GetUserCardsHandler(db))

	userID := seedTestUser(t, db, 0, 0)
	cardIDs := seedTestCards(t, db)

	t.Run("returns 401 when user is not authenticated", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/me/cards", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}

		var response map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["error"] != "Unauthorized" {
			t.Errorf("Expected 'Unauthorized' error, got %s", response["error"])
		}
	})

	t.Run("returns empty array when user has no cards", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/me/cards", nil)
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", userID)

		GetUserCardsHandler(db)(c)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string][]models.UserCardWithDetails
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if len(response["cards"]) != 0 {
			t.Errorf("Expected 0 cards, got %d", len(response["cards"]))
		}
	})

	t.Run("returns user's cards with quantity and acquisition date", func(t *testing.T) {
		// Add cards to user
		_, err := db.Exec(`
			INSERT INTO user_trading_cards (user_id, card_id, quantity)
			VALUES (?, ?, 1), (?, ?, 2)
		`, userID, cardIDs[0], userID, cardIDs[1])
		if err != nil {
			t.Fatalf("Failed to add cards to user: %v", err)
		}

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/me/cards", nil)
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", userID)

		GetUserCardsHandler(db)(c)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string][]models.UserCardWithDetails
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		cards := response["cards"]
		if len(cards) != 2 {
			t.Errorf("Expected 2 cards, got %d", len(cards))
		}

		// Verify card details
		for _, card := range cards {
			if card.Quantity == 0 {
				t.Error("Expected quantity to be set")
			}
			if card.AcquiredAt.IsZero() {
				t.Error("Expected acquired_at to be set")
			}
		}
	})
}

func TestClaimCardHandler(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	router := setupTestRouter()
	router.POST("/me/cards/:id/claim", ClaimCardHandler(db))

	userID := seedTestUser(t, db, 25, 10)
	cardIDs := seedTestCards(t, db)

	t.Run("returns 401 when user is not authenticated", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/me/cards/1/claim", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("returns 400 for invalid card ID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/me/cards/invalid/claim", nil)
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", userID)
		c.Params = gin.Params{gin.Param{Key: "id", Value: "invalid"}}

		ClaimCardHandler(db)(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("returns 404 for non-existent card", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/me/cards/999/claim", nil)
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", userID)
		c.Params = gin.Params{gin.Param{Key: "id", Value: "999"}}

		ClaimCardHandler(db)(c)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}

		var response map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["error"] != "Card not found" {
			t.Errorf("Expected 'Card not found' error, got %s", response["error"])
		}
	})

	t.Run("returns 409 when card is already owned", func(t *testing.T) {
		// Give user a card
		err := models.AddCardToUser(db, userID, cardIDs[0])
		if err != nil {
			t.Fatalf("Failed to add card to user: %v", err)
		}

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/me/cards/1/claim", nil)
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", userID)
		c.Params = gin.Params{gin.Param{Key: "id", Value: "1"}}

		ClaimCardHandler(db)(c)

		if w.Code != http.StatusConflict {
			t.Errorf("Expected status 409, got %d", w.Code)
		}

		var response map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["error"] != "Card already claimed" {
			t.Errorf("Expected 'Card already claimed' error, got %s", response["error"])
		}
	})

	t.Run("returns 403 when card has no reward conditions", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/me/cards/2/claim", nil)
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", userID)
		c.Params = gin.Params{gin.Param{Key: "id", Value: "2"}}

		ClaimCardHandler(db)(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", w.Code)
		}

		var response map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["error"] != "Card has no reward conditions" {
			t.Errorf("Expected 'Card has no reward conditions' error, got %s", response["error"])
		}
	})

	t.Run("returns 403 when requirements not met", func(t *testing.T) {
		// Add a reward requirement that user doesn't meet
		_, err := db.Exec(`
			INSERT INTO card_rewards (card_id, reward_type, requirement_value)
			VALUES (?, 'game_win', 20)
		`, cardIDs[1])
		if err != nil {
			t.Fatalf("Failed to insert reward: %v", err)
		}

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/me/cards/2/claim", nil)
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", userID)
		c.Params = gin.Params{gin.Param{Key: "id", Value: "2"}}

		ClaimCardHandler(db)(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", w.Code)
		}

		var response map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["error"] != "Requirements not met for this card" {
			t.Errorf("Expected 'Requirements not met' error, got %s", response["error"])
		}
	})

	t.Run("successfully claims card when game_win requirements are met", func(t *testing.T) {
		// Clear existing rewards and add one user can meet
		_, err := db.Exec(`DELETE FROM card_rewards`)
		if err != nil {
			t.Fatalf("Failed to clear rewards: %v", err)
		}

		_, err = db.Exec(`
			INSERT INTO card_rewards (card_id, reward_type, requirement_value)
			VALUES (?, 'game_win', 5)
		`, cardIDs[2])
		if err != nil {
			t.Fatalf("Failed to insert reward: %v", err)
		}

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/me/cards/3/claim", nil)
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", userID)
		c.Params = gin.Params{gin.Param{Key: "id", Value: "3"}}

		ClaimCardHandler(db)(c)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if response["message"] != "Card claimed successfully" {
			t.Errorf("Expected success message, got %v", response["message"])
		}

		// Verify user has the card
		has, err := models.UserHasCard(db, userID, cardIDs[2])
		if err != nil {
			t.Fatalf("Failed to check card ownership: %v", err)
		}
		if !has {
			t.Error("Expected user to have card after claiming")
		}
	})

	t.Run("successfully claims card when games_played requirements are met", func(t *testing.T) {
		// Create a new user and card for this test
		newUserID := seedTestUser(t, db, 50, 10)
		res, err := db.Exec(`
			INSERT INTO trading_cards (name, description, rarity, artwork_url, category)
			VALUES (?, ?, ?, ?, ?)
		`, "Games Played Card", "Earned by playing games", "common", "/test.png", "test")
		if err != nil {
			t.Fatalf("Failed to insert card: %v", err)
		}
		newCardID, _ := res.LastInsertId()

		_, err = db.Exec(`
			INSERT INTO card_rewards (card_id, reward_type, requirement_value)
			VALUES (?, 'games_played', 30)
		`, newCardID)
		if err != nil {
			t.Fatalf("Failed to insert reward: %v", err)
		}

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/me/cards/4/claim", nil)
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", newUserID)
		c.Params = gin.Params{gin.Param{Key: "id", Value: "4"}}

		ClaimCardHandler(db)(c)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
		}

		// Verify user has the card
		has, err := models.UserHasCard(db, newUserID, newCardID)
		if err != nil {
			t.Fatalf("Failed to check card ownership: %v", err)
		}
		if !has {
			t.Error("Expected user to have card after claiming")
		}
	})

	t.Run("skips eligibility check for unsupported reward types", func(t *testing.T) {
		// Create card with only unsupported reward type
		res, err := db.Exec(`
			INSERT INTO trading_cards (name, description, rarity, artwork_url, category)
			VALUES (?, ?, ?, ?, ?)
		`, "Special Event Card", "Special event only", "rare", "/special.png", "event")
		if err != nil {
			t.Fatalf("Failed to insert card: %v", err)
		}
		specialCardID, _ := res.LastInsertId()

		_, err = db.Exec(`
			INSERT INTO card_rewards (card_id, reward_type, requirement_value)
			VALUES (?, 'special_event', 100)
		`, specialCardID)
		if err != nil {
			t.Fatalf("Failed to insert reward: %v", err)
		}

		newUserID := seedTestUser(t, db, 0, 0)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/me/cards/5/claim", nil)
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", newUserID)
		c.Params = gin.Params{gin.Param{Key: "id", Value: "5"}}

		ClaimCardHandler(db)(c)

		// Should return 403 since unsupported types don't make user eligible
		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403 for unsupported reward type, got %d", w.Code)
		}
	})
}

func TestGetCardProgressHandler(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	router := setupTestRouter()
	router.GET("/me/cards/progress", GetCardProgressHandler(db))

	userID := seedTestUser(t, db, 15, 5)
	cardIDs := seedTestCards(t, db)

	t.Run("returns 401 when user is not authenticated", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/me/cards/progress", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("returns progress for all cards", func(t *testing.T) {
		// Add rewards
		_, err := db.Exec(`
			INSERT INTO card_rewards (card_id, reward_type, requirement_value)
			VALUES
				(?, 'game_win', 10),
				(?, 'games_played', 20),
				(?, 'win_streak', 3)
		`, cardIDs[0], cardIDs[1], cardIDs[2])
		if err != nil {
			t.Fatalf("Failed to insert rewards: %v", err)
		}

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/me/cards/progress", nil)
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", userID)

		GetCardProgressHandler(db)(c)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string][]models.CardProgress
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		progress := response["progress"]
		if len(progress) != 3 {
			t.Errorf("Expected progress for 3 cards, got %d", len(progress))
		}

		// Verify structure
		for _, p := range progress {
			if p.Card.ID == 0 {
				t.Error("Expected card ID to be set")
			}
			if p.Card.Name == "" {
				t.Error("Expected card name to be set")
			}
		}
	})

	t.Run("shows correct progress for game_win rewards", func(t *testing.T) {
		// Clear rewards
		_, err := db.Exec(`DELETE FROM card_rewards`)
		if err != nil {
			t.Fatalf("Failed to clear rewards: %v", err)
		}

		_, err = db.Exec(`
			INSERT INTO card_rewards (card_id, reward_type, requirement_value)
			VALUES (?, 'game_win', 10)
		`, cardIDs[0])
		if err != nil {
			t.Fatalf("Failed to insert reward: %v", err)
		}

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/me/cards/progress", nil)
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", userID)

		GetCardProgressHandler(db)(c)

		var response map[string][]models.CardProgress
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		progress := response["progress"]
		// Find the card with the reward
		var cardProgress *models.CardProgress
		for i := range progress {
			if progress[i].Card.ID == cardIDs[0] {
				cardProgress = &progress[i]
				break
			}
		}

		if cardProgress == nil {
			t.Fatal("Expected to find progress for card with reward")
		}

		if cardProgress.Progress != 5 {
			t.Errorf("Expected progress 5 (user's games won), got %d", cardProgress.Progress)
		}
		if cardProgress.RequiredValue != 10 {
			t.Errorf("Expected required value 10, got %d", cardProgress.RequiredValue)
		}
		if cardProgress.RewardType != "game_win" {
			t.Errorf("Expected reward type 'game_win', got %s", cardProgress.RewardType)
		}
	})

	t.Run("shows correct progress for games_played rewards", func(t *testing.T) {
		_, err := db.Exec(`DELETE FROM card_rewards`)
		if err != nil {
			t.Fatalf("Failed to clear rewards: %v", err)
		}

		_, err = db.Exec(`
			INSERT INTO card_rewards (card_id, reward_type, requirement_value)
			VALUES (?, 'games_played', 20)
		`, cardIDs[1])
		if err != nil {
			t.Fatalf("Failed to insert reward: %v", err)
		}

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/me/cards/progress", nil)
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", userID)

		GetCardProgressHandler(db)(c)

		var response map[string][]models.CardProgress
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		progress := response["progress"]
		var cardProgress *models.CardProgress
		for i := range progress {
			if progress[i].Card.ID == cardIDs[1] {
				cardProgress = &progress[i]
				break
			}
		}

		if cardProgress == nil {
			t.Fatal("Expected to find progress for card with reward")
		}

		if cardProgress.Progress != 15 {
			t.Errorf("Expected progress 15 (user's games played), got %d", cardProgress.Progress)
		}
	})

	t.Run("marks owned cards as unlocked", func(t *testing.T) {
		// Give user a card
		err := models.AddCardToUser(db, userID, cardIDs[0])
		if err != nil {
			t.Fatalf("Failed to add card to user: %v", err)
		}

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/me/cards/progress", nil)
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", userID)

		GetCardProgressHandler(db)(c)

		var response map[string][]models.CardProgress
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		progress := response["progress"]
		var cardProgress *models.CardProgress
		for i := range progress {
			if progress[i].Card.ID == cardIDs[0] {
				cardProgress = &progress[i]
				break
			}
		}

		if cardProgress == nil {
			t.Fatal("Expected to find progress for owned card")
		}

		if !cardProgress.Unlocked {
			t.Error("Expected owned card to be marked as unlocked")
		}
	})

	t.Run("returns 500 when database error occurs", func(t *testing.T) {
		// Close database to force error
		db.Close()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/me/cards/progress", nil)
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", userID)

		GetCardProgressHandler(db)(c)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", w.Code)
		}
	})
}

func TestClaimCardHandler_EdgeCases(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	t.Run("handles multiple reward conditions (any satisfied)", func(t *testing.T) {
		userID := seedTestUser(t, db, 100, 5)
		cardIDs := seedTestCards(t, db)

		// Add multiple reward conditions
		_, err := db.Exec(`
			INSERT INTO card_rewards (card_id, reward_type, requirement_value)
			VALUES
				(?, 'game_win', 20),
				(?, 'games_played', 50)
		`, cardIDs[0], cardIDs[0])
		if err != nil {
			t.Fatalf("Failed to insert rewards: %v", err)
		}

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/me/cards/1/claim", nil)
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", userID)
		c.Params = gin.Params{gin.Param{Key: "id", Value: "1"}}

		ClaimCardHandler(db)(c)

		// User has 5 wins (doesn't meet game_win requirement of 20)
		// but has 100 games played (meets games_played requirement of 50)
		// Should succeed because at least one condition is met
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
		}
	})

	t.Run("handles card with only unsupported reward types", func(t *testing.T) {
		userID := seedTestUser(t, db, 10, 5)

		res, err := db.Exec(`
			INSERT INTO trading_cards (name, description, rarity, artwork_url, category)
			VALUES (?, ?, ?, ?, ?)
		`, "Streak Card", "Win streak card", "rare", "/streak.png", "achievement")
		if err != nil {
			t.Fatalf("Failed to insert card: %v", err)
		}
		cardID, _ := res.LastInsertId()

		_, err = db.Exec(`
			INSERT INTO card_rewards (card_id, reward_type, requirement_value)
			VALUES (?, 'win_streak', 5), (?, 'high_score', 100)
		`, cardID, cardID)
		if err != nil {
			t.Fatalf("Failed to insert rewards: %v", err)
		}

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/me/cards/4/claim", nil)
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", userID)
		c.Params = gin.Params{gin.Param{Key: "id", Value: "4"}}

		ClaimCardHandler(db)(c)

		// All reward types are unsupported, so user should not be eligible
		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", w.Code)
		}
	})
}