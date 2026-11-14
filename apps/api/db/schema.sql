PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY NOT NULL,
    username TEXT NOT NULL COLLATE NOCASE UNIQUE,
    bio TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL COLLATE NOCASE UNIQUE,
    avatar_url TEXT,
    password_hash TEXT NOT NULL,
    elo INTEGER NOT NULL DEFAULT 1000 CHECK (elo >= 0),
    role TEXT NOT NULL DEFAULT 'player'
        CHECK (role IN ('player', 'staff', 'developer', 'admin')),
    created_at TEXT NOT NULL,
    last_login TEXT,
    deleted_at TEXT
);

CREATE TABLE IF NOT EXISTS user_stats (
    user_id TEXT PRIMARY KEY NOT NULL,
    games_played INTEGER NOT NULL DEFAULT 0 CHECK (games_played >= 0),
    games_won INTEGER NOT NULL DEFAULT 0 CHECK (games_won >= 0),
    current_streak INTEGER NOT NULL DEFAULT 0 CHECK (current_streak >= 0),
    max_streak INTEGER NOT NULL DEFAULT 0 CHECK (max_streak >= 0),
    unos_called INTEGER NOT NULL DEFAULT 0 CHECK (unos_called >= 0),
    uno_penalties INTEGER NOT NULL DEFAULT 0 CHECK (uno_penalties >= 0),
    draw_fours_played INTEGER NOT NULL DEFAULT 0 CHECK (draw_fours_played >= 0),
    cards_drawn INTEGER NOT NULL DEFAULT 0 CHECK (cards_drawn >= 0),
    CONSTRAINT fk_user_stats_user
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS games (
    id TEXT PRIMARY KEY NOT NULL,
    status TEXT NOT NULL DEFAULT 'waiting'
        CHECK (status IN ('waiting', 'in_progress', 'finished')),
    created_by TEXT,
    started_at TEXT,
    ended_at TEXT,
    turn_time_limit INTEGER NOT NULL DEFAULT 15 CHECK (turn_time_limit BETWEEN 5 AND 120),
    mode TEXT NOT NULL DEFAULT 'custom'
        CHECK (mode IN ('ranked', 'unranked', 'custom')),
    is_private INTEGER NOT NULL DEFAULT 0 CHECK (is_private IN (0, 1)),
    join_code TEXT,
    created_at TEXT NOT NULL,
    CONSTRAINT fk_games_creator
        FOREIGN KEY (created_by) REFERENCES users (id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS game_settings (
    game_id TEXT PRIMARY KEY NOT NULL,
    max_players INTEGER NOT NULL DEFAULT 4 CHECK (max_players BETWEEN 2 AND 8),
    starting_hand_size INTEGER NOT NULL DEFAULT 7 CHECK (starting_hand_size BETWEEN 3 AND 14),
    draw_penalty INTEGER NOT NULL DEFAULT 2 CHECK (draw_penalty BETWEEN 1 AND 4),
    stack_draw_cards INTEGER NOT NULL DEFAULT 0 CHECK (stack_draw_cards IN (0, 1)),
    jump_in INTEGER NOT NULL DEFAULT 0 CHECK (jump_in IN (0, 1)),
    seven_zero INTEGER NOT NULL DEFAULT 0 CHECK (seven_zero IN (0, 1)),
    draw_until_playable INTEGER NOT NULL DEFAULT 0 CHECK (draw_until_playable IN (0, 1)),
    force_play INTEGER NOT NULL DEFAULT 0 CHECK (force_play IN (0, 1)),
    challenge_draw_four INTEGER NOT NULL DEFAULT 1 CHECK (challenge_draw_four IN (0, 1)),
    allow_multiple_decks INTEGER NOT NULL DEFAULT 0 CHECK (allow_multiple_decks IN (0, 1)),
    bots_enabled INTEGER NOT NULL DEFAULT 0 CHECK (bots_enabled IN (0, 1)),
    bot_difficulty TEXT NOT NULL DEFAULT 'intermediate' CHECK (bot_difficulty IN ('easy', 'intermediate', 'expert')),
    CONSTRAINT fk_game_settings_game
        FOREIGN KEY (game_id) REFERENCES games (id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS game_participants (
    game_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    position INTEGER CHECK (position IS NULL OR position BETWEEN 1 AND 99),
    score INTEGER NOT NULL DEFAULT 0,
    -- Signed ranked-rating movement for this match. Unranked/custom games stay at zero.
    elo_delta INTEGER NOT NULL DEFAULT 0,
    last_seen_at TEXT,
    PRIMARY KEY (game_id, user_id),
    CONSTRAINT fk_game_participants_game
        FOREIGN KEY (game_id) REFERENCES games (id) ON DELETE CASCADE,
    CONSTRAINT fk_game_participants_user
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS game_spectators (
    game_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    joined_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    PRIMARY KEY (game_id, user_id),
    CONSTRAINT fk_game_spectators_game
        FOREIGN KEY (game_id) REFERENCES games (id) ON DELETE CASCADE,
    CONSTRAINT fk_game_spectators_user
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS uno_game_states (
    game_id TEXT PRIMARY KEY NOT NULL,
    version INTEGER NOT NULL DEFAULT 0 CHECK (version >= 0),
    state_json TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CONSTRAINT fk_uno_game_states_game
        FOREIGN KEY (game_id) REFERENCES games (id) ON DELETE CASCADE
);

-- One immutable event per rating change.  The game engine can attach game_id
-- when ranked matches are settled and administrative corrections remain traceable
-- with a NULL game_id and an explicit reason.
CREATE TABLE IF NOT EXISTS user_elo_history (
    id TEXT PRIMARY KEY NOT NULL,
    user_id TEXT NOT NULL,
    game_id TEXT,
    previous_elo INTEGER NOT NULL CHECK (previous_elo >= 0),
    new_elo INTEGER NOT NULL CHECK (new_elo >= 0),
    delta INTEGER NOT NULL,
    reason TEXT NOT NULL DEFAULT 'game_result',
    created_at TEXT NOT NULL,
    CONSTRAINT fk_user_elo_history_user
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_user_elo_history_game
        FOREIGN KEY (game_id) REFERENCES games (id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_user_elo_history_user_time
    ON user_elo_history (user_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_user_elo_history_game
    ON user_elo_history (game_id);

CREATE TABLE IF NOT EXISTS game_whitelist (
    game_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (game_id, user_id),
    CONSTRAINT fk_game_whitelist_game
        FOREIGN KEY (game_id) REFERENCES games (id) ON DELETE CASCADE,
    CONSTRAINT fk_game_whitelist_user
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS auth_sessions (
    id TEXT PRIMARY KEY NOT NULL,
    user_id TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    revoked_at TEXT,
    user_agent TEXT,
    ip_address TEXT,
    CONSTRAINT fk_auth_sessions_user
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id TEXT PRIMARY KEY NOT NULL,
    actor_user_id TEXT,
    actor_username TEXT,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id TEXT,
    details TEXT NOT NULL DEFAULT '',
    severity TEXT NOT NULL DEFAULT 'info'
        CHECK (severity IN ('info', 'warning', 'critical')),
    ip_address TEXT,
    user_agent TEXT,
    created_at TEXT NOT NULL,
    CONSTRAINT fk_audit_logs_actor
        FOREIGN KEY (actor_user_id) REFERENCES users (id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_users_elo ON users (elo DESC);
CREATE INDEX IF NOT EXISTS idx_users_username_prefix ON users (username COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_users_last_login ON users (last_login);
CREATE INDEX IF NOT EXISTS idx_games_status_created_at ON games (status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_games_created_by ON games (created_by);
CREATE INDEX IF NOT EXISTS idx_games_creator_status ON games (created_by, status);
CREATE INDEX IF NOT EXISTS idx_game_settings_max_players ON game_settings (max_players);
CREATE INDEX IF NOT EXISTS idx_game_participants_user ON game_participants (user_id);
CREATE INDEX IF NOT EXISTS idx_game_spectators_game ON game_spectators (game_id, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_game_spectators_user ON game_spectators (user_id);
CREATE INDEX IF NOT EXISTS idx_uno_game_states_updated_at ON uno_game_states (updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_game_whitelist_user ON game_whitelist (user_id);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_user ON auth_sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_expiry ON auth_sessions (expires_at);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs (action, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_severity ON audit_logs (severity, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs (actor_user_id, created_at DESC);
