package model

// Stats is kept separate from a profile to mirror the database's one-to-one
// user_stats relation.
type Stats struct {
	GamesPlayed     int `json:"gamesPlayed"`
	GamesWon        int `json:"gamesWon"`
	CurrentStreak   int `json:"currentStreak"`
	MaxStreak       int `json:"maxStreak"`
	UnosCalled      int `json:"unosCalled"`
	UnoPenalties    int `json:"unoPenalties"`
	DrawFoursPlayed int `json:"drawFoursPlayed"`
	CardsDrawn      int `json:"cardsDrawn"`
}

type User struct {
	ID        string  `json:"id"`
	Username  string  `json:"username"`
	Bio       string  `json:"bio"`
	Email     *string `json:"email"`
	AvatarURL *string `json:"avatarUrl"`
	Elo       int     `json:"elo"`
	Role      string  `json:"role"`
	CreatedAt string  `json:"createdAt"`
	LastLogin *string `json:"lastLogin"`
	Stats     Stats   `json:"stats"`
}

type AuthResponse struct {
	AccessToken string `json:"accessToken"`
	ExpiresAt   string `json:"expiresAt"`
	User        User   `json:"user"`
}

type GameSettings struct {
	MaxPlayers         int  `json:"maxPlayers"`
	StartingHandSize   int  `json:"startingHandSize"`
	DrawPenalty        int  `json:"drawPenalty"`
	StackDrawCards     bool `json:"stackDrawCards"`
	JumpIn             bool `json:"jumpIn"`
	SevenZero          bool `json:"sevenZero"`
	DrawUntilPlayable  bool `json:"drawUntilPlayable"`
	ForcePlay          bool `json:"forcePlay"`
	ChallengeDrawFour  bool `json:"challengeDrawFour"`
	AllowMultipleDecks bool `json:"allowMultipleDecks"`
	BotsEnabled        bool `json:"botsEnabled"`
}

func DefaultGameSettings() GameSettings {
	return GameSettings{MaxPlayers: 4, StartingHandSize: 7, DrawPenalty: 2, ChallengeDrawFour: true}
}

type Game struct {
	ID               string       `json:"id"`
	Status           string       `json:"status"`
	CreatedBy        *string      `json:"createdBy"`
	CreatorUsername  *string      `json:"creatorUsername"`
	CreatedAt        string       `json:"createdAt"`
	StartedAt        *string      `json:"startedAt"`
	EndedAt          *string      `json:"endedAt"`
	TurnTimeLimit    int          `json:"turnTimeLimit"`
	Mode             string       `json:"mode"`
	ParticipantCount int          `json:"participantCount"`
	PrivateGame      bool         `json:"privateGame"`
	JoinCode         *string      `json:"joinCode"`
	Host             bool         `json:"host"`
	Joined           bool         `json:"joined"`
	Spectating       bool         `json:"spectating"`
	ViewerRole       string       `json:"viewerRole"`
	SpectatorCount   int          `json:"spectatorCount"`
	WhitelistCount   int          `json:"whitelistCount"`
	Settings         GameSettings `json:"settings"`
}

type GameDetail struct {
	Game         Game               `json:"game"`
	Participants []LobbyParticipant `json:"participants"`
	Spectators   []LobbySpectator   `json:"spectators"`
	Whitelist    []WhitelistEntry   `json:"whitelist"`
}

type GameResultParticipant struct {
	UserID   string `json:"userId"`
	Position int    `json:"position"`
	Score    int    `json:"score"`
	EloDelta int    `json:"eloDelta"`
}

type GameResultResponse struct {
	GameID         string                  `json:"gameId"`
	Mode           string                  `json:"mode"`
	Status         string                  `json:"status"`
	EndedAt        string                  `json:"endedAt"`
	AlreadySettled bool                    `json:"alreadySettled"`
	Participants   []GameResultParticipant `json:"participants"`
}

type LobbyParticipant struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Host     bool   `json:"host"`
}

type LobbySpectator struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
}

type WhitelistEntry struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
}

type LeaderboardEntry struct {
	Rank        int     `json:"rank"`
	UserID      string  `json:"userId"`
	Username    string  `json:"username"`
	AvatarURL   *string `json:"avatarUrl"`
	Elo         int     `json:"elo"`
	GamesPlayed int     `json:"gamesPlayed"`
	GamesWon    int     `json:"gamesWon"`
	WinRate     float64 `json:"winRate"`
}

