package countrycodes

import (
	"strings"
	"testing"
)

func TestLoadCSVKeepsExistingDataOnParseError(t *testing.T) {
	restore := snapshotState()
	t.Cleanup(func() {
		restoreState(restore)
	})

	if err := LoadCSV(strings.NewReader("name,alpha-2\nFinland,FI\nSweden,SE\n")); err != nil {
		t.Fatalf("LoadCSV valid input: %v", err)
	}

	if err := LoadCSV(strings.NewReader("\"unterminated")); err == nil {
		t.Fatal("expected parse error")
	}

	if !IsLoaded() {
		t.Fatal("expected dataset to remain loaded after failed reload")
	}
	if !IsValid("FI") {
		t.Fatal("expected existing country code to remain valid after failed reload")
	}
	name, ok := EnglishName("FI")
	if !ok {
		t.Fatal("expected existing country name to remain available after failed reload")
	}
	if name != "Finland" {
		t.Fatalf("expected Finland to remain loaded, got %q", name)
	}
}

type stateSnapshot struct {
	codes     map[string]struct{}
	countries map[string]string
	loaded    bool
}

func snapshotState() stateSnapshot {
	mu.RLock()
	defer mu.RUnlock()

	codes := make(map[string]struct{}, len(codeSet))
	for code := range codeSet {
		codes[code] = struct{}{}
	}

	countries := make(map[string]string, len(countryMap))
	for code, name := range countryMap {
		countries[code] = name
	}

	return stateSnapshot{
		codes:     codes,
		countries: countries,
		loaded:    loaded,
	}
}

func restoreState(snapshot stateSnapshot) {
	mu.Lock()
	defer mu.Unlock()

	codeSet = make(map[string]struct{}, len(snapshot.codes))
	for code := range snapshot.codes {
		codeSet[code] = struct{}{}
	}

	countryMap = make(map[string]string, len(snapshot.countries))
	for code, name := range snapshot.countries {
		countryMap[code] = name
	}

	loaded = snapshot.loaded
}
