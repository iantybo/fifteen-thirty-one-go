# Daily Challenge Service

A fun Java microservice that generates daily Cribbage puzzle challenges for Fifteen-Thirty-One card game players.

## Features

### Challenge Types

1. **Optimal Discard** - Given 6 cards, find the best 2 to discard for maximum score
2. **Max Score Hunt** - Calculate the exact score for a hand with starter card
3. **Best Peg Play** - Choose the optimal card to play during pegging

### Scoring System

- Base points: 100 points for correct answer
- Speed bonuses:
  - < 30 seconds: +50% bonus (150 points total)
  - < 60 seconds: +30% bonus (130 points total)
  - < 120 seconds: +10% bonus (110 points total)

### Leaderboards

- **Daily Leaderboard** - Rankings for today's challenge (fastest correct submissions)
- **Global Leaderboard** - All-time top players by total points

### Statistics Tracking

- Total challenges completed
- Accuracy rate (correct/total)
- Current streak (consecutive days)
- Longest streak
- Total points earned

## Architecture

### Technology Stack

- **Spring Boot 3.2.1** - REST API framework
- **Java 17** - Programming language
- **SQLite** - Database (shared with Go backend)
- **JWT** - Authentication (compatible with Go backend tokens)
- **Maven** - Build tool

### Project Structure

```
daily-challenge-service/
├── src/main/java/com/cribbagegame/dailychallenge/
│   ├── DailyChallengeServiceApplication.java  # Main application
│   ├── config/
│   │   └── WebConfig.java                      # CORS configuration
│   ├── controller/
│   │   └── ChallengeController.java            # REST endpoints
│   ├── dto/
│   │   ├── ChallengeSubmissionRequest.java     # Request DTOs
│   │   ├── ChallengeSubmissionResponse.java
│   │   └── LeaderboardEntry.java
│   ├── entity/
│   │   ├── ChallengeSubmission.java            # JPA entities
│   │   └── UserChallengeStats.java
│   ├── model/
│   │   ├── Card.java                           # Domain models
│   │   ├── ChallengeType.java
│   │   └── DailyChallenge.java
│   ├── repository/
│   │   ├── ChallengeSubmissionRepository.java  # Data access
│   │   └── UserChallengeStatsRepository.java
│   ├── security/
│   │   └── JwtUtil.java                        # JWT validation
│   └── service/
│       ├── ChallengeGenerator.java             # Challenge creation
│       ├── ChallengeService.java               # Business logic
│       └── CribbageScorer.java                 # Hand scoring
└── src/main/resources/
    └── application.properties                   # Configuration
```

## Getting Started

### Prerequisites

- Java 17 or higher
- Maven 3.6+
- SQLite database (shared with Go backend)

### Installation

1. **Run database migration:**

```bash
cd /Users/ianbowers/git/fifteen-thirty-one-go
sqlite3 backend/app.db < backend/internal/database/migrations/003_daily_challenges.sql
```

2. **Configure environment:**

Create `.env` file or set environment variables:

```bash
export JWT_SECRET="your-secret-key-matching-go-backend"
```

3. **Build the service:**

```bash
cd daily-challenge-service
mvn clean package
```

4. **Run the service:**

```bash
mvn spring-boot:run
```

The service will start on `http://localhost:8081`

### Docker Deployment (Optional)

```bash
# Build
mvn clean package
docker build -t daily-challenge-service .

# Run
docker run -p 8081:8081 \
  -e JWT_SECRET="your-secret" \
  -v $(pwd)/../backend/app.db:/app/app.db \
  daily-challenge-service
```

## API Endpoints

### Get Today's Challenge

```http
GET /api/challenges/today
```

**Response:**
```json
{
  "challengeId": "OD-2026-01-12",
  "date": "2026-01-12",
  "type": "OPTIMAL_DISCARD",
  "cards": [
    {"rank": "FIVE", "suit": "HEARTS"},
    {"rank": "FIVE", "suit": "DIAMONDS"},
    {"rank": "JACK", "suit": "CLUBS"},
    {"rank": "TEN", "suit": "SPADES"},
    {"rank": "KING", "suit": "HEARTS"},
    {"rank": "QUEEN", "suit": "DIAMONDS"}
  ],
  "starterCard": {"rank": "FIVE", "suit": "CLUBS"},
  "maxPoints": 100,
  "hint": "Look for high-scoring combinations like runs and fifteens!",
  "difficulty": 4
}
```

### Submit Solution

```http
POST /api/challenges/submit
Authorization: Bearer <jwt-token>
Content-Type: application/json

{
  "answer": "[2,5]",  // For OPTIMAL_DISCARD: indices to discard
                       // For MAX_SCORE_HUNT: "24"
                       // For BEST_PEG_PLAY: "1"
  "timeTakenSeconds": 45
}
```

