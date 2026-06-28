package twentyq

import (
	"context"
	"errors"
	"fmt"

	"github.com/tiennm99/miti99bot/internal/storage"
)

// Turn is one Q&A entry stored in the game's history.
type Turn struct {
	Text    string `json:"text" bson:"text"`
	IsGuess bool   `json:"isGuess" bson:"isGuess"`
	Answer  string `json:"answer" bson:"answer"` // "yes" | "no"
	Hint    string `json:"hint" bson:"hint"`
	TS      int64  `json:"ts" bson:"ts"`
}

type GameState struct {
	Category    string `json:"category" bson:"category"`
	Target      string `json:"target" bson:"target"`
	InitialHint string `json:"initialHint" bson:"initialHint"`
	StartedAt   *int64 `json:"startedAt" bson:"startedAt"`
	Solved      bool   `json:"solved" bson:"solved"`
	Turns       []Turn `json:"turns" bson:"turns"`
}

type Stats struct {
	Played        int    `json:"played" bson:"played"`
	Solved        int    `json:"solved" bson:"solved"`
	TotalTurns    int    `json:"totalTurns" bson:"totalTurns"`
	BestTurnCount *int   `json:"bestTurnCount" bson:"bestTurnCount"`
	LastResultAt  *int64 `json:"lastResultAt" bson:"lastResultAt"`
}

// GameStore is the twentyq module's typed game store.
type GameStore = storage.DocStore[GameState]

// StatsStore is the twentyq module's typed stats store.
type StatsStore = storage.DocStore[Stats]

func gameKey(subject string) string  { return "game:" + subject }
func statsKey(subject string) string { return "stats:" + subject }

func loadGame(ctx context.Context, games GameStore, subject string) (*GameState, error) {
	g, _, err := games.Get(ctx, gameKey(subject))
	switch {
	case err == nil:
		return &g, nil
	case errors.Is(err, storage.ErrNotFound):
		return nil, nil
	default:
		return nil, fmt.Errorf("twentyq loadGame: %w", err)
	}
}

func saveGame(ctx context.Context, games GameStore, subject string, g *GameState) error {
	if err := games.Put(ctx, gameKey(subject), *g); err != nil {
		return fmt.Errorf("twentyq saveGame: %w", err)
	}
	return nil
}

func clearGame(ctx context.Context, games GameStore, subject string) error {
	if err := games.Delete(ctx, gameKey(subject)); err != nil && !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("twentyq clearGame: %w", err)
	}
	return nil
}

func loadStats(ctx context.Context, st StatsStore, subject string) (*Stats, error) {
	s, _, err := st.Get(ctx, statsKey(subject))
	switch {
	case err == nil:
		return &s, nil
	case errors.Is(err, storage.ErrNotFound):
		return &Stats{}, nil
	default:
		return nil, fmt.Errorf("twentyq loadStats: %w", err)
	}
}

func recordResult(ctx context.Context, st StatsStore, subject string, solved bool, turnCount int, nowMillis int64) (*Stats, error) {
	s, err := loadStats(ctx, st, subject)
	if err != nil {
		return nil, err
	}
	s.Played++
	s.TotalTurns += turnCount
	if solved {
		s.Solved++
		if s.BestTurnCount == nil || turnCount < *s.BestTurnCount {
			tc := turnCount
			s.BestTurnCount = &tc
		}
	}
	now := nowMillis
	s.LastResultAt = &now
	if err := st.Put(ctx, statsKey(subject), *s); err != nil {
		return nil, fmt.Errorf("twentyq recordResult: %w", err)
	}
	return s, nil
}
