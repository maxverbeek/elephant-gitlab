package main

import (
	"github.com/abenz1267/elephant/v2/pkg/common"
)

type Config struct {
	common.Config   `koanf:",squash"`
	GitLabURL       string `koanf:"gitlab_url" desc:"base URL of the GitLab instance; its host selects the Secret Service item" default:"https://gitlab.com"`
	RefreshInterval int    `koanf:"refresh_interval" desc:"minutes between background API refreshes" default:"15"`
	MaxProjects     int    `koanf:"max_projects" desc:"maximum number of projects to fetch" default:"1000"`
	MembershipOnly  bool   `koanf:"membership_only" desc:"only fetch projects the user is a member of" default:"true"`
	History         bool   `koanf:"history" desc:"enable history-based scoring" default:"true"`
	Command         string `koanf:"command" desc:"command used to open URLs" default:"xdg-open"`
}
