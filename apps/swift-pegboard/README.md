# swift-pegboard

A Swift package that models a two-player cribbage pegboard: front and back
pegs, undo, autosave, and game-over detection with skunks.

## Build and run

```bash
cd apps/swift-pegboard
swift run pegboard alice:12 bob:8 alice:24
```

## Rules

- The first player to reach 121 wins.
- If the loser has fewer than 91 points, it is a **skunk**.
- If the loser has fewer than 61 points, it is a **double skunk**.
