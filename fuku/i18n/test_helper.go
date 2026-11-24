//go:build testtools

package i18n

import (
	"crypto/sha256"
	"embed"
	"errors"
	"sort"
	"sync"
)

// testManagers holds one LocaleManager per distinct set of YAML texts a test installed, keyed
// by a SHA-256 of the language codes and texts. A manager built from given texts never changes
// afterwards (its translators and lookup index are derived from the parsed maps, which nothing
// writes after the parse, and Initialize refuses a manager that already has a localeFS), so a
// second override with the same texts installs the same manager instead of parsing the files
// again. Parsing the real locale files is about a second under -race, and fuku/modules does it
// in hundreds of tests.
var (
	testManagersMu sync.Mutex
	testManagers   = map[[sha256.Size]byte]*LocaleManager{}
)

// testManagerFor returns the cached manager for yamlByLang, building it with build on a miss.
func testManagerFor(yamlByLang map[string]string, build func() (*LocaleManager, error)) (*LocaleManager, error) {
	codes := make([]string, 0, len(yamlByLang))
	for code := range yamlByLang {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	h := sha256.New()
	for _, code := range codes {
		content := yamlByLang[code]
		h.Write([]byte(code))
		h.Write([]byte{0})
		h.Write([]byte(content))
		h.Write([]byte{0})
	}
	var key [sha256.Size]byte
	copy(key[:], h.Sum(nil))

	testManagersMu.Lock()
	defer testManagersMu.Unlock()
	if lm, ok := testManagers[key]; ok {
		return lm, nil
	}
	lm, err := build()
	if err != nil {
		return nil, err
	}
	testManagers[key] = lm
	return lm, nil
}

// installTestManager makes lm the global manager and returns the function that puts the
// previous one back.
func installTestManager(lm *LocaleManager) func() {
	// Ensure managerOnce.Do has already fired before we overwrite managerInstance.
	// If we set managerInstance first and Then GetManager() is called, Once.Do would
	// fire and overwrite our value with the empty default singleton.
	_ = GetManager()
	prev := managerInstance
	managerInstance = lm
	return func() { managerInstance = prev }
}

func NewTestTranslator(yamlContent string) (*Translator, error) {
	data, err := parseYAML([]byte(yamlContent))
	if err != nil {
		return nil, err
	}
	lm := &LocaleManager{
		defaultLang: "en",
		localeMaps:  map[string]map[string]any{"en": data},
	}
	return &Translator{
		langCode: "en",
		manager:  lm,
		data:     data,
	}, nil
}

// OverrideManagerForTest replaces the global LocaleManager singleton with one backed
// by the provided inline YAML content. It returns a restore function that reverts
// managerInstance to its prior value; call it via t.Cleanup. This is intentionally
// NOT goroutine-safe across parallel sub-tests that also call this function — run
// the parent test without t.Parallel() or ensure sub-tests share the same override.
//
// It is guarded by the testtools build tag and is never compiled into production.
func OverrideManagerForTest(yamlContent string) (restore func(), err error) {
	lm, err := testManagerFor(map[string]string{"en": yamlContent}, func() (*LocaleManager, error) {
		data, parseErr := parseYAML([]byte(yamlContent))
		if parseErr != nil {
			return nil, parseErr
		}
		// A non-nil *embed.FS is required to pass GetTranslator's localeFS nil-guard.
		// The value is never used for file I/O because localeMaps is already populated.
		var dummyFS embed.FS
		return &LocaleManager{
			defaultLang: "en",
			localeMaps:  map[string]map[string]any{"en": data},
			localeFS:    &dummyFS,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return installTestManager(lm), nil
}

// OverrideManagerLanguagesForTest is OverrideManagerForTest for several languages at once:
// yamlByLang maps a language code to the full YAML text of its locale file, and "en" must be
// among them (it is the default). A test that needs the real words of a language, for
// example to prove a menu speaks the group's language, passes the real locale files. It
// returns a restore function to call from t.Cleanup, and is guarded by the testtools build
// tag like the helpers above.
func OverrideManagerLanguagesForTest(yamlByLang map[string]string) (restore func(), err error) {
	if _, ok := yamlByLang["en"]; !ok {
		return nil, errors.New("OverrideManagerLanguagesForTest: the default language en is missing")
	}
	lm, err := testManagerFor(yamlByLang, func() (*LocaleManager, error) {
		localeMaps := make(map[string]map[string]any, len(yamlByLang))
		for code, content := range yamlByLang {
			data, parseErr := parseYAML([]byte(content))
			if parseErr != nil {
				return nil, parseErr
			}
			localeMaps[code] = data
		}
		var dummyFS embed.FS
		return &LocaleManager{
			defaultLang: "en",
			localeMaps:  localeMaps,
			localeFS:    &dummyFS,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return installTestManager(lm), nil
}
