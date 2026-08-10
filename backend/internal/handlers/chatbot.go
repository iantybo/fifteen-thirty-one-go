package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"fifteen-thirty-one-go/backend/internal/models"
	"github.com/gin-gonic/gin"
)

type ChatbotRequest struct {
	Message     string       `json:"message" binding:"required"`
	GameContext *GameContext `json:"game_context,omitempty"`
}

type GameContext struct {
	GameID   int64   `json:"game_id"`
	Stage    string  `json:"stage"`
	Scores   []int64 `json:"scores"`
	HandSize int     `json:"hand_size"`
}

type ChatbotResponse struct {
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AnthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []AnthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
}

type AnthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

// ChatbotHandler handles chatbot requests for games with bot opponents.
// It validates user access, verifies the game has bot players, and returns AI-generated responses.
func ChatbotHandler(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := UserID(c)
		if userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		gameID, err := ParseInt64Param(c, "id")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid game id"})
			return
		}

		var req ChatbotRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Verify user is in the game
		players, err := models.ListGamePlayersByGame(db, gameID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load game"})
			return
		}

		userInGame := false
		hasBot := false
		for _, p := range players {
			if p.UserID == userID {
				userInGame = true
			}
			if p.IsBot {
				hasBot = true
			}
		}

		if !userInGame {
			c.JSON(http.StatusForbidden, gin.H{"error": "you are not in this game"})
			return
		}

		if !hasBot {
			c.JSON(http.StatusBadRequest, gin.H{"error": "chatbot only available in games with bot opponents"})
			return
		}

		// Build system prompt with game context
		systemPrompt := buildSystemPrompt(req.GameContext)

		// Call Anthropic API
		response, err := callAnthropicAPI(c.Request.Context(), systemPrompt, req.Message)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get chatbot response"})
			return
		}

		c.JSON(http.StatusOK, ChatbotResponse{
			Message:   response,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// buildSystemPrompt constructs the system prompt for the Anthropic API.
// If ctx is provided, it includes current game state information in the prompt.
func buildSystemPrompt(ctx *GameContext) string {
	basePrompt := `You are a helpful assistant for a cribbage card game called "Fifteen Thirty-One".
You help players understand the game rules, strategies, and answer questions about their current game state.

The game follows standard cribbage rules with a pegging phase where players try to reach 15 or 31 points without going over.
Be concise, friendly, and focus on helping the player improve their gameplay.`

	if ctx != nil {
		basePrompt += fmt.Sprintf(`

Current game state:
- Stage: %s
- Scores: %v
- Cards in hand: %d

Use this context to provide relevant, specific advice.`, ctx.Stage, ctx.Scores, ctx.HandSize)
	}

	return basePrompt
}

func callAnthropicAPI(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	apiKey := getAnthropicAPIKey()
	if apiKey == "" {
		return "I'm sorry, the chatbot service is not configured. Please contact the administrator.", nil
	}

	reqBody := AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 500,
		System:    systemPrompt,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: userMessage,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var apiResp AnthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	return apiResp.Content[0].Text, nil
}

func getAnthropicAPIKey() string {
	// Check environment variable for API key
	// Set ANTHROPIC_API_KEY environment variable to enable chatbot functionality
	return os.Getenv("ANTHROPIC_API_KEY")
}
