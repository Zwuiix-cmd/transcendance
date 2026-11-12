export type Stats = {
  gamesPlayed: number
  gamesWon: number
  currentStreak: number
  maxStreak: number
  unosCalled: number
  unoPenalties: number
  drawFoursPlayed: number
  cardsDrawn: number
}

export type UserRole = 'player' | 'staff' | 'developer' | 'admin'

export type User = {
  id: string
  username: string
  bio: string
  email: string | null
  avatarUrl: string | null
  elo: number
  role: UserRole
  createdAt: string
  lastLogin: string | null
  stats: Stats
}

export type AuthResponse = {
  accessToken: string
  expiresAt: string
  user: User
}

export type Game = {
  id: string
  status: 'waiting' | 'in_progress' | 'finished'
  createdBy: string | null
  creatorUsername: string | null
  createdAt: string
  startedAt: string | null
  endedAt: string | null
  turnTimeLimit: number
  mode: 'ranked' | 'unranked' | 'custom'
  participantCount: number
  privateGame: boolean
  joinCode: string | null
  host: boolean
  joined: boolean
  spectating: boolean
  viewerRole: 'player' | 'spectator' | 'observer'
  spectatorCount: number
  whitelistCount: number
  settings: GameSettings
}

export type GameSettings = {
  maxPlayers: number
  startingHandSize: number
  drawPenalty: number
  stackDrawCards: boolean
  jumpIn: boolean
  sevenZero: boolean
  drawUntilPlayable: boolean
  forcePlay: boolean
  challengeDrawFour: boolean
  allowMultipleDecks: boolean
  botsEnabled: boolean
}

export type QueueMode = 'ranked' | 'unranked'

export type MatchmakingPlayer = {
  userId: string
  mode: QueueMode
  elo: number
  joinedAt: string
}

export type MatchmakingStatus = {
  status: 'waiting' | 'matched' | 'not_waiting'
  mode: QueueMode | null
  queued: boolean
  matched: boolean
  queueSize: number
  position: number
  waitedSeconds: number
  estimatedWaitSeconds: number
  ticketId: string | null
  createdAt: string | null
  players: MatchmakingPlayer[]
  searchEloMin: number | null
  searchEloMax: number | null
  searchEloGap: number | null
}

export type LobbyParticipant = {
  userId: string
  username: string
  host: boolean
}

export type WhitelistEntry = {
  userId: string
  username: string
}

export type GameDetail = {
  game: Game
  participants: LobbyParticipant[]
  spectators: LobbySpectator[]
  whitelist: WhitelistEntry[]
}

export type UnoPlayerState = {
  id: string
  name: string
  bot: boolean
  finished: boolean
  cardCount: number
  hand?: string[]
}

export type PlayerResult = {
  rank: number
  playerId: string
  name: string
  bot: boolean
  winner: boolean
  cardsRemaining: number
  handPoints: number
  pointsEarned: number
}

export type UnoSnapshot = {
  gameId: string
  status: 'active' | 'finished'
  version: number
  turnPlayerId: string
  turnNumber: number
  currentColor: string
  topCard: string
  drawCount: number
  players: UnoPlayerState[]

  winnerId?: string
  winnerScore: number
  results?: PlayerResult[]

  lastAction: string
}

export type LobbySpectator = {
  userId: string
  username: string
}

export type LeaderboardEntry = {
  rank: number
  userId: string
  username: string
  avatarUrl: string | null
  elo: number
  gamesPlayed: number
  gamesWon: number
  winRate: number
}

export type ProfileSearchResult = {
  id: string
  username: string
  bio: string
  avatarUrl: string | null
  elo: number
  gamesPlayed: number
  gamesWon: number
}

export type UserAnalytics = {
  summary: UserAnalyticsSummary
  eloHistory: EloHistoryPoint[]
  activity: UserActivityPoint[]
  recentGames: UserGamePoint[]
  activityMetrics: ActivityMetrics
  eloChart: EloChartGeometry | null
  activityChart: PlayerActivityChartGeometry | null
}

export type ActivityMetrics = {
  totalGames: number
  totalWins: number
  totalLosses: number
  totalScore: number
  totalEloDelta: number
  activeDays: number
  bestDay: number
  bestDayGames: number
  winRate: number
  averageGamesPerActiveDay: number
}

export type EloChartGeometry = {
  min: number
  max: number
  points: Array<[number, number]>
}

export type PlayerActivityChartGeometry = {
  max: number
  points: Array<[number, number, number]>
}

export type UserAnalyticsSummary = {
  currentElo: number
  averageElo: number
  peakElo: number
  lowestElo: number
  netEloDelta: number
  volatility: number
  averageEloDelta: number
  positiveEloDeltas: number
  negativeEloDeltas: number
  eloEvents: number
  rank: number
  totalPlayers: number
  percentile: number
  gamesPlayed: number
  gamesWon: number
  winRate: number
  currentStreak: number
  maxStreak: number
  unosCalled: number
  unoPenalties: number
  drawFoursPlayed: number
  cardsDrawn: number
  totalScore: number
  totalEloDelta: number
}

export type EloHistoryPoint = {
  id: string
  gameId: string | null
  previousElo: number
  newElo: number
  delta: number
  reason: string
  createdAt: string
}

export type UserActivityPoint = {
  date: string
  games: number
  wins: number
  losses: number
  score: number
  eloDelta: number
}

export type UserGamePoint = {
  gameId: string
  mode: 'ranked' | 'unranked' | 'custom'
  status: 'waiting' | 'in_progress' | 'finished'
  position: number | null
  score: number
  eloDelta: number
  playedAt: string
}

export type ApiError = {
  code?: string
  message?: string
}

export type GameDeletedEvent = {
  gameId: string
}

export type UnoActionRequest =
  | { type: 'draw' }
  | { type: 'play'; cardIndex: number; color: string | null }
