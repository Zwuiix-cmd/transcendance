import type { ApiError, AuthResponse, Game, GameDetail, GameSettings, LeaderboardEntry, MatchmakingStatus, ProfileSearchResult, QueueMode, UnoActionRequest, UnoSnapshot, User, UserAnalytics } from '../types'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

export class ApiClientError extends Error {
  readonly status: number
  readonly code?: string

  constructor(message: string, status: number, code?: string) {
    super(message)
    this.name = 'ApiClientError'
    this.status = status
    this.code = code
  }
}

class ApiClient {
  private token: string | null = null
  private readonly inFlight = new Map<string, Promise<unknown>>()

  setToken(token: string | null) {
    this.token = token
  }

  async register(username: string, email: string, password: string) {
    return this.request<AuthResponse>('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ username, email, password }),
    })
  }

  async login(email: string, password: string) {
    return this.request<AuthResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    })
  }

  async logout() {
    await this.request<void>('/auth/logout', { method: 'POST' })
  }

  me() {
    return this.request<User>('/auth/me')
  }

  updateProfile(bio: string) {
    return this.request<User>('/users/me', {
      method: 'PUT',
      body: JSON.stringify({ bio }),
    })
  }

  async changePassword(currentPassword: string, newPassword: string) {
    await this.request<void>('/users/me/password', {
      method: 'PUT',
      body: JSON.stringify({ currentPassword, newPassword }),
    })
  }

  async uploadAvatar(file: File) {
    const form = new FormData()
    form.append('avatar', await optimizeAvatar(file))
    return this.request<User>('/users/me/avatar', {
      method: 'PUT',
      body: form,
    })
  }

  async deleteProfile() {
    await this.request<void>('/users/me', { method: 'DELETE' })
  }

  listGames(status?: Game['status'], page = 1, limit = 20) {
    const query = new URLSearchParams({ page: String(page), limit: String(limit) })
    if (status) query.set('status', status)
    return this.request<Game[]>(`/games?${query.toString()}`)
  }

  createGame(turnTimeLimit: number, privateGame = false, mode: 'ranked' | 'unranked' | 'custom' = 'custom', settings?: Partial<GameSettings>) {
    return this.request<Game>('/games', {
      method: 'POST',
      body: JSON.stringify({ turnTimeLimit, privateGame, mode, ...settings }),
    })
  }

  joinMatchmaking(mode: QueueMode) {
    return this.request<MatchmakingStatus>('/matchmaking/queue', {
      method: 'POST',
      body: JSON.stringify({ mode }),
    })
  }

  matchmakingStatus() {
    return this.request<MatchmakingStatus>('/matchmaking/queue')
  }

  async leaveMatchmaking() {
    await this.request<void>('/matchmaking/queue', { method: 'DELETE' })
  }

  getGame(id: string) {
    return this.request<GameDetail>(`/games/${encodeURIComponent(id)}`)
  }

  updateGameStatus(id: string, status: Extract<Game['status'], 'waiting' | 'in_progress'>) {
    return this.request<Game>(`/games/${encodeURIComponent(id)}/status`, {
      method: 'PUT',
      body: JSON.stringify({ status }),
    })
  }

  startUnoGame(id: string) {
    return this.request<UnoSnapshot>(`/games/${encodeURIComponent(id)}/uno/start`, { method: 'POST' })
  }

  getUnoState(id: string) {
    return this.request<UnoSnapshot>(`/games/${encodeURIComponent(id)}/uno/state`)
  }

  drawUnoCard(id: string) {
    return this.request<UnoSnapshot>(`/games/${encodeURIComponent(id)}/uno/draw`, { method: 'POST' })
  }

  playUnoCard(id: string, cardIndex: number, color?: string | null) {
    return this.request<UnoSnapshot>(`/games/${encodeURIComponent(id)}/uno/play`, {
      method: 'POST',
      body: JSON.stringify({ cardIndex, color: color ?? '' }),
    })
  }

  unoAction(id: string, action: UnoActionRequest) {
    const body: Record<string, unknown> = { type: action.type }
    if (action.type === 'play') {
      body.cardIndex = action.cardIndex
      body.color = action.color ?? ''
      body.uno = Boolean(action.uno)
    }
    if (action.type === 'catch' || action.type === 'swap') body.targetId = action.targetId
    return this.request<UnoSnapshot>(`/games/${encodeURIComponent(id)}/uno/action`, { method: 'POST', body: JSON.stringify(body) })
  }

  reconnectGame(id: string) {
    return this.request<Game>(`/games/${encodeURIComponent(id)}/reconnect`, { method: 'POST' })
  }

  spectateGame(id: string) {
    return this.request<Game>(`/games/${encodeURIComponent(id)}/spectate`, { method: 'POST' })
  }

  async leaveSpectating(id: string) {
    await this.request<void>(`/games/${encodeURIComponent(id)}/spectate`, { method: 'DELETE' })
  }

  getPublicProfile(id: string) {
    return this.request<User>(`/users/${encodeURIComponent(id)}`)
  }

  getUserAnalytics(id: string) {
    return this.request<UserAnalytics>(`/users/${encodeURIComponent(id)}/analytics`)
  }

  searchProfiles(query: string, limit = 12) {
    return this.request<ProfileSearchResult[]>(`/users/search?q=${encodeURIComponent(query)}&limit=${limit}`)
  }

  joinGame(id: string, code?: string) {
    return this.request<Game>(`/games/${encodeURIComponent(id)}/join`, {
      method: 'POST',
      body: JSON.stringify(code ? { code } : {}),
    })
  }

  joinGameByCode(code: string) {
    return this.request<Game>('/games/join', {
      method: 'POST',
      body: JSON.stringify({ code }),
    })
  }

  async leaveGame(id: string) {
    await this.request<void>(`/games/${encodeURIComponent(id)}/leave`, { method: 'POST' })
  }

  async kickParticipant(gameId: string, userId: string) {
    await this.request<void>(`/games/${encodeURIComponent(gameId)}/participants/${encodeURIComponent(userId)}`, { method: 'DELETE' })
  }

  async addWhitelist(gameId: string, username: string) {
    await this.request<void>(`/games/${encodeURIComponent(gameId)}/whitelist`, {
      method: 'POST',
      body: JSON.stringify({ username }),
    })
  }

  async removeWhitelist(gameId: string, userId: string) {
    await this.request<void>(`/games/${encodeURIComponent(gameId)}/whitelist/${encodeURIComponent(userId)}`, { method: 'DELETE' })
  }

  rotateGameCode(id: string) {
    return this.request<Game>(`/games/${encodeURIComponent(id)}/code/rotate`, { method: 'POST' })
  }

  async deleteGame(id: string) {
    await this.request<void>(`/games/${encodeURIComponent(id)}`, { method: 'DELETE' })
  }

  leaderboard(limit = 100, page = 1) {
    return this.request<LeaderboardEntry[]>(`/leaderboard?page=${page}&limit=${limit}`)
  }

  private async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const method = (init.method ?? 'GET').toUpperCase()
    if (method !== 'GET') return this.execute<T>(path, init)
    const key = `${this.token ?? 'anonymous'}:${path}`
    const pending = this.inFlight.get(key)
    if (pending) return pending as Promise<T>
    const request = this.execute<T>(path, init)
    this.inFlight.set(key, request)
    try {
      return await request
    } finally {
      this.inFlight.delete(key)
    }
  }

  private async execute<T>(path: string, init: RequestInit): Promise<T> {
    const headers = new Headers(init.headers)
    headers.set('Accept', 'application/json')
    if (init.body && !(init.body instanceof FormData)) headers.set('Content-Type', 'application/json')
    if (this.token) headers.set('Authorization', `Bearer ${this.token}`)

    const controller = new AbortController()
    const timeout = window.setTimeout(() => controller.abort(), 15000)
    let response: Response
    try {
      response = await fetch(`${API_BASE_URL}${path}`, { ...init, headers, signal: init.signal ?? controller.signal })
    } finally {
      window.clearTimeout(timeout)
    }
    if (response.ok) {
      if (response.status === 204) return undefined as T
      return response.json() as Promise<T>
    }

    let payload: ApiError = {}
    try {
      payload = (await response.json()) as ApiError
    } catch {
      // Keep a useful status-based message when the proxy returns non-JSON data.
    }
    throw new ApiClientError(
      payload.message ?? 'The request could not be completed.',
      response.status,
      payload.code,
    )
  }
}

async function optimizeAvatar(file: File): Promise<File> {
  if (!file.type.startsWith('image/') || file.type === 'image/svg+xml' || typeof createImageBitmap !== 'function') return file
  try {
    const bitmap = await createImageBitmap(file)
    const scale = Math.min(1, 512 / Math.max(bitmap.width, bitmap.height))
    const width = Math.max(1, Math.round(bitmap.width * scale))
    const height = Math.max(1, Math.round(bitmap.height * scale))
    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height
    const context = canvas.getContext('2d')
    if (!context) {
      bitmap.close()
      return file
    }
    context.drawImage(bitmap, 0, 0, width, height)
    bitmap.close()
    const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/webp', 0.82))
    if (!blob) return file
    return new File([blob], `${file.name.replace(/\.[^.]+$/, '') || 'avatar'}.webp`, { type: 'image/webp' })
  } catch {
    return file
  }
}

export const api = new ApiClient()
