# 🎴 Cribbage Hand Analyzer

A fun and interactive Java utility that analyzes cribbage hands, calculates scores with detailed breakdowns, and provides interesting statistics and probabilities!

## ✨ Features

- **Accurate Cribbage Scoring**: Calculates points for fifteens, pairs, runs, flushes, and nobs
- **Detailed Breakdowns**: Shows exactly how each point was earned
- **Fun Statistics**: Get hand quality ratings, probability info, and interesting patterns
- **Interactive CLI**: Multiple modes including random hand analysis, custom hands, and simulations
- **Perfect Hand Demo**: See the legendary 29-point hand explained
- **Batch Simulation**: Analyze hundreds of hands to see score distributions

## 🚀 Quick Start

### Prerequisites

- Java 17 or higher
- Maven 3.6+

### Build the Project

```bash
cd cribbage-analyzer
mvn clean package
```

### Run the Demo

```bash
mvn exec:java -Dexec.mainClass="com.cribbagegame.analyzer.CribbageAnalyzerDemo"
```

Or run the compiled JAR:

```bash
java -jar target/cribbage-analyzer-1.0.0.jar
```

## 🎮 Usage Examples

### Interactive Menu

The demo provides an interactive menu with several options:

```
Choose an option:
  1. 🎲 Analyze a random hand
  2. ✍️  Analyze a custom hand
  3. 🏆 Show the perfect 29-point hand
  4. 📊 Simulate multiple hands
  5. 📖 Scoring guide
  6. 🚪 Quit
```

### Example Output

```
🎴 Your Hand:
   5♣  5♦  10♠  J♥

⭐ Starter: 5♥

──────────────────────────────────────────────────
📊 SCORE BREAKDOWN:
──────────────────────────────────────────────────
   Fifteen: 2 points (5♣ + 10♠)
   Fifteen: 2 points (5♦ + 10♠)
   Fifteen: 2 points (5♥ + 10♠)
   Fifteen: 2 points (5♣ + 5♦ + 5♥)
   Pair: 6 points (3 × 5)
   Nobs: 1 point (Jack of ♥)
──────────────────────────────────────────────────
🎯 TOTAL SCORE: 15 points
──────────────────────────────────────────────────

✨ Excellent! (Top 10% of hands)

📈 16+ point hands happen in roughly 5% of deals. Well done!

✨ Interesting patterns:
   💡 Fifteen heaven - Multiple fifteen combinations!
   🎴 Jack-Five combo: The foundation of the perfect 29-point hand!

💡 Fun fact: Fives are the most valuable cards in cribbage!
```

### Custom Hand Entry

When entering custom hands, use this format:
- **Ranks**: A, 2, 3, 4, 5, 6, 7, 8, 9, 10, J, Q, K
- **Suits**: C (♣), D (♦), H (♥), S (♠)
- **Examples**: `5H`, `JC`, `QD`, `AS`, `10H`

### Simulation Mode

Analyze hundreds of hands to see statistical distributions:

```
📈 RESULTS:
   Average score: 7.85 points
   Highest score: 24 points
   Lowest score:  0 points
   Zero-point hands: 48 (9.6%)

📊 Score Distribution:
    0 pts: ████ 48 hands (9.6%)
    2 pts: ████████ 87 hands (17.4%)
    4 pts: ██████████ 103 hands (20.6%)
    6 pts: ████████ 89 hands (17.8%)
    8 pts: ██████ 67 hands (13.4%)
   10 pts: ████ 45 hands (9.0%)
   ...
```

## 🎯 Cribbage Scoring Rules

### Fifteens (2 points each)
Any combination of cards that sum to 15.

### Pairs (2 points per pair)
- Two of a kind: 2 points
- Three of a kind: 6 points (3 pairs)
- Four of a kind: 12 points (6 pairs)

### Runs (1 point per card)
Three or more consecutive cards. Double/triple runs count multiple times.

### Flush (4 or 5 points)
- 4 cards in hand same suit: 4 points
- 5 cards including starter: 5 points
- In crib: requires all 5 cards

### Nobs (1 point)
Jack in hand matching the starter card's suit.

## 🏆 Fun Facts

- **Perfect Hand**: The 29-point hand (J-5-5-5 with matching 5 starter) occurs once in 216,580 hands!
- **Impossible Scores**: You can never score 19, 25, 26, 27, or 28 points
- **Average Score**: Most hands score around 7-8 points
- **Zero Hands**: About 10% of hands score nothing
- **Best Card**: Fives are the most valuable due to fifteen-making potential

## 📚 Using as a Library

You can use the analyzer programmatically in your own Java projects:

```java
import com.cribbagegame.analyzer.*;
import java.util.*;

// Create cards
List<Card> hand = Arrays.asList(
    new Card(Card.Rank.FIVE, Card.Suit.CLUBS),
    new Card(Card.Rank.FIVE, Card.Suit.DIAMONDS),
    new Card(Card.Rank.JACK, Card.Suit.HEARTS),
    new Card(Card.Rank.TEN, Card.Suit.SPADES)
);
Card starter = new Card(Card.Rank.FIVE, Card.Suit.HEARTS);

// Analyze the hand
CribbageHandAnalyzer analyzer = new CribbageHandAnalyzer();
HandScore score = analyzer.scoreHand(hand, starter, false);

System.out.println("Total: " + score.getTotalScore() + " points");

// Get statistics
HandStatistics.Statistics stats = HandStatistics.analyze(hand, starter, score);
System.out.println(stats.getHandQuality());
System.out.println(stats.getFunFact());
```

## 🔧 Project Structure

```
cribbage-analyzer/
├── src/
│   └── main/
│       └── java/
│           └── com/cribbagegame/analyzer/
│               ├── Card.java                    # Card representation
│               ├── Deck.java                    # Deck management
│               ├── HandScore.java               # Score data structure
│               ├── CribbageHandAnalyzer.java    # Core scoring logic
│               ├── HandStatistics.java          # Fun stats & insights
│               └── CribbageAnalyzerDemo.java    # Interactive CLI
├── pom.xml
└── README.md
```

## 🎨 Why This is Fun

1. **Learn by Playing**: See exactly how cribbage hands are scored
2. **Discover Patterns**: Find interesting card combinations and rare hands
3. **Understand Probabilities**: Learn what makes a good vs great hand
4. **Test Your Knowledge**: Enter custom hands to verify your counting skills
5. **Data Driven**: Run simulations to see real statistics

## 🤝 Integration with Main Project

This analyzer can be used alongside the main Fifteen-Thirty-One Go project:

- **Hand Validation**: Verify scoring in the main game
- **Bot AI**: Help bots evaluate hand strength
- **Learning Tool**: Help players understand scoring
- **Testing**: Generate test cases for the Go backend
- **Statistics**: Track and analyze game history

## 📝 License

This is part of the Fifteen-Thirty-One project. Feel free to use and modify!

## 🎲 Have Fun!

Whether you're learning cribbage, testing your counting skills, or just curious about probabilities, this analyzer makes exploring the game fun and educational. Enjoy!
