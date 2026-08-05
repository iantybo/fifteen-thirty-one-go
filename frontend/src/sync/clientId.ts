/**
 * Stable, deterministic client-action id generation.
 *
 * Every optimistic action gets a `clientId` that is:
 *  - **stable across retries** (the same logical action keeps its id), so the
 *    server can dedupe and the engine can correlate acknowledgements, and
 *  - **deterministic** (no `Math.random`), so tests are reproducible.
 *
 * The format is `c:<gameId>:<userId>:<seq>` where `seq` is a per-engine
 * monotonic counter. `IdGenerator` encapsulates that counter so the engine does
 * not hand-roll id strings.
 */

/** Parsed components of a client id. */
export type ParsedClientId = {
  gameId: number
  userId: number
  seq: number
}

const PREFIX = 'c'

/** A per-engine monotonic client-id generator. */
export class IdGenerator {
  private seq = 0

  constructor(
    private readonly gameId: number,
    private readonly userId: number,
  ) {}

  /** Produce the next unique client id for this game/user. */
  next(): string {
    this.seq += 1
    return `${PREFIX}:${this.gameId}:${this.userId}:${this.seq}`
  }

  /** The current sequence value (for diagnostics/tests). */
  current(): number {
    return this.seq
  }
}

/** Parse a client id back into components; returns null on malformed input. */
export function parseClientId(id: string): ParsedClientId | null {
  const parts = id.split(':')
  if (parts.length !== 4 || parts[0] !== PREFIX) return null
  const gameId = Number(parts[1])
  const userId = Number(parts[2])
  const seq = Number(parts[3])
  if (!Number.isFinite(gameId) || !Number.isFinite(userId) || !Number.isFinite(seq)) return null
  return { gameId, userId, seq }
}

/** True when `id` is a well-formed client id. */
export function isClientId(id: string): boolean {
  return parseClientId(id) !== null
}
