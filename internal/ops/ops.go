// Package ops holds the profile operations shared by the CLI and the picker.
package ops

import (
	"errors"
	"fmt"
	"os"

	"github.com/uzairrehman4/gitswitch/internal/gitx"
	"github.com/uzairrehman4/gitswitch/internal/profile"
)

// AddResult describes what Add did.
type AddResult struct {
	Profile   profile.Profile
	PublicKey string // empty when the profile has no key
	Generated bool   // true if a new key was created
}

// Add validates and stores a profile. If p.KeyPath is empty and genKey is true
// an ed25519 key is generated (or an existing gitswitch key file is reused).
func Add(s *profile.Store, p profile.Profile, genKey bool) (AddResult, error) {
	res := AddResult{Profile: p}
	if err := profile.Validate(p); err != nil {
		return res, err
	}
	if _, exists := s.Get(p.Name); exists {
		return res, fmt.Errorf("profile %q already exists", p.Name)
	}
	switch {
	case p.KeyPath != "":
		if _, err := os.Stat(p.KeyPath); err != nil {
			return res, fmt.Errorf("key not found: %s", p.KeyPath)
		}
	case genKey:
		path, err := gitx.DefaultKeyPath(p.Name)
		if err != nil {
			return res, err
		}
		if res.Generated, err = gitx.GenerateKey(path, p.Email); err != nil {
			return res, err
		}
		p.KeyPath = path
	}
	if err := s.Add(p); err != nil {
		return res, err
	}
	if err := s.Save(); err != nil {
		return res, err
	}
	if err := gitx.WriteConfigFile(s.ConfigFile(p), p); err != nil {
		return res, err
	}
	res.Profile = p
	if p.KeyPath != "" {
		pub, err := gitx.PublicKey(p.KeyPath)
		if err != nil {
			return res, err
		}
		res.PublicKey = pub
	}
	return res, nil
}

// Edit replaces the profile called name with updated. The name cannot change.
// If the profile was the active global identity, the new values are applied.
func Edit(s *profile.Store, name string, updated profile.Profile) error {
	old, ok := s.Get(name)
	if !ok {
		return fmt.Errorf("no profile named %q", name)
	}
	updated.Name = old.Name
	if err := profile.Validate(updated); err != nil {
		return err
	}
	if updated.KeyPath != "" {
		if _, err := os.Stat(updated.KeyPath); err != nil {
			return fmt.Errorf("key not found: %s", updated.KeyPath)
		}
	}
	g := gitx.GlobalIdentity()
	wasActive := g.Email != "" && g.Email == old.Email && g.Name == old.GitName
	s.Replace(updated)
	if err := s.Save(); err != nil {
		return err
	}
	if err := gitx.WriteConfigFile(s.ConfigFile(updated), updated); err != nil {
		return err
	}
	if wasActive {
		return gitx.Apply(updated, gitx.Global, s.Profiles)
	}
	return nil
}

// Remove deletes a profile, its folder rules and its include file. The SSH key
// file is left in place.
func Remove(s *profile.Store, name string) error {
	p, ok := s.Get(name)
	if !ok {
		return fmt.Errorf("no profile named %q", name)
	}
	gitx.UnlinkProfile(s.ProfilesDir(), s.ConfigFile(p))
	s.Remove(p.Name)
	if err := s.Save(); err != nil {
		return err
	}
	os.Remove(s.ConfigFile(p))
	return nil
}

// ImportGlobal saves the current global git identity as a profile.
func ImportGlobal(s *profile.Store, label string) (profile.Profile, error) {
	g := gitx.GlobalIdentity()
	if g.Email == "" || g.Name == "" {
		return profile.Profile{}, errors.New("no global git identity (user.name and user.email) to import")
	}
	if existing, ok := s.MatchIdentity(g.Name, g.Email); ok {
		return existing, fmt.Errorf("the global identity is already profile %q", existing.Name)
	}
	p := profile.Profile{Name: label, GitName: g.Name, Email: g.Email}
	if err := s.Add(p); err != nil {
		return p, err
	}
	if err := s.Save(); err != nil {
		return p, err
	}
	return p, gitx.WriteConfigFile(s.ConfigFile(p), p)
}
