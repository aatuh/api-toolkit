package countrycodes

import (
	"encoding/csv"
	"io"
	"strings"
	"sync"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

var (
	mu         sync.RWMutex
	codeSet    map[string]struct{}
	countryMap map[string]string
	loaded     bool
)

// LoadCSV populates the country code map from a CSV reader with ISO 3166 data.
// The CSV must have headers and at least columns "name" and "alpha-2".
func LoadCSV(r io.Reader) error {
	mu.Lock()
	defer mu.Unlock()

	codeSet = make(map[string]struct{})
	countryMap = make(map[string]string)

	cr := csv.NewReader(r)
	records, err := cr.ReadAll()
	if err != nil {
		loaded = false
		return err
	}
	for i, row := range records {
		if i == 0 || len(row) < 2 {
			continue
		}
		code := Normalize(row[1])
		name := strings.TrimSpace(row[0])
		if code == "" {
			continue
		}
		codeSet[code] = struct{}{}
		if name != "" {
			countryMap[code] = name
		}
	}
	loaded = true
	return nil
}

// MustLoadCSV loads CSV data or panics. Intended for wiring during startup.
func MustLoadCSV(r io.Reader) {
	if err := LoadCSV(r); err != nil {
		panic(err)
	}
}

// IsLoaded reports whether country data has been loaded.
func IsLoaded() bool {
	mu.RLock()
	defer mu.RUnlock()
	return loaded
}

// Normalize trims whitespace and uppercases the country code.
func Normalize(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// IsValid returns true if the value is an ISO 3166-1 alpha-2 country code.
func IsValid(code string) bool {
	mu.RLock()
	defer mu.RUnlock()
	if code == "" {
		return false
	}
	_, ok := codeSet[Normalize(code)]
	return ok
}

// EnglishName returns the English country name for a code, if available.
func EnglishName(code string) (string, bool) {
	mu.RLock()
	defer mu.RUnlock()
	if !loaded {
		return "", false
	}
	name, ok := countryMap[Normalize(code)]
	return name, ok
}

// LocalizedName returns a localized name using CLDR via golang.org/x/text.
// Falls back to the English name or code if localization is unavailable.
func LocalizedName(code string, locale string) string {
	c := Normalize(code)
	if c == "" {
		return ""
	}
	mu.RLock()
	defer mu.RUnlock()
	if !loaded {
		return c
	}
	tag := language.Make(locale)
	if r, err := language.ParseRegion(c); err == nil {
		if tag == language.Und {
			tag = language.English
		}
		if name := display.Regions(tag).Name(r); name != "" && strings.ToUpper(name) != c {
			return name
		}
	}
	if name, ok := EnglishName(c); ok {
		return name
	}
	return c
}
