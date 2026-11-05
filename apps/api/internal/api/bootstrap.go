package api

import (
	"database/sql"
	"fmt"
	"strings"

	"card-arena/api/internal/config"
	"card-arena/api/internal/db"
	"github.com/google/uuid"
)

// Bootstrap restores the explicit development/admin access rules without
// silently inventing production accounts.
func Bootstrap(store *db.Store, cfg config.Config) error {
	for _, username := range cfg.AdminUsernames {
		if strings.TrimSpace(username) == "" {
			continue
		}
		if _, err := store.DB.Exec(`UPDATE users SET role='admin', deleted_at=NULL WHERE username=? COLLATE NOCASE`, strings.TrimSpace(username)); err != nil {
			return fmt.Errorf("promote configured admin %q: %w", username, err)
		}
	}
	for _, email := range cfg.AdminEmails {
		if strings.TrimSpace(email) == "" {
			continue
		}
		if _, err := store.DB.Exec(`UPDATE users SET role='admin', deleted_at=NULL WHERE email=? COLLATE NOCASE`, strings.ToLower(strings.TrimSpace(email))); err != nil {
			return fmt.Errorf("promote configured admin email: %w", err)
		}
	}
	if !cfg.DebugAuthEnabled || strings.TrimSpace(cfg.DebugAuthUsername) == "" {
		return nil
	}
	username := strings.TrimSpace(cfg.DebugAuthUsername)
	var id string
	err := store.DB.QueryRow(`SELECT id FROM users WHERE username=? COLLATE NOCASE`, username).Scan(&id)
	if err == nil {
		_, err = store.DB.Exec(`UPDATE users SET role='admin', deleted_at=NULL WHERE id=?`, id)
		return err
	}
	if err != sql.ErrNoRows {
		return err
	}
	id = uuid.NewString()
	email := "debug+" + strings.ToLower(strings.ReplaceAll(id, "-", ""))[:12] + "@learner.42.tech"
	created := nowString()
	_, err = store.DB.Exec(`INSERT INTO users(id,username,email,password_hash,elo,role,created_at) VALUES(?,?,?,?,1000,'admin',?)`, id, username, email, "$debug-only$", created)
	if err != nil {
		return fmt.Errorf("create debug account: %w", err)
	}
	if _, err = store.DB.Exec(`INSERT INTO user_stats(user_id) VALUES(?)`, id); err != nil {
		return err
	}
	_, err = store.DB.Exec(`INSERT INTO user_elo_history(id,user_id,previous_elo,new_elo,delta,reason,created_at) VALUES(?,?,?,?,?,?,?)`, uuid.NewString(), id, 1000, 1000, 0, "account_created", created)
	return err
}
