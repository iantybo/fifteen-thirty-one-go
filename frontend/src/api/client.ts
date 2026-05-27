import { apiBaseUrl } from '../lib/env'
import { ApiError, apiFetch } from '../lib/http'
import type {
  Achievement,
  AchievementsSnapshot,
  AuthResponse,
<<<<<<< Updated upstream
  FriendSummary,
  FriendsListResponse,
  Friendship,
=======
  CardTheme,
>>>>>>> Stashed changes
  Game,
  GameMove,
  GameSnapshot,
  LeaderboardResponse,
  Lobby,
  LobbyChatMessage,
  PresenceStatus,
  SpectatorInfo,
  ToggleReactionResponse,
  User,
  UserPreferences,
  UserStats,
} from './types'

type AuthCredentials = { username: string; password: string }
export type RegisterRequest = AuthCredentials
export type LoginRequest = AuthCredentials
export type CreateLobbyRequest = { name: string; max_players: number }
export type GameMoveRequest =
  | { type: 'discard'; cards: string[] }
  | { type: 'play_card'; card: string }
  | { type: 'go' }
export type AddBotRequest = { difficulty?: 'easy' | 'medium' | 'hard' }
export type SendChatMessageRequest = { message: string }
export type UpdatePresenceRequest = { status: 'online' | 'away' | 'in_game' | 'offline' }

const UNEXPECTED_EMPTY_RESPONSE_STATUS = 599

// Most endpoints must return a non-empty body. apiFetch returns undefined for
// 204/empty responses, so this asserts presence and narrows the type.
function required<T>(res: T | undefined): T {
  if (res === undefined || res === null) {
    throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
  }
  return res
}

const url = (path: string) => `${apiBaseUrl()}${path}`

