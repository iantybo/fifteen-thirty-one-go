# Player history & stats API

All endpoints below require authentication and operate on the calling player. Each
returns `401` when no authenticated user is present.

Invalid query values are rejected with `400` rather than ignored, so a typo surfaces to
the caller instead of silently returning unfiltered or default-paged results.

## `GET /api/games/history`

The player's finished games, most recently completed first.

| Query param | Type | Default | Notes |
| --- | --- | --- | --- |
| `limit` | int | 20 | Capped at 100 |
| `offset` | int | 0 | Must be >= 0 |
| `result` | `wins` \| `losses` | *(unset)* | Filter by outcome |
| `opponent_id` | int | *(unset)* | Only games shared with this player |
| `since` | RFC3339 | *(unset)* | Inclusive lower bound on completion time |
| `until` | RFC3339 | *(unset)* | Inclusive upper bound on completion time |

Filters combine. `total` reflects the filtered set, so paging over a filtered view stays
consistent. An `until` earlier than `since` is a `400`.

```json
{
  "items": [
    {
      "game_id": 42,
      "lobby_id": 7,
      "started_at": "2026-08-01T10:00:00Z",
      "finished_at": "2026-08-01T10:35:00Z",
      "my_score": 121,
      "my_position": 1,
      "won": true,
      "opponents": [
        {"user_id": 9, "username": "rival", "score": 98, "position": 2, "is_bot": false}
      ]
    }
  ],
  "limit": 20,
  "offset": 0,
  "total": 1
}
```

## `GET /api/me/stats`

Aggregate summary across the player's finished games. Figures are derived from
`scoreboard` rows rather than the denormalized `users.games_played` / `users.games_won`
counters, so a drifted counter cannot skew them. A player with no finished games gets a
zero-valued summary, not a `404`.

`current_streak` is the unbroken run of wins ending at the most recent game (0 if that
game was a loss); `longest_streak` is the best run at any point in the player's history.

```json
{
  "user_id": 1,
  "games_played": 12,
  "games_won": 7,
  "win_rate": 0.5833333333333334,
  "best_score": 121,
  "worst_score": 64,
  "average_score": 104.25,
  "total_points": 1251,
  "current_streak": 2,
  "longest_streak": 4
}
```

## `GET /api/me/head_to_head`

Win/loss record against each opponent the player has shared a finished game with, ordered
by games played descending. Accepts `limit` (default 20, capped at 100).

A result is decided by comparing the two players' standings positions within the same
game, not by testing `position == 1`, so in a game with three or more players the record
still reflects who finished ahead of whom. `point_diff` is the player's cumulative final
score minus the opponent's across their shared games.

```json
{
  "items": [
    {
      "user_id": 9,
      "username": "rival",
      "is_bot": false,
      "games": 8,
      "wins": 5,
      "losses": 3,
      "win_rate": 0.625,
      "point_diff": 42
    }
  ],
  "limit": 20
}
```

## `GET /api/games/:id/recap`

Full post-game view of one game: metadata, final standings, and the move timeline in
chronological (oldest-first) order — the opposite of `GET /api/games/:id/moves`, which
returns newest-first.

Restricted to participants of the game; a non-participant gets `403`, since a recap
exposes every player's moves. An unknown game id is a `404`.

The timeline is capped at 500 entries. `move_count` is the true total, and `truncated` is
`true` when the timeline was cut short. For a game with no `scoreboard` rows yet,
`standings` falls back to provisional placements ranked by current score.

```json
{
  "game_id": 42,
  "lobby_id": 7,
  "lobby_name": "Friday night",
  "status": "finished",
  "started_at": "2026-08-01T10:00:00Z",
  "finished_at": "2026-08-01T10:35:00Z",
  "standings": [
    {"user_id": 1, "username": "me", "is_bot": false, "score": 121, "position": 1}
  ],
  "moves": [
    {
      "id": 1,
      "player_id": 1,
      "username": "me",
      "is_bot": false,
      "move_type": "play",
      "card_played": "5H",
      "is_corrected": false,
      "created_at": "2026-08-01T10:01:00Z"
    }
  ],
  "move_count": 1,
  "truncated": false
}
```
