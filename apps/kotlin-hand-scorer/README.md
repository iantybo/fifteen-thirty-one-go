# kotlin-hand-scorer

A command-line tool that scores a cribbage hand (four cards plus the starter).

## Build and run

```bash
cd apps/kotlin-hand-scorer
gradle run --args="5H 5D JS 10C 5S"
gradle run --args="--crib 2H 3H 4H 9H KS"
```

## Scoring rules

- **Fifteens**: 2 points for each combination of cards that adds up to 15
- **Pairs**: 2 points for each pair
- **Runs**: 1 point per card in each run of three or more
- **Flush**: 4 points if all four hand cards share a suit, 5 if the starter matches too.
  In the crib, only a five-card flush scores.
- **Nobs**: 1 point for a jack in the hand that matches the starter's suit
