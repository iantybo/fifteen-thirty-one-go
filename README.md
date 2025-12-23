# Fifteen-Thirty-One

Cribbage (first), with a platform designed to grow into **multiple card games** (e.g. Spades, Poker) over time.

## Player Experience Goals

### Core gameplay
- **Play in the browser** with a modern UI that works on desktop first (mobile-friendly later).
- **Multi-player online** via lobbies: create, browse, join, and start games.
- **Play vs a bot** with multiple difficulty levels.
- **Real-time play** (live updates, turn indicators, reconnect support).

### Scoring & counting (cribbage)
- **Auto-count or manual count** at the moment a hand/crib is counted (player-selectable).
- **Auto-count** shows a full breakdown (15s, pairs, runs, flush, nobs) and can be applied with one click.
- **Manual count** lets a player enter a score (and optionally the breakdown).
- **Miscount handling (configurable)**:
  - The server always computes the **actual** score.
  - If a player’s manual count is wrong, the server can **automatically award the difference to the opponent** (default), or use another policy (off / “muggins”-style / review).
  - This behavior is **changeable per game/lobby** (host setting) and is logged for transparency.
- **Count correction**: clear audit trail of what was claimed, what was computed, and what was ultimately applied.

### Lobbies & social
- Lobby settings: max players (2–4), bot slots, bot difficulty, scoring policy toggles.
- Game history and a **scoreboard** (wins, games played, rating/leaderboard later).

### Trust, fairness, and clarity
- Server-authoritative rules and scoring.
- All scoring events show “who got points, why, and when”.
- Anti-confusion UX: highlight legal moves, show pegging total, show last action, and show “pending confirmation” states.

## Architecture (high level)
- **Backend**: Go + Gin, SQLite (local), JWT auth, WebSockets for real-time gameplay.
- **Frontend**: React + TypeScript + Vite, **TanStack React Query** for API state, WebSockets for game state.
- **Designed for extension**: a game-engine interface and registry so we can add other games later without rewriting the platform.

## Go version
- **Go toolchain**: Go **1.25.5** (see `backend/go.mod` `go`/`toolchain` directives).

## Public Repo / Security Rules (important)
This repository is **public**. Do not commit secrets.

- Use `.env` files locally; commit only `.env.example`.
- JWT secrets, ngrok auth tokens, and database files must never be checked in.
- Prefer configuration through environment variables; provide safe defaults for local dev.

## Local Development (planned)
Once scaffolding is in place:
- Backend will run locally (SQLite file DB).
- Frontend will run locally (Vite dev server).
- We’ll expose the backend (or a reverse proxy) via ngrok for testing.

