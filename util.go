package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func lookGit() (string, error) {
	p, err := exec.LookPath("git")
	if err != nil {
		return "", fmt.Errorf("git was not found on PATH; install it from https://git-scm.com")
	}
	return p, nil
}

func remoteURL() (string, error) {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	return strings.TrimSpace(string(out)), err
}
