package wc

import "github.com/tiennm99/miti99bot/internal/storage"

// Team is the football-data.org team object shape used by World Cup matches.
type Team struct {
	ID        int    `json:"id,omitempty" bson:"id,omitempty"`
	Name      string `json:"name,omitempty" bson:"name,omitempty"`
	ShortName string `json:"shortName,omitempty" bson:"shortName,omitempty"`
	TLA       string `json:"tla,omitempty" bson:"tla,omitempty"`
	Crest     string `json:"crest,omitempty" bson:"crest,omitempty"`
}

// ScoreValue holds the home/away goals for one score phase. Pointers preserve
// "not available yet" separately from a real 0-0 score.
type ScoreValue struct {
	Home *int `json:"home,omitempty" bson:"home,omitempty"`
	Away *int `json:"away,omitempty" bson:"away,omitempty"`
}

// Score is the subset of the football-data.org score payload that the bot
// needs for live/finished display.
type Score struct {
	Winner   string     `json:"winner,omitempty" bson:"winner,omitempty"`
	Duration string     `json:"duration,omitempty" bson:"duration,omitempty"`
	FullTime ScoreValue `json:"fullTime,omitempty" bson:"fullTime,omitempty"`
	HalfTime ScoreValue `json:"halfTime,omitempty" bson:"halfTime,omitempty"`
}

// Match is the normalized provider match persisted in the module cache.
type Match struct {
	ID          int    `json:"id,omitempty" bson:"id,omitempty"`
	UTCDate     string `json:"utcDate,omitempty" bson:"utcDate,omitempty"`
	Status      string `json:"status,omitempty" bson:"status,omitempty"`
	Matchday    int    `json:"matchday,omitempty" bson:"matchday,omitempty"`
	Stage       string `json:"stage,omitempty" bson:"stage,omitempty"`
	Group       string `json:"group,omitempty" bson:"group,omitempty"`
	LastUpdated string `json:"lastUpdated,omitempty" bson:"lastUpdated,omitempty"`
	HomeTeam    Team   `json:"homeTeam,omitempty" bson:"homeTeam,omitempty"`
	AwayTeam    Team   `json:"awayTeam,omitempty" bson:"awayTeam,omitempty"`
	Score       Score  `json:"score,omitempty" bson:"score,omitempty"`
	Venue       string `json:"venue,omitempty" bson:"venue,omitempty"`
}

type matchesResponse struct {
	Matches []Match `json:"matches"`
}

type cacheRecord struct {
	Ts      int64   `json:"ts" bson:"ts"`
	Matches []Match `json:"matches" bson:"matches"`
}

// CacheStore is the typed store for World Cup schedule cache records.
type CacheStore = storage.DocStore[cacheRecord]
