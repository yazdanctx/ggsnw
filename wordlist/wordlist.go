package wordlist

import (
	"os"
	"sort"
	"strings"
)

type Wordlist struct {
	words map[string]struct{}
}

func New() *Wordlist {
	return &Wordlist{words: make(map[string]struct{})}
}

func (w *Wordlist) Add(words ...string) {
	for _, word := range words {
		w.words[word] = struct{}{}
	}
}

func (w *Wordlist) All() []string {
	result := make([]string, 0, len(w.words))
	for word := range w.words {
		result = append(result, word)
	}
	sort.Strings(result)
	return result
}

func (w *Wordlist) Count() int {
	return len(w.words)
}

func (w *Wordlist) Clear() {
	w.words = make(map[string]struct{})
}

func (w *Wordlist) Export(path string) error {
	data := strings.Join(w.All(), "\n") + "\n"
	return os.WriteFile(path, []byte(data), 0o644)
}
