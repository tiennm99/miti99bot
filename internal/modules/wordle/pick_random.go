package wordle

import (
	"errors"
	"math/rand"
)

// errEmptyWordList is returned by pickRandom when the dictionary is empty.
// startFresh propagates it so the handler fails instead of saving a round
// with no answer.
var errEmptyWordList = errors.New("wordle: word list is empty")

// pickRandom returns a uniformly random word. rng allows tests to inject a deterministic source. When rng is nil we fall
// through to math/rand's package-level Intn, which IS goroutine-safe via an
// internal mutex on the global Source — important because the bot dispatcher
// runs each Telegram update in its own goroutine and concurrent /wordle_new
// calls would otherwise race on a shared *rand.Rand.
func pickRandom(words []string, rng *rand.Rand) (string, error) {
	if len(words) == 0 {
		return "", errEmptyWordList
	}
	if rng != nil {
		return words[rng.Intn(len(words))], nil
	}
	return words[rand.Intn(len(words))], nil
}
