import { apiBaseUrl } from '../lib/env'
import { ApiError, apiFetch } from '../lib/http'
import type { AuthResponse, Game, GameSnapshot, Lobby, User } from './types'

type AuthCredentials = { username: string; password: string }
export type RegisterRequest = AuthCredentials
export type LoginRequest = AuthCredentials
export type CreateLobbyRequest = { name: string; max_players: number }

export const api = {
  async register(req: RegisterRequest) {
    const res = await apiFetch<AuthResponse>(`${apiBaseUrl()}/api/auth/register`, {
      method: 'POST',
      body: req,
    })
    if (!res) throw new ApiError('Empty response', 200)
    return res
  },
  async login(req: LoginRequest) {
    const res = await apiFetch<AuthResponse>(`${apiBaseUrl()}/api/auth/login`, {
      method: 'POST',
      body: req,
    })
    if (!res) throw new ApiError('Empty response', 200)
    return res
  },
  async me() {
    const res = await apiFetch<{ user: User }>(`${apiBaseUrl()}/api/auth/me`)
    if (!res) throw new ApiError('Empty response', 200)
    return res
  },
  async logout() {
    // Logout is best-effort; empty 204/empty-body is OK.
    await apiFetch<void>(`${apiBaseUrl()}/api/auth/logout`, { method: 'POST' })
  },
  async listLobbies() {
    const res = await apiFetch<{ lobbies: Lobby[] }>(`${apiBaseUrl()}/api/lobbies`)
    if (!res) throw new ApiError('Empty response', 200)
    return res
  },
  async createLobby(req: CreateLobbyRequest) {
    const res = await apiFetch<{ lobby: Lobby; game: Game }>(`${apiBaseUrl()}/api/lobbies`, {
      method: 'POST',
      body: req,
    })
    if (!res) throw new ApiError('Empty response', 200)
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
    if (!res) throw new ApiError('Empty response', 200)
    return res
  },
  async getGame(gameId: number) {
    const res = await apiFetch<GameSnapshot>(`${apiBaseUrl()}/api/games/${gameId}`)
    if (!res) throw new ApiError('Empty response', 200)
    return res
  },
}