type ProfileSearchResult struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	Bio         string  `json:"bio"`
	AvatarURL   *string `json:"avatarUrl"`
	Elo         int     `json:"elo"`
	GamesPlayed int     `json:"gamesPlayed"`
	GamesWon    int     `json:"gamesWon"`
}

type EloHistoryPoint struct {
	ID          string  `json:"id"`
	GameID      *string `json:"gameId"`
	PreviousElo int     `json:"previousElo"`
	NewElo      int     `json:"newElo"`
	Delta       int     `json:"delta"`
	Reason      string  `json:"reason"`
	CreatedAt   string  `json:"createdAt"`
}

type UserActivityPoint struct {
	Date     string `json:"date"`
	Games    int    `json:"games"`
	Wins     int    `json:"wins"`
	Losses   int    `json:"losses"`
	Score    int    `json:"score"`
	EloDelta int    `json:"eloDelta"`
}

type UserGamePoint struct {
	GameID   string `json:"gameId"`
	Mode     string `json:"mode"`
	Status   string `json:"status"`
	Position *int   `json:"position"`
	Score    int    `json:"score"`
	EloDelta int    `json:"eloDelta"`
	PlayedAt string `json:"playedAt"`
}

type UserAnalyticsSummary struct {
	CurrentElo        int     `json:"currentElo"`
	AverageElo        float64 `json:"averageElo"`
	PeakElo           int     `json:"peakElo"`
	LowestElo         int     `json:"lowestElo"`
	NetEloDelta       int     `json:"netEloDelta"`
	Volatility        float64 `json:"volatility"`
	AverageEloDelta   float64 `json:"averageEloDelta"`
	PositiveEloDeltas int     `json:"positiveEloDeltas"`
	NegativeEloDeltas int     `json:"negativeEloDeltas"`
	EloEvents         int     `json:"eloEvents"`
	Rank              int     `json:"rank"`
	TotalPlayers      int     `json:"totalPlayers"`
	Percentile        float64 `json:"percentile"`
	GamesPlayed       int     `json:"gamesPlayed"`
	GamesWon          int     `json:"gamesWon"`
	WinRate           float64 `json:"winRate"`
	CurrentStreak     int     `json:"currentStreak"`
	MaxStreak         int     `json:"maxStreak"`
	UnosCalled        int     `json:"unosCalled"`
	UnoPenalties      int     `json:"unoPenalties"`
	DrawFoursPlayed   int     `json:"drawFoursPlayed"`
	CardsDrawn        int     `json:"cardsDrawn"`
	TotalScore        int     `json:"totalScore"`
	TotalEloDelta     int     `json:"totalEloDelta"`
}

type UserAnalytics struct {
	Summary         UserAnalyticsSummary         `json:"summary"`
	EloHistory      []EloHistoryPoint            `json:"eloHistory"`
	Activity        []UserActivityPoint          `json:"activity"`
	RecentGames     []UserGamePoint              `json:"recentGames"`
	ActivityMetrics ActivityMetrics              `json:"activityMetrics"`
	EloChart        *EloChartGeometry            `json:"eloChart"`
	ActivityChart   *PlayerActivityChartGeometry `json:"activityChart"`
}

type ActivityMetrics struct {
	TotalGames               int     `json:"totalGames"`
	TotalWins                int     `json:"totalWins"`
	TotalLosses              int     `json:"totalLosses"`
	TotalScore               int     `json:"totalScore"`
	TotalEloDelta            int     `json:"totalEloDelta"`
	ActiveDays               int     `json:"activeDays"`
	BestDay                  int     `json:"bestDay"`
	BestDayGames             int     `json:"bestDayGames"`
	WinRate                  float64 `json:"winRate"`
	AverageGamesPerActiveDay float64 `json:"averageGamesPerActiveDay"`
}

type EloChartGeometry struct {
	Min    float64      `json:"min"`
	Max    float64      `json:"max"`
	Points [][2]float64 `json:"points"`
}

type PlayerActivityChartGeometry struct {
	Max    float64      `json:"max"`
	Points [][3]float64 `json:"points"`
}

