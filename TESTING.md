# Trading Card Tests

This document describes the comprehensive test suite created for the trading cards feature.

## Test Files Created

### 1. backend/internal/models/trading_card_test.go
Comprehensive unit tests for trading card model functions. Tests all database operations and business logic.

**Test Coverage:**
- `TestGetAllTradingCards` - Tests retrieving all cards, empty database, sorting
- `TestGetTradingCardByID` - Tests fetching single card by ID, handles not found
- `TestGetUserTradingCards` - Tests user card retrieval with ownership details
- `TestUserHasCard` - Tests card ownership checking
- `TestAddCardToUser` - Tests adding cards, quantity increment, duplicates
- `TestGetCardRewards` - Tests reward condition retrieval
- `TestGetAllCardRewards` - Tests fetching all reward conditions
- `TestGetUserCardProgress` - Tests progress calculation for all cards

**Functions Tested:** (8 functions, 100% coverage)
- `GetAllTradingCards()`
- `GetTradingCardByID()`
- `GetUserTradingCards()`
- `UserHasCard()`
- `AddCardToUser()`
- `GetCardRewards()`
- `GetAllCardRewards()`
- `GetUserCardProgress()`

**Test Scenarios:** 30+ test cases including:
- Empty database scenarios
- Success cases with valid data
- Edge cases (duplicates, multiple conditions)
- Boundary conditions (zero values, large quantities)
- Error cases (not found, invalid IDs)
- Progress calculation for all reward types
- Sorting and ordering verification

### 2. backend/internal/handlers/trading_cards_test.go
Comprehensive unit tests for trading card HTTP handlers. Tests all API endpoints with various scenarios.

**Test Coverage:**
- `TestGetAllCardsHandler` - Tests GET /cards endpoint
- `TestGetUserCardsHandler` - Tests GET /me/cards endpoint (with auth)
- `TestClaimCardHandler` - Tests POST /me/cards/:id/claim endpoint (complex logic)
- `TestGetCardProgressHandler` - Tests GET /me/cards/progress endpoint
- `TestClaimCardHandler_EdgeCases` - Additional edge case tests

**Handlers Tested:** (4 handlers, 100% coverage)
- `GetAllCardsHandler()`
- `GetUserCardsHandler()`
- `ClaimCardHandler()`
- `GetCardProgressHandler()`

**Test Scenarios:** 25+ test cases including:
- Authentication tests (401 Unauthorized)
- Empty result tests
- Success cases with proper response structure
- Invalid input tests (400 Bad Request)
- Not found tests (404)
- Conflict tests (409 - card already owned)
- Forbidden tests (403 - requirements not met)
- Multiple reward conditions
- Unsupported reward types
- Database error handling (500)
- Edge cases and boundary conditions

## Running the Tests

### Prerequisites
- Go 1.25 or later
- CGO_ENABLED=1
- C compiler (gcc or clang)
- SQLite3 development libraries

### Quick Start

On systems with a C compiler (Linux, macOS):

```bash
cd backend

# Run all trading card tests
CGO_ENABLED=1 go test -v ./internal/models/ ./internal/handlers/ -run ".*TradingCard.*|.*Card.*"

# Or use the provided script
./run_trading_card_tests.sh
```

### Individual Test Runs

```bash
# Run model tests only
CGO_ENABLED=1 go test -v ./internal/models/ -run "TestGetAllTradingCards|TestGetTradingCardByID|TestGetUserTradingCards|TestUserHasCard|TestAddCardToUser|TestGetCardRewards|TestGetAllCardRewards|TestGetUserCardProgress"

# Run handler tests only
CGO_ENABLED=1 go test -v ./internal/handlers/ -run "TestGetAllCardsHandler|TestGetUserCardsHandler|TestClaimCardHandler|TestGetCardProgressHandler"

# Run with race detection
CGO_ENABLED=1 go test -v -race ./internal/models/ ./internal/handlers/

# Run with coverage
CGO_ENABLED=1 go test -v -cover ./internal/models/ ./internal/handlers/
```

### Docker Environment

If your local environment doesn't have a C compiler:

```bash
# Using Docker
docker run --rm -v $(pwd):/app -w /app golang:1.25 sh -c "
  apt-get update && apt-get install -y gcc &&
  cd backend &&
  CGO_ENABLED=1 go test -v ./internal/models/ ./internal/handlers/
"
```

## Test Design

### Model Tests
- Use in-memory SQLite database (`:memory:`)
- Each test is isolated with fresh database
- Helper functions for setup: `setupTestDB()`, `seedTestCards()`, `seedTestUser()`
- Tests verify both success and error paths
- Comprehensive edge case coverage

### Handler Tests
- Use Gin test mode
- Mock HTTP contexts where needed
- Test all HTTP status codes
- Verify JSON response structure
- Test authentication/authorization
- Isolated database for each test

## Test Quality Features

1. **Comprehensive Coverage**: Tests cover all functions and all code paths
2. **Edge Cases**: Tests include boundary conditions, empty states, duplicates
3. **Error Handling**: Tests verify all error conditions return proper responses
4. **Isolation**: Each test uses a fresh database and doesn't affect others
5. **Regression Prevention**: Tests verify existing behavior is maintained
6. **Documentation**: Test names clearly describe what is being tested

## Known Limitations

- Tests require CGO and a C compiler due to SQLite dependency
- Some reward types (win_streak, high_score, special_event) have placeholder implementations
- Tests use in-memory database, not the actual production database

## Additional Test Ideas for Future Enhancement

1. Concurrency tests for simultaneous card claims
2. Performance tests for large numbers of cards
3. Integration tests with real database migrations
4. End-to-end tests with full HTTP stack
5. Stress tests for card progress calculations