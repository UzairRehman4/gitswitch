// Package profile stores gitswitch identities on disk.
package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Profile is one Git/GitHub identity.
type Profile struct {
	Name    string `json:"name"`               // short label, e.g. "work"
	GitName string `json:"git_name"`           // user.name
	Email   string `json:"email"`              // user.email
	GitHub  string `json:"github,omitempty"`   // GitHub username (HTTPS credential hint)
	KeyPath string `json:"key_path,omitempty"` // SSH private key for this identity
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,31}$`)

// Store is the set of saved profiles.
type Store struct {
	Profiles []Profile `json:"profiles"`
	dir      string
}

// Dir returns the gitswitch config directory. GITSWITCH_HOME overrides it.
func Dir() (string, error) {
	if d := os.Getenv("GITSWITCH_HOME"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "gitswitch"), nil
}

// Load reads the store, returning an empty one if none exists yet.
func Load() (*Store, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	s := &Store{dir: dir}
	data, err := os.ReadFile(filepath.Join(dir, "profiles.json"))
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("corrupt profiles.json: %w", err)
	}
	return s, nil
}

// Save writes the store atomically.
func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Join(s.dir, "profiles"), 0o700); err != nil {
		return err
	}
	sort.Slice(s.Profiles, func(i, j int) bool { return s.Profiles[i].Name < s.Profiles[j].Name })
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(s.dir, "profiles.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Get finds a profile by name (case-insensitive).
func (s *Store) Get(name string) (Profile, bool) {
	for _, p := range s.Profiles {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return Profile{}, false
}

// Validate checks a profile before it is added.
func Validate(p Profile) error {
	if !nameRe.MatchString(p.Name) {
		return fmt.Errorf("profile name %q must be 1-32 letters, digits, '-' or '_'", p.Name)
	}
	if strings.TrimSpace(p.GitName) == "" {
		return errors.New("git user name is required")
	}
	if !strings.Contains(p.Email, "@") {
		return fmt.Errorf("email %q does not look valid", p.Email)
	}
	return nil
}

// Add inserts a new profile.
func (s *Store) Add(p Profile) error {
	if err := Validate(p); err != nil {
		return err
	}
	if _, exists := s.Get(p.Name); exists {
		return fmt.Errorf("profile %q already exists", p.Name)
	}
	s.Profiles = append(s.Profiles, p)
	return nil
}

// Remove deletes a profile by name.
func (s *Store) Remove(name string) bool {
	for i, p := range s.Profiles {
		if strings.EqualFold(p.Name, name) {
			s.Profiles = append(s.Profiles[:i], s.Profiles[i+1:]...)
			return true
		}
	}
	return false
}

// MatchIdentity returns the profile whose email (and name, if given) matches.
func (s *Store) MatchIdentity(gitName, email string) (Profile, bool) {
	for _, p := range s.Profiles {
		if strings.EqualFold(p.Email, email) && (gitName == "" || p.GitName == gitName) {
			return p, true
		}
	}
	for _, p := range s.Profiles {
		if strings.EqualFold(p.Email, email) {
			return p, true
		}
	}
	return Profile{}, false
}

// ProfilesDir is where per-profile gitconfig files live.
func (s *Store) ProfilesDir() string { return filepath.Join(s.dir, "profiles") }

// ConfigFile is the path of a profile's generated gitconfig include file.
func (s *Store) ConfigFile(p Profile) string {
	return filepath.Join(s.ProfilesDir(), p.Name+".gitconfig")
}