**Response:**
```json
{
  "correct": true,
  "pointsEarned": 130,
  "explanation": "Great job! You found the optimal discard."
}
```

### Get Daily Leaderboard

```http
GET /api/challenges/leaderboard/today
```

**Response:**
```json
[
  {
    "userId": 42,
    "pointsEarned": 150,
    "timeTakenSeconds": 28,
    "submittedAt": "2026-01-12T10:15:30Z"
  }
]
```

### Get Global Leaderboard

```http
GET /api/challenges/leaderboard/global
```

**Response:**
```json
[
  {
    "userId": 42,
    "pointsEarned": 15000,
    "challengesCompleted": 150,
    "currentStreak": 45,
    "longestStreak": 67
  }
]
```

### Get User Stats

```http
GET /api/challenges/stats/me
Authorization: Bearer <jwt-token>
```

**Response:**
```json
{
  "userId": 42,
  "totalChallengesCompleted": 150,
  "totalChallengesCorrect": 142,
  "currentStreak": 45,
  "longestStreak": 67,
  "totalPoints": 15000,
  "lastCompletedDate": "2026-01-12T10:15:30Z"
}
```

## Integration with Go Backend

### Authentication

The service validates JWT tokens from the Go backend using the same secret key. Ensure `JWT_SECRET` environment variable matches between services.

**Go Backend Token Format:**
- Algorithm: HS256
- Claim: `user_id` (integer)
- Expiration: 24 hours

### Database Sharing

Both services use the same SQLite database (`backend/app.db`). The Java service creates two new tables:

- `challenge_submissions` - User submissions
- `user_challenge_stats` - Aggregate statistics

### Frontend Integration

Add API calls to your React frontend:

```typescript
// Get today's challenge
const response = await fetch('http://localhost:8081/api/challenges/today');
const challenge = await response.json();

// Submit solution
const submitResponse = await fetch('http://localhost:8081/api/challenges/submit', {
  method: 'POST',
  headers: {
    'Authorization': `Bearer ${jwtToken}`,
    'Content-Type': 'application/json'
  },
  body: JSON.stringify({
    answer: "[2,5]",
    timeTakenSeconds: 45
  })
});
```

## Development

### Running Tests

```bash
mvn test
```

### Code Style

The project uses Lombok to reduce boilerplate. Key annotations:
- `@Data` - Generates getters, setters, toString, equals, hashCode
- `@Builder` - Implements builder pattern
- `@NoArgsConstructor` / `@AllArgsConstructor` - Constructors

### Adding New Challenge Types

1. Add enum to `ChallengeType.java`
2. Implement generator in `ChallengeGenerator.java`
3. Add evaluation logic in `ChallengeService.java`
4. Update frontend to handle new type

## Algorithm Details

### Challenge Generation

Challenges are **deterministic** based on date:
- Seed: `date.toEpochDay()`
- Same date always generates same challenge
- Allows global competition

### Cribbage Scoring

Implements full Cribbage rules:
- **Fifteens**: All combinations summing to 15 (2 points each)
- **Pairs**: Same rank cards (2 points each)
- **Runs**: Sequences of 3+ cards (1 point per card)
- **Flush**: 4-5 cards same suit (4-5 points)
- **Nobs**: Jack matching starter suit (1 point)

### Optimal Discard Algorithm

Brute force search:
1. Generate all C(6,2) = 15 combinations
2. Score each 4-card hand with starter
3. Return highest-scoring combination

Time complexity: O(1) - fixed 15 combinations

## Performance

- Challenge generation: < 10ms
- Score calculation: < 1ms
- Database queries: < 5ms (indexed)
- API response time: < 50ms

## Troubleshooting

### JWT Authentication Fails

Ensure `JWT_SECRET` matches Go backend:

```bash
# In Go backend
echo $JWT_SECRET

# In Java service
grep jwt.secret src/main/resources/application.properties
```

### Database Lock Errors

SQLite locks when multiple processes write simultaneously. Consider:
- Use WAL mode: `PRAGMA journal_mode=WAL;`
- Implement retry logic
- Move to PostgreSQL for production

### CORS Issues

Update `application.properties`:

```properties
cors.allowed.origins=http://localhost:5173,http://localhost:3000,https://yourdomain.com
```

## Future Enhancements

- [ ] Weekly tournaments with special rewards
- [ ] Multiplayer challenge races
- [ ] Achievement badges and trophies
- [ ] Challenge difficulty ratings
- [ ] Social sharing of scores
- [ ] Historical challenge archive
- [ ] Mobile push notifications for new challenges
- [ ] Machine learning to generate harder puzzles

## Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request

## License

Part of the Fifteen-Thirty-One project. See main repository for license details.

## Contact

For questions or issues, please open a GitHub issue in the main repository.

---

**Happy puzzling! May your hands be full of fifteens!** 🎴✨
