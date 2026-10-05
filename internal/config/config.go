// Package config stores connection profiles — named endpoints such as "staging" or "prod" —
// in one JSON file with mode 0600, since it holds passwords. The local container is the
// built-in "local" profile and is never stored.
package config

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/phuthuycoding/dbclone/internal/driver"
)

type Profile struct {
	Name   string            `json:"name"`
	Values map[string]string `json:"values"`
}

type Store struct {
	Path     string    `json:"-"`
	Profiles []Profile `json:"profiles"`
}

var validName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// ValidName checks a new profile name.
func ValidName(name string) error {
	switch {
	case name == driver.LocalName:
		return fmt.Errorf("%q is the built-in local container", name)
	case !validName.MatchString(name):
		return errors.New("use letters, digits, - and _ only")
	}
	return nil
}

// DefaultPath is <user config dir>/dbclone/profiles.json.
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "dbclone-profiles.json"
	}
	return filepath.Join(dir, "dbclone", "profiles.json")
}

// Load reads the store; a missing file is an empty store.
func Load(path string) (*Store, error) {
	s := &Store{Path: path}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}

// Save writes the store with mode 0600 — it holds passwords.
func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.Path, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Chmod(s.Path, 0o600) // WriteFile keeps the mode of an existing file
}

// Names returns the profile names, sorted.
func (s *Store) Names() []string {
	names := make([]string, len(s.Profiles))
	for i, p := range s.Profiles {
		names[i] = p.Name
	}
	slices.Sort(names)
	return names
}

func (s *Store) Get(name string) (Profile, bool) {
	for _, p := range s.Profiles {
		if p.Name == name {
			return p, true
		}
	}
	return Profile{}, false
}

// Put adds p, or replaces the profile called old (old may equal p.Name).
func (s *Store) Put(old string, p Profile) {
	for i := range s.Profiles {
		if s.Profiles[i].Name == old {
			s.Profiles[i] = p
			return
		}
	}
	s.Profiles = append(s.Profiles, p)
}

func (s *Store) Delete(name string) {
	s.Profiles = slices.DeleteFunc(s.Profiles, func(p Profile) bool { return p.Name == name })
}

// Endpoint resolves a profile name, "local" included.
func (s *Store) Endpoint(name string) (driver.Endpoint, error) {
	if name == driver.LocalName {
		return driver.Endpoint{Name: name, Local: true}, nil
	}
	p, ok := s.Get(name)
	if !ok {
		return driver.Endpoint{}, fmt.Errorf("no profile %q (have: %s)", name, strings.Join(append(s.Names(), driver.LocalName), ", "))
	}
	return driver.Endpoint{Name: p.Name, Values: p.Values}, nil
}

// legacyKeys maps the settings of the pre-profile .env.staging file.
var legacyKeys = map[string]string{
	"STG_MONGO_URI":      "mongo.uri",
	"STG_MYSQL_HOST":     "mysql.host",
	"STG_MYSQL_PORT":     "mysql.port",
	"STG_MYSQL_USER":     "mysql.user",
	"STG_MYSQL_PASSWORD": "mysql.password",
}

// ImportLegacy turns an old .env.staging file into a "staging" profile, once. It reports
// whether a profile was added.
func (s *Store) ImportLegacy(path string) (bool, error) {
	if _, ok := s.Get("staging"); ok {
		return false, nil
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	values := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
		if !ok || strings.HasPrefix(k, "#") {
			continue
		}
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
			v = v[1 : len(v)-1]
		}
		if key, ok := legacyKeys[strings.TrimSpace(strings.TrimPrefix(k, "export "))]; ok && v != "" {
			values[key] = v
		}
	}
	if err := sc.Err(); err != nil || len(values) == 0 {
		return false, err
	}
	s.Put("staging", Profile{Name: "staging", Values: values})
	return true, s.Save()
}
