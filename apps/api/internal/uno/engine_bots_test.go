package uno

import (
	"os"
	"testing"
	"time"

	"card-arena/api/internal/db"
)

func openStore(t *testing.T) *db.Store {
	t.Helper()
	schema := ""
	for _, candidate := range []string{"../../db/schema.sql", "../../schema.sql"} {
		if _, err := os.Stat(candidate); err == nil {
			schema = candidate
			break
		}
	}
	if schema == "" {
		t.Fatal("schema.sql was not found next to the api module")
	}
	store, err := db.Open(":memory:", schema)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestEngineSchedulesBotTurns(t *testing.T) {
	store := openStore(t)
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	mustExec := func(query string, args ...any) {
		if _, err := store.DB.Exec(query, args...); err != nil {
			t.Fatalf("%s: %v", query, err)
		}
	}
	mustExec(`INSERT INTO users(id,username,email,password_hash,created_at) VALUES('host','host','host@example.test','x',?)`, stamp)
	mustExec(`INSERT INTO games(id,status,created_by,turn_time_limit,mode,is_private,created_at) VALUES('g1','waiting','host',30,'custom',0,?)`, stamp)
	mustExec(`INSERT INTO game_settings(game_id,max_players,bots_enabled,bot_difficulty) VALUES('g1',3,1,'expert')`)
	mustExec(`INSERT INTO game_participants(game_id,user_id,last_seen_at) VALUES('g1','host',?)`, stamp)

	engine := NewEngine(store.DB)
	engine.botSpeed = 0
	t.Cleanup(engine.Stop)
	updates := make(chan Update, 64)
	engine.OnChange(func(update Update) { updates <- update })

	snapshot, err := engine.Start("g1", "host")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if len(snapshot.Players) != 3 || !snapshot.Players[1].Bot || !snapshot.Players[2].Bot {
		t.Fatalf("expected two bots at the table, got %+v", snapshot.Players)
	}
	deadline := time.After(5 * time.Second)
	for {
		current, err := engine.State("g1", "host")
		if err != nil {
			t.Fatalf("state: %v", err)
		}
		if current.Status == "finished" {
			t.Fatalf("game finished before the host could act")
		}
		if current.TurnPlayerID == "host" && current.Phase == PhaseTurn {
			after, err := engine.Draw("g1", "host")
			if err != nil {
				t.Fatalf("host draw: %v", err)
			}
			if after.Phase == PhaseDrawn {
				if _, err = engine.Act("g1", "host", Action{Type: ActionPass}); err != nil {
					t.Fatalf("host pass: %v", err)
				}
			}
			break
		}
		if current.TurnPlayerID == "host" {
			action := Action{Type: ActionPass}
			if current.Phase == PhaseChallenge {
				action.Type = ActionAccept
			}
			if _, err = engine.Act("g1", "host", action); err != nil {
				t.Fatalf("host %s: %v", action.Type, err)
			}
			break
		}
		select {
		case <-updates:
		case <-deadline:
			t.Fatalf("bots never handed the turn to the host")
		}
	}
	deadline = time.After(5 * time.Second)
	for {
		select {
		case update := <-updates:
			current := update.Snapshot("host")
			if current.LastEvent.PlayerID != "" && current.LastEvent.PlayerID != "host" && current.LastEvent.Type != "start" {
				return
			}
		case <-deadline:
			t.Fatalf("no bot move was published after the host's turn")
		}
	}
}