export const api = {
  async register(req: RegisterRequest) {
    return required(await apiFetch<AuthResponse>(url('/api/auth/register'), { method: 'POST', body: req }))
  },
  async login(req: LoginRequest) {
    return required(await apiFetch<AuthResponse>(url('/api/auth/login'), { method: 'POST', body: req }))
  },
  async me() {
    return required(await apiFetch<{ user: User }>(url('/api/auth/me')))
  },
  async logout() {
    // Logout is best-effort; empty 204/empty-body is OK.
    await apiFetch<void>(url('/api/auth/logout'), { method: 'POST' })
  },
  async listLobbies() {
    return required(await apiFetch<{ lobbies: Lobby[] }>(url('/api/lobbies')))
  },
  async createLobby(req: CreateLobbyRequest) {
    return required(
      await apiFetch<{ lobby: Lobby; game: Game }>(url('/api/lobbies'), { method: 'POST', body: req }),
    )
  },
  async joinLobby(lobbyId: number) {
    return required(
      await apiFetch<{
        lobby: Lobby
        game_id: number
        joined_persisted?: boolean
        realtime_sync?: string
      }>(url(`/api/lobbies/${lobbyId}/join`), { method: 'POST' }),
    )
  },
  async addBotToLobby(lobbyId: number, req: AddBotRequest = {}) {
    return required(
      await apiFetch<{ game_id: number; bot_user_id: number; bot_username: string }>(
        url(`/api/lobbies/${lobbyId}/add_bot`),
        { method: 'POST', body: req },
      ),
    )
  },
  async getGame(gameId: number) {
    return required(await apiFetch<GameSnapshot>(url(`/api/games/${gameId}`)))
  },
  async getUserStats(userId: number) {
    return required(await apiFetch<UserStats>(url(`/api/scoreboard/${userId}`)))
  },
  async getLeaderboard(days = 30) {
    const qs = new URLSearchParams({ days: String(days) })
    return required(await apiFetch<LeaderboardResponse>(url(`/api/leaderboard?${qs.toString()}`)))
  },
  async listGameMoves(gameId: number) {
    return required(await apiFetch<{ moves: GameMove[] }>(url(`/api/games/${gameId}/moves`)))
  },
  async quitGame(gameId: number) {
    await apiFetch<void>(url(`/api/games/${gameId}/quit`), { method: 'POST' })
  },
  async nextHand(gameId: number) {
    await apiFetch<void>(url(`/api/games/${gameId}/next_hand`), { method: 'POST' })
  },

  async moveGame(gameId: number, move: GameMoveRequest) {
    return required(
      await apiFetch<unknown>(url(`/api/games/${gameId}/move`), { method: 'POST', body: move }),
    )
  },

  // Lobby chat
  async getLobbyChatHistory(lobbyId: number, limit = 100) {
    return required(
      await apiFetch<{ messages: LobbyChatMessage[] }>(url(`/api/lobbies/${lobbyId}/chat?limit=${limit}`)),
    )
  },
  async sendLobbyChatMessage(lobbyId: number, req: SendChatMessageRequest) {
    return required(
      await apiFetch<LobbyChatMessage>(url(`/api/lobbies/${lobbyId}/chat`), { method: 'POST', body: req }),
    )
  },

  // Spectators
  async joinAsSpectator(lobbyId: number) {
    return required(
      await apiFetch<{ success: boolean; spectator: SpectatorInfo }>(
        url(`/api/lobbies/${lobbyId}/spectate`),
        { method: 'POST' },
      ),
    )
  },
  async leaveAsSpectator(lobbyId: number) {
    return required(
      await apiFetch<{ success: boolean }>(url(`/api/lobbies/${lobbyId}/spectate`), { method: 'DELETE' }),
    )
  },
  async getSpectators(lobbyId: number) {
    return required(
      await apiFetch<{ spectators: SpectatorInfo[] }>(url(`/api/lobbies/${lobbyId}/spectators`)),
    )
  },

  // User presence
  async updatePresence(req: UpdatePresenceRequest) {
    return required(
      await apiFetch<PresenceStatus>(url('/api/users/presence'), { method: 'PUT', body: req }),
    )
  },
  async presenceHeartbeat() {
    return required(
      await apiFetch<{ success: boolean }>(url('/api/users/presence/heartbeat'), { method: 'POST' }),
    )
  },
  async getUserPresence(userId: number) {
    return required(await apiFetch<PresenceStatus>(url(`/api/users/${userId}/presence`)))
  },

  // ---------------------------------------------------------------------------
  // Friends
  // ---------------------------------------------------------------------------
  async listFriends() {
    return required(await apiFetch<FriendsListResponse>(url('/api/friends')))
  },
  async sendFriendRequest(userId: number) {
    return required(
      await apiFetch<{ request: Friendship }>(url('/api/friends/requests'), {
        method: 'POST',
        body: { user_id: userId },
      }),
    )
  },
  async acceptFriendRequest(requestId: number) {
    return required(
      await apiFetch<{ friendship: Friendship }>(url(`/api/friends/requests/${requestId}/accept`), {
        method: 'POST',
      }),
    )
  },
  async declineFriendRequest(requestId: number) {
    return required(
      await apiFetch<{ success: boolean }>(url(`/api/friends/requests/${requestId}/decline`), {
        method: 'POST',
      }),
    )
  },
  async removeFriend(userId: number) {
    return required(
      await apiFetch<{ success: boolean }>(url(`/api/friends/${userId}`), { method: 'DELETE' }),
    )
  },
  async blockUser(userId: number) {
    return required(
      await apiFetch<{ success: boolean }>(url('/api/friends/blocks'), {
        method: 'POST',
        body: { user_id: userId },
      }),
    )
  },
  async unblockUser(userId: number) {
    return required(
      await apiFetch<{ success: boolean }>(url(`/api/friends/blocks/${userId}`), { method: 'DELETE' }),
    )
  },
  async listBlocked() {
    return required(await apiFetch<{ blocked: FriendSummary[] }>(url('/api/friends/blocks')))
  },

  // ---------------------------------------------------------------------------
  // Achievements
  // ---------------------------------------------------------------------------
  async getAchievementCatalogue() {
    return required(await apiFetch<{ achievements: Achievement[] }>(url('/api/achievements/catalogue')))
  },
  async getMyAchievements() {
    return required(await apiFetch<AchievementsSnapshot>(url('/api/achievements')))
  },
  async getUserAchievements(userId: number) {
    return required(await apiFetch<AchievementsSnapshot>(url(`/api/users/${userId}/achievements`)))
  },
  async evaluateMyAchievements() {
    return required(
      await apiFetch<{ newly_unlocked: Achievement[] }>(url('/api/achievements/evaluate'), {
        method: 'POST',
      }),
    )
  },

  // ---------------------------------------------------------------------------
  // Chat reactions
  // ---------------------------------------------------------------------------
  async toggleReaction(lobbyId: number, msgId: number, emoji: string) {
    return required(
      await apiFetch<ToggleReactionResponse>(url(`/api/lobbies/${lobbyId}/chat/${msgId}/react`), {
        method: 'POST',
        body: { emoji },
      }),
    )
  },
  async getMessageReactions(lobbyId: number, msgId: number) {
    return required(
      await apiFetch<{ reactions: ToggleReactionResponse['reactions'] }>(
        url(`/api/lobbies/${lobbyId}/chat/${msgId}/reactions`),
      ),
    )
  },

  // Preferences
  async getPreferences() {
    const res = await apiFetch<UserPreferences>(`${apiBaseUrl()}/api/me/preferences`)
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
  async setCardTheme(theme: CardTheme) {
    const res = await apiFetch<UserPreferences>(`${apiBaseUrl()}/api/me/preferences`, {
      method: 'PUT',
      body: { card_theme: theme },
    })
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },

  // Reactions
  async sendReaction(gameId: number, emoji: string) {
    const res = await apiFetch<{ ok: boolean }>(`${apiBaseUrl()}/api/games/${gameId}/reaction`, {
      method: 'POST',
      body: { emoji },
    })
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
}
