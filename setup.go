package main

import (
	_ "embed"
	"fmt"
	"log/slog"
	"time"

	"github.com/abenz1267/elephant/v2/pkg/common"
	"github.com/abenz1267/elephant/v2/pkg/common/history"
	"github.com/abenz1267/elephant/v2/pkg/pb/pb"
)

//go:embed README.md
var readme string

var (
	Name       = "gitlab"
	NamePretty = "GitLab"
	config     *Config
	client     *gitlabClient
	userID     int64
	h          *history.History
)

func Available() bool {
	return true
}

func Setup() {
	config = &Config{
		Config: common.Config{
			Icon:     "gitlab",
			MinScore: 20,
		},
		GitLabURL:       "https://gitlab.com",
		PATFile:         "~/.gitlab_pat",
		RefreshInterval: 15,
		MaxProjects:     1000,
		MembershipOnly:  true,
		History:         true,
		Command:         "xdg-open",
	}

	common.LoadConfig(Name, config)

	if config.NamePretty != "" {
		NamePretty = config.NamePretty
	}

	h = history.Load(Name)

	pat := readPAT(config.PATFile)
	if pat == "" {
		slog.Error(Name, "setup", "no PAT found, provider will serve cached data only")
	}

	if err := openDB(); err != nil {
		slog.Error(Name, "setup", err)
		return
	}

	if pat != "" {
		client = newGitLabClient(config.GitLabURL, pat)

		user, err := client.getCurrentUser()
		if err != nil {
			slog.Error(Name, "setup", fmt.Sprintf("failed to get current user: %v", err))
		} else {
			userID = user.ID
			slog.Info(Name, "user", user.Username)
		}

		go syncAll()
		go backgroundRefresh()
	}
}

func syncAll() {
	if client == nil {
		return
	}

	start := time.Now()
	slog.Info(Name, "sync", "starting")

	projects := client.fetchProjects(config.MaxProjects, config.MembershipOnly)
	if len(projects) > 0 {
		if err := upsertProjects(projects); err != nil {
			slog.Error(Name, "sync", fmt.Sprintf("projects: %v", err))
		}
	}
	slog.Info(Name, "sync", fmt.Sprintf("fetched %d projects", len(projects)))

	if err := clearMergeRequests(); err != nil {
		slog.Error(Name, "sync", fmt.Sprintf("clear mrs: %v", err))
	}

	assigned := client.fetchAssignedMRs()
	if len(assigned) > 0 {
		if err := upsertMergeRequests(assigned, "assigned"); err != nil {
			slog.Error(Name, "sync", fmt.Sprintf("assigned mrs: %v", err))
		}
	}

	authored := client.fetchAuthoredMRs()
	if len(authored) > 0 {
		if err := upsertMergeRequests(authored, "authored"); err != nil {
			slog.Error(Name, "sync", fmt.Sprintf("authored mrs: %v", err))
		}
	}

	if userID > 0 {
		reviewing := client.fetchReviewingMRs(userID)
		if len(reviewing) > 0 {
			if err := upsertMergeRequests(reviewing, "reviewing"); err != nil {
				slog.Error(Name, "sync", fmt.Sprintf("reviewing mrs: %v", err))
			}
		}
	}

	slog.Info(Name, "sync", fmt.Sprintf("done in %v", time.Since(start)))
}

func backgroundRefresh() {
	ticker := time.NewTicker(time.Duration(config.RefreshInterval) * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		syncAll()
	}
}

func PrintDoc() {
	fmt.Println(readme)
}

func Icon() string {
	return config.Icon
}

func HideFromProviderlist() bool {
	return config.HideFromProviderlist
}

func State(action string) *pb.ProviderStateResponse {
	return &pb.ProviderStateResponse{
		Actions: []string{ActionRefresh},
	}
}
