package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store owns the SQLite connection. SQLite is intentionally kept to a single
// writer connection; handlers remain concurrent while writes stay serialized.
type Store struct{ DB *sql.DB }

func Open(path, schema string) (*Store, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	database, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	store := &Store{DB: database}
	if _, err = database.Exec(`PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL; PRAGMA busy_timeout = 5000;`); err != nil {
		database.Close()
		return nil, fmt.Errorf("configure sqlite: %w", err)
	}
	if err = store.applySchema(schema); err != nil {
		database.Close()
		return nil, err
	}
	if err = store.applyMigrations(); err != nil {
		database.Close()
		return nil, err
	}
	return store, nil
}

func (store *Store) Close() error { return store.DB.Close() }

// ResetTransientGames clears development game states that cannot survive an
// API restart because the temporary game service has no durable game engine.
// Finished games remain in the history, while waiting lobbies are removed
// because they cannot be resumed safely after a restart.
func (store *Store) ResetTransientGames() error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := store.DB.Exec(`UPDATE games SET status='finished', ended_at=? WHERE status='in_progress'`, now); err != nil {
		return fmt.Errorf("finish transient games: %w", err)
	}
	if _, err := store.DB.Exec(`DELETE FROM games WHERE status='waiting'`); err != nil {
		return fmt.Errorf("remove waiting lobbies: %w", err)
	}
	return nil
}

func (store *Store) applySchema(schemaPath string) error {
	contents, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("read schema %s: %w", schemaPath, err)
	}
	lines := make([]string, 0)
	for _, line := range strings.Split(string(contents), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		lines = append(lines, line)
	}
	for _, statement := range strings.Split(strings.Join(lines, "\n"), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if _, err = store.DB.Exec(statement); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
	}
	return nil
}

// applyMigrations keeps local development databases usable when the canonical
// schema renames a column. Fresh databases are already created with the new
// name, while existing databases are upgraded without dropping match data.
func (store *Store) applyMigrations() error {
	hasLegacyColumn, err := store.hasColumn("game_participants", "xp_earned")
	if err != nil {
		return fmt.Errorf("inspect game_participants columns: %w", err)
	}
	hasEloColumn, err := store.hasColumn("game_participants", "elo_delta")
	if err != nil {
		return fmt.Errorf("inspect game_participants columns: %w", err)
	}
	if hasLegacyColumn && !hasEloColumn {
		if _, err := store.DB.Exec(`ALTER TABLE game_participants RENAME COLUMN xp_earned TO elo_delta`); err != nil {
			return fmt.Errorf("rename game_participants.xp_earned: %w", err)
		}
	}
	if err := store.ensureGameBotSetting(); err != nil {
		return fmt.Errorf("configure game bot setting: %w", err)
	}
	if err := store.ensureGamePresence(); err != nil {
		return fmt.Errorf("configure game presence: %w", err)
	}
	if _, err := store.DB.Exec(`UPDATE game_participants SET last_seen_at=(SELECT created_at FROM games WHERE games.id=game_participants.game_id) WHERE last_seen_at IS NULL`); err != nil {
		return fmt.Errorf("initialize game presence: %w", err)
	}
	if err := store.ensureUserSearch(); err != nil {
		return fmt.Errorf("configure user search: %w", err)
	}
	return nil
}

func (store *Store) ensureGamePresence() error {
	hasLastSeen, err := store.hasColumn("game_participants", "last_seen_at")
	if err != nil {
		return err
	}
	if !hasLastSeen {
		if _, err := store.DB.Exec(`ALTER TABLE game_participants ADD COLUMN last_seen_at TEXT`); err != nil {
			return err
		}
	}
	_, err = store.DB.Exec(`
CREATE TABLE IF NOT EXISTS game_spectators (
    game_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    joined_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    PRIMARY KEY (game_id, user_id),
    CONSTRAINT fk_game_spectators_game FOREIGN KEY (game_id) REFERENCES games (id) ON DELETE CASCADE,
    CONSTRAINT fk_game_spectators_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_game_spectators_game ON game_spectators (game_id, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_game_spectators_user ON game_spectators (user_id);
`)
	return err
}

func (store *Store) ensureGameBotSetting() error {
	hasBotsEnabled, err := store.hasColumn("game_settings", "bots_enabled")
	if err != nil {
		return err
	}
	if hasBotsEnabled {
		return nil
	}
	_, err = store.DB.Exec(`ALTER TABLE game_settings ADD COLUMN bots_enabled INTEGER NOT NULL DEFAULT 0 CHECK (bots_enabled IN (0, 1))`)
	return err
}

// ensureUserSearch adds an FTS5 index for substring-capable username search.
// Older SQLite builds may not include FTS5; in that case the normal indexed
// username prefix query remains available and startup still succeeds.
func (store *Store) ensureUserSearch() error {
	if _, err := store.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_users_username_prefix ON users(username COLLATE NOCASE)`); err != nil {
		return err
	}
	_, err := store.DB.Exec(`
CREATE VIRTUAL TABLE IF NOT EXISTS users_fts USING fts5(
	username,
	bio,
	content='users',
	content_rowid='rowid',
	tokenize='trigram'
);
CREATE TRIGGER IF NOT EXISTS users_fts_insert AFTER INSERT ON users BEGIN
	INSERT INTO users_fts(rowid, username, bio) VALUES (new.rowid, new.username, new.bio);
END;
CREATE TRIGGER IF NOT EXISTS users_fts_delete AFTER DELETE ON users BEGIN
	INSERT INTO users_fts(users_fts, rowid, username, bio) VALUES ('delete', old.rowid, old.username, old.bio);
END;
CREATE TRIGGER IF NOT EXISTS users_fts_update AFTER UPDATE OF username, bio ON users BEGIN
	INSERT INTO users_fts(users_fts, rowid, username, bio) VALUES ('delete', old.rowid, old.username, old.bio);
	INSERT INTO users_fts(rowid, username, bio) VALUES (new.rowid, new.username, new.bio);
END;
`)
	if err != nil {
		// SQLite without FTS5 is still usable through the prefix index above.
		return nil
	}
	var indexed, users int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM users_fts`).Scan(&indexed); err != nil {
		return err
	}
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
		return err
	}
	if indexed != users {
		if _, err := store.DB.Exec(`INSERT INTO users_fts(users_fts) VALUES ('rebuild')`); err != nil {
			return fmt.Errorf("rebuild user search index: %w", err)
		}
	}
	return nil
}

func (store *Store) hasColumn(table, wanted string) (bool, error) {
	rows, err := store.DB.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid          int
			name         string
			columnType   string
			notNull      int
			defaultValue sql.NullString
			primaryKey   int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		if name == wanted {
			return true, nil
		}
	}
	return false, rows.Err()
}
