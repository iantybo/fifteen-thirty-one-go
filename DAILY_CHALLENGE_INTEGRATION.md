# Daily Challenge Service - Integration Guide

## Overview

The **Daily Challenge Service** is a fun Java microservice that generates daily Cribbage puzzle challenges for users. It integrates seamlessly with your existing Go backend and shares the same SQLite database.

## What Makes This Fun for Users? 🎯

### Engaging Daily Puzzles
- **New challenge every day** - Automatically generated at midnight UTC
- **Three challenge types** to keep things interesting:
  - 🃏 **Optimal Discard** - "Which 2 cards should you throw to maximize your score?"
  - 🔢 **Max Score Hunt** - "What's the exact score of this hand?"
  - 🎮 **Best Peg Play** - "Which card gives you the best advantage?"

### Competitive Elements
- **Daily Leaderboard** - See who solved today's challenge fastest
- **Global Leaderboard** - All-time top players
- **Speed Bonuses** - Solve under 30 seconds for 150% points!
- **Streak Tracking** - Build your consecutive days streak

### Personal Progress
- Track your accuracy rate
- Build and maintain streaks
- Earn points for every correct answer
- See your improvement over time

## Quick Start

### 1. Run Database Migration

```bash
cd /Users/ianbowers/git/fifteen-thirty-one-go
sqlite3 backend/app.db < backend/internal/database/migrations/003_daily_challenges.sql
```

This creates two new tables:
- `challenge_submissions` - User submissions
- `user_challenge_stats` - Aggregate statistics

### 2. Set JWT Secret

The service needs to validate JWT tokens from your Go backend:

```bash
export JWT_SECRET="your-secret-key-matching-go-backend"
```

### 3. Start the Service

**Option A: Using the run script**
```bash
cd daily-challenge-service
./run.sh
```

**Option B: Maven directly**
```bash
cd daily-challenge-service
mvn spring-boot:run
```

The service starts on **http://localhost:8081**

### 4. Test It Out

```bash
# Get today's challenge (no auth required)
curl http://localhost:8081/api/challenges/today

# Submit a solution (requires JWT token)
curl -X POST http://localhost:8081/api/challenges/submit \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"answer": "[2,5]", "timeTakenSeconds": 45}'

# View today's leaderboard
curl http://localhost:8081/api/challenges/leaderboard/today
```

## API Endpoints

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/api/challenges/today` | GET | No | Get today's challenge |
| `/api/challenges/{date}` | GET | No | Get challenge for specific date |
| `/api/challenges/submit` | POST | Yes | Submit solution |
| `/api/challenges/leaderboard/today` | GET | No | Daily leaderboard |
| `/api/challenges/leaderboard/global` | GET | No | All-time leaderboard |
| `/api/challenges/stats/me` | GET | Yes | Your statistics |
| `/api/challenges/stats/{userId}` | GET | No | User statistics |

## Frontend Integration

### 1. Add Challenge Page

Create a new React component:

```typescript
// frontend/src/pages/DailyChallengePage.tsx
import { useState, useEffect } from 'react';
import { apiClient } from '../api/client';