type MatchmakingPlayer struct {
	UserID   string `json:"userId"`
	Mode     string `json:"mode"`
	Elo      int    `json:"elo"`
	JoinedAt string `json:"joinedAt"`
}

type MatchmakingStatus struct {
	Status               string              `json:"status"`
	Mode                 *string             `json:"mode"`
	Queued               bool                `json:"queued"`
	Matched              bool                `json:"matched"`
	QueueSize            int                 `json:"queueSize"`
	Position             int                 `json:"position"`
	WaitedSeconds        int64               `json:"waitedSeconds"`
	EstimatedWaitSeconds int                 `json:"estimatedWaitSeconds"`
	TicketID             *string             `json:"ticketId"`
	CreatedAt            *string             `json:"createdAt"`
	Players              []MatchmakingPlayer `json:"players"`
	SearchEloMin         *int                `json:"searchEloMin"`
	SearchEloMax         *int                `json:"searchEloMax"`
	SearchEloGap         *int                `json:"searchEloGap"`
}

type AdminOverview struct {
	TotalUsers    int `json:"totalUsers"`
	ActiveUsers   int `json:"activeUsers"`
	AdminUsers    int `json:"adminUsers"`
	AverageElo    int `json:"averageElo"`
	HighestElo    int `json:"highestElo"`
	TotalGames    int `json:"totalGames"`
	WaitingGames  int `json:"waitingGames"`
	ActiveGames   int `json:"activeGames"`
	FinishedGames int `json:"finishedGames"`
	RecordedGames int `json:"recordedGames"`
}

type AdminActivityPoint struct {
	Date           string `json:"date"`
	TotalRequests  int    `json:"totalRequests"`
	Registrations  int    `json:"registrations"`
	Logins         int    `json:"logins"`
	GameActions    int    `json:"gameActions"`
	AdminActions   int    `json:"adminActions"`
	SecurityEvents int    `json:"securityEvents"`
}

type AdminDistribution struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

type AuditLog struct {
	ID            string  `json:"id"`
	ActorUsername *string `json:"actorUsername"`
	Action        string  `json:"action"`
	TargetType    string  `json:"targetType"`
	TargetID      *string `json:"targetId"`
	Details       string  `json:"details"`
	StatusCode    *int    `json:"statusCode"`
	Severity      string  `json:"severity"`
	CreatedAt     string  `json:"createdAt"`
	IPAddress     *string `json:"ipAddress"`
	UserAgent     *string `json:"userAgent"`
}

type AdminDashboard struct {
	Overview         AdminOverview        `json:"overview"`
	LogsToday        int                  `json:"logsToday"`
	Activity         []AdminActivityPoint `json:"activity"`
	EloDistribution  []AdminDistribution  `json:"eloDistribution"`
	ModeDistribution []AdminDistribution  `json:"modeDistribution"`
	RecentLogs       []AuditLog           `json:"recentLogs"`
}

type AdminUser struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	Email       string  `json:"email"`
	AvatarURL   *string `json:"avatarUrl"`
	Elo         int     `json:"elo"`
	Role        string  `json:"role"`
	CreatedAt   string  `json:"createdAt"`
	LastLogin   *string `json:"lastLogin"`
	GamesPlayed int     `json:"gamesPlayed"`
	GamesWon    int     `json:"gamesWon"`
}

type AdminActorStats struct {
	Username           string `json:"username"`
	TotalEvents        int    `json:"totalEvents"`
	Requests           int    `json:"requests"`
	SuccessfulRequests int    `json:"successfulRequests"`
	FailedRequests     int    `json:"failedRequests"`
	SecurityEvents     int    `json:"securityEvents"`
	LastSeen           string `json:"lastSeen"`
}

type AdminLogStats struct {
	TotalEvents        int               `json:"totalEvents"`
	TotalRequests      int               `json:"totalRequests"`
	SuccessfulRequests int               `json:"successfulRequests"`
	ClientErrors       int               `json:"clientErrors"`
	ServerErrors       int               `json:"serverErrors"`
	SecurityEvents     int               `json:"securityEvents"`
	AdminActions       int               `json:"adminActions"`
	ActiveActors       int               `json:"activeActors"`
	Actors             []AdminActorStats `json:"actors"`
}
