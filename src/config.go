package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/abenz1267/elephant/v2/pkg/common"
)

type Config struct {
	common.Config   `koanf:",squash"`
	GitLabURL       string `koanf:"gitlab_url" desc:"base URL of the GitLab instance" default:"https://gitlab.com"`
	PATFile         string `koanf:"pat_file" desc:"path to file containing a GitLab personal access token" default:"~/.gitlab_pat"`
	RefreshInterval int    `koanf:"refresh_interval" desc:"minutes between background API refreshes" default:"15"`
	MaxProjects     int    `koanf:"max_projects" desc:"maximum number of projects to fetch" default:"1000"`
	MembershipOnly  bool   `koanf:"membership_only" desc:"only fetch projects the user is a member of" default:"true"`
	History         bool   `koanf:"history" desc:"enable history-based scoring" default:"true"`
	Command         string `koanf:"command" desc:"command used to open URLs" default:"xdg-open"`
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			slog.Error(Name, "expandpath", err)
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func readPAT(path string) string {
	path = expandPath(path)

	data, err := os.ReadFile(path)
	if err != nil {
		slog.Error(Name, "readpat", err)
		return ""
	}

	return strings.TrimSpace(string(data))
}