export function DailyChallengePage() {
  const [challenge, setChallenge] = useState(null);
  const [startTime, setStartTime] = useState(Date.now());

  useEffect(() => {
    fetch('http://localhost:8081/api/challenges/today')
      .then(res => res.json())
      .then(data => {
        setChallenge(data);
        setStartTime(Date.now());
      });
  }, []);

  const submitAnswer = async (answer: string) => {
    const timeTaken = Math.floor((Date.now() - startTime) / 1000);

    const response = await apiClient.post(
      'http://localhost:8081/api/challenges/submit',
      {
        answer,
        timeTakenSeconds: timeTaken
      }
    );

    if (response.correct) {
      alert(`Correct! You earned ${response.pointsEarned} points!`);
    } else {
      alert(`Incorrect. The answer was: ${response.correctAnswer}`);
    }
  };

  // Render challenge UI based on challenge.type
  return <div>...</div>;
}
```

### 2. Add Route

```typescript
// frontend/src/main.tsx or routes config
<Route path="/daily-challenge" element={<DailyChallengePage />} />
```

### 3. Add Navigation Link

```typescript
// frontend/src/components/Navigation.tsx
<Link to="/daily-challenge">Daily Challenge 🎯</Link>
```

## Challenge Types & Answer Formats

### Optimal Discard
**Question:** "Which 2 cards should you discard?"

**Answer Format:** JSON array of indices `[2, 5]`
- Indices are 0-based
- Must select exactly 2 cards

**Example:**
```json
{
  "answer": "[2,5]",
  "timeTakenSeconds": 45
}
```

### Max Score Hunt
**Question:** "What's the total score of this hand?"

**Answer Format:** Integer as string `"24"`

**Example:**
```json
{
  "answer": "24",
  "timeTakenSeconds": 32
}
```

### Best Peg Play
**Question:** "Which card should you play?"

**Answer Format:** Card index as string `"1"`
- Index is 0-based

**Example:**
```json
{
  "answer": "1",
  "timeTakenSeconds": 18
}
```

## Scoring System

### Base Points
- Correct answer: **100 points**
- Incorrect answer: **0 points**

### Speed Bonuses
- Under 30 seconds: **+50%** (150 points total)
- Under 60 seconds: **+30%** (130 points total)
- Under 120 seconds: **+10%** (110 points total)
- Over 120 seconds: No bonus (100 points)

### Streaks
- Solve challenges on consecutive days to build your streak
- Your longest streak is tracked and displayed on leaderboards
- Break a streak by missing a day or getting an answer wrong

## Architecture

### How Challenges Are Generated

1. **Deterministic Randomness** - Same date always generates same challenge
   - Seed: `date.toEpochDay()`
   - Ensures all players worldwide get the same challenge

2. **Challenge Rotation** - Type rotates based on day of week
   - Monday, Thursday, Sunday: Optimal Discard
   - Tuesday, Friday: Max Score Hunt
   - Wednesday, Saturday: Best Peg Play

3. **Difficulty Calibration** - Automatically balanced
   - Challenges are generated until they fall within target difficulty range
   - Too easy or too hard challenges are regenerated

### Database Schema

**challenge_submissions**
```sql
id, user_id, challenge_id, answer, points_earned,
is_correct, time_taken_seconds, submitted_at
```

**user_challenge_stats**
```sql
id, user_id, total_challenges_completed, total_challenges_correct,
current_streak, longest_streak, total_points, last_completed_date
```

### Technology Stack
- **Spring Boot 3.2.1** - REST framework
- **Java 17** - Language
- **SQLite** - Shared database
- **JWT** - Authentication (validates Go backend tokens)
- **Maven** - Build tool

## Deployment

### Development
```bash
# Run locally alongside Go backend
cd daily-challenge-service
mvn spring-boot:run
```

### Production (Docker)

Create `Dockerfile`:
```dockerfile
FROM eclipse-temurin:17-jre
WORKDIR /app
COPY target/*.jar app.jar
EXPOSE 8081
CMD ["java", "-jar", "app.jar"]
```

Build and run:
```bash
mvn clean package
docker build -t daily-challenge-service .
docker run -p 8081:8081 \
  -e JWT_SECRET="your-secret" \
  -v /path/to/app.db:/app/app.db \
  daily-challenge-service
```

### Running Both Services

**Terminal 1 - Go Backend:**
```bash
cd backend
go run cmd/server/main.go
# Runs on :8080
```

**Terminal 2 - Java Challenge Service:**
```bash
cd daily-challenge-service
./run.sh
# Runs on :8081
```

**Terminal 3 - Frontend:**
```bash
cd frontend
npm run dev
# Runs on :5173
```

## Troubleshooting

### JWT Token Validation Fails

**Problem:** Getting 401 Unauthorized when submitting solutions

**Solution:** Ensure JWT_SECRET matches between services

```bash
# Check Go backend secret
grep JWT_SECRET backend/.env

# Set same secret for Java service
export JWT_SECRET="same-secret-as-go-backend"
```

### Database Table Not Found

**Problem:** Error about missing `challenge_submissions` table

**Solution:** Run the migration
```bash
sqlite3 backend/app.db < backend/internal/database/migrations/003_daily_challenges.sql
```

### CORS Errors

**Problem:** Browser blocks requests from frontend

**Solution:** Update `application.properties`
```properties
cors.allowed.origins=http://localhost:5173,http://localhost:3000
```

### Port Already in Use

**Problem:** Port 8081 is already taken

**Solution:** Change port in `application.properties`
```properties
server.port=8082
```

## Future Enhancements

Ideas to make it even more fun:

- [ ] **Weekly Tournaments** - Special challenges with big prizes
- [ ] **Multiplayer Races** - First to solve wins
- [ ] **Achievement Badges** - "Perfect Week", "Speed Demon", "Century Club"
- [ ] **Social Sharing** - Share your scores on Twitter/Discord
- [ ] **Challenge Archive** - Browse and replay past challenges
- [ ] **Difficulty Levels** - Easy/Medium/Hard modes
- [ ] **Custom Challenges** - Create and share your own puzzles
- [ ] **Mobile App** - Native iOS/Android with push notifications

## Testing

### Run Unit Tests
```bash
cd daily-challenge-service
mvn test
```

### Integration Test
```bash
# 1. Start the service
mvn spring-boot:run

# 2. In another terminal
curl http://localhost:8081/api/challenges/today
```

Expected response:
```json
{
  "challengeId": "OD-2026-01-12",
  "date": "2026-01-12",
  "type": "OPTIMAL_DISCARD",
  "cards": [...],
  "maxPoints": 100,
  "difficulty": 3
}
```

## Support

For issues or questions:
1. Check the README in `daily-challenge-service/`
2. Review the Troubleshooting section above
3. Open a GitHub issue

## Summary

The Daily Challenge Service adds a fun, competitive element to your Cribbage game:

✅ **Engaging daily puzzles** keep users coming back
✅ **Leaderboards** create friendly competition
✅ **Streaks** encourage daily engagement
✅ **Speed bonuses** reward quick thinking
✅ **Three challenge types** provide variety
✅ **Seamless integration** with existing auth/database
✅ **Deterministic generation** ensures fairness

**Get started now:**
```bash
cd daily-challenge-service && ./run.sh
```

Happy coding! 🎴✨
