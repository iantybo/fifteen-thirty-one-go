import { apiBaseUrl } from '../lib/env'
import { ApiError, apiFetch } from '../lib/http'
import type {
  AuthResponse,
  ChatbotRequest,
  ChatbotResponse,
  Game,
  GameMove,
  GameSnapshot,
  LeaderboardResponse,
  Lobby,
  LobbyChatMessage,
  PresenceStatus,
  SpectatorInfo,
  User,
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

export const api = {
  async register(req: RegisterRequest) {
    const res = await apiFetch<AuthResponse>(`${apiBaseUrl()}/api/auth/register`, {
      method: 'POST',
      body: req,
    })
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
  async login(req: LoginRequest) {
    const res = await apiFetch<AuthResponse>(`${apiBaseUrl()}/api/auth/login`, {
      method: 'POST',
      body: req,
    })
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
  async me() {
    const res = await apiFetch<{ user: User }>(`${apiBaseUrl()}/api/auth/me`)
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
  async logout() {
    // Logout is best-effort; empty 204/empty-body is OK.
    await apiFetch<void>(`${apiBaseUrl()}/api/auth/logout`, { method: 'POST' })
  },
  async listLobbies() {
    const res = await apiFetch<{ lobbies: Lobby[] }>(`${apiBaseUrl()}/api/lobbies`)
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
  async createLobby(req: CreateLobbyRequest) {
    const res = await apiFetch<{ lobby: Lobby; game: Game }>(`${apiBaseUrl()}/api/lobbies`, {
      method: 'POST',
      body: req,
    })
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
  async joinLobby(lobbyId: number) {
    const res = await apiFetch<{
      lobby: Lobby
      game_id: number
      joined_persisted?: boolean
      realtime_sync?: string
    }>(`${apiBaseUrl()}/api/lobbies/${lobbyId}/join`, {
      method: 'POST',
    })
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
  async addBotToLobby(lobbyId: number, req: AddBotRequest = {}) {
    const res = await apiFetch<{ game_id: number; bot_user_id: number; bot_username: string }>(
      `${apiBaseUrl()}/api/lobbies/${lobbyId}/add_bot`,
      {
        method: 'POST',
        body: req,
      },
    )
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
  async getGame(gameId: number) {
    const res = await apiFetch<GameSnapshot>(`${apiBaseUrl()}/api/games/${gameId}`)
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
  async getUserStats(userId: number) {
    const res = await apiFetch<UserStats>(`${apiBaseUrl()}/api/scoreboard/${userId}`)
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
  async getLeaderboard(days = 30) {
    const qs = new URLSearchParams()
    qs.set('days', String(days))
    const res = await apiFetch<LeaderboardResponse>(`${apiBaseUrl()}/api/leaderboard?${qs.toString()}`)
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
  async listGameMoves(gameId: number) {
    const res = await apiFetch<{ moves: GameMove[] }>(`${apiBaseUrl()}/api/games/${gameId}/moves`)
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
  async quitGame(gameId: number) {
    await apiFetch<void>(`${apiBaseUrl()}/api/games/${gameId}/quit`, { method: 'POST' })
  },
  async nextHand(gameId: number) {
    await apiFetch<void>(`${apiBaseUrl()}/api/games/${gameId}/next_hand`, { method: 'POST' })
  },

  async moveGame(gameId: number, move: GameMoveRequest) {
    const res = await apiFetch<unknown>(`${apiBaseUrl()}/api/games/${gameId}/move`, {
      method: 'POST',
      body: move,
    })
    if (res === undefined) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },

  // Lobby chat
  async getLobbyChatHistory(lobbyId: number, limit = 100) {
    const res = await apiFetch<{ messages: LobbyChatMessage[] }>(`${apiBaseUrl()}/api/lobbies/${lobbyId}/chat?limit=${limit}`)
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
  async sendLobbyChatMessage(lobbyId: number, req: SendChatMessageRequest) {
    const res = await apiFetch<LobbyChatMessage>(`${apiBaseUrl()}/api/lobbies/${lobbyId}/chat`, {
      method: 'POST',
      body: req,
    })
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },

  // Spectators
  async joinAsSpectator(lobbyId: number) {
    const res = await apiFetch<{ success: boolean; spectator: SpectatorInfo }>(`${apiBaseUrl()}/api/lobbies/${lobbyId}/spectate`, {
      method: 'POST',
    })
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
  async leaveAsSpectator(lobbyId: number) {
    const res = await apiFetch<{ success: boolean }>(`${apiBaseUrl()}/api/lobbies/${lobbyId}/spectate`, {
      method: 'DELETE',
    })
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
  async getSpectators(lobbyId: number) {
    const res = await apiFetch<{ spectators: SpectatorInfo[] }>(`${apiBaseUrl()}/api/lobbies/${lobbyId}/spectators`)
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },

  // User presence
  async updatePresence(req: UpdatePresenceRequest) {
    const res = await apiFetch<PresenceStatus>(`${apiBaseUrl()}/api/users/presence`, {
      method: 'PUT',
      body: req,
    })
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
  async presenceHeartbeat() {
    const res = await apiFetch<{ success: boolean }>(`${apiBaseUrl()}/api/users/presence/heartbeat`, {
      method: 'POST',
    })
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
  async getUserPresence(userId: number) {
    const res = await apiFetch<PresenceStatus>(`${apiBaseUrl()}/api/users/${userId}/presence`)
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },

  // Chatbot for games with bot opponents
  async sendChatbotMessage(gameId: number, req: ChatbotRequest) {
    const res = await apiFetch<ChatbotResponse>(`${apiBaseUrl()}/api/games/${gameId}/chatbot`, {
      method: 'POST',
      body: req,
    })
    if (!res) throw new ApiError('Unexpected empty response', UNEXPECTED_EMPTY_RESPONSE_STATUS)
    return res
  },
}

