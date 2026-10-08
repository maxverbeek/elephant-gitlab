package main

import (
	_ "embed"
	"fmt"
	"log/slog"
	"net/url"
	"sync"
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
	syncMu     sync.Mutex
)

func Available() bool {
	return true
}

func LoadConfig() {
	config = &Config{
		Config: common.Config{
			Icon:     "gitlab",
			MinScore: 20,
		},
		GitLabURL:       "https://gitlab.com",
		RefreshInterval: 15,
		MaxProjects:     1000,
		MembershipOnly:  true,
		History:         true,
		Command:         "xdg-open",
	}

	common.LoadConfig(Name, config)
}

func Setup() {
	LoadConfig()

	if config.NamePretty != "" {
		NamePretty = config.NamePretty
	}

	h = history.Load(Name)

	if err := openDB(); err != nil {
		slog.Error(Name, "setup", err)
		return
	}

	go syncAll()
	go backgroundRefresh()
}

// refreshClient re-reads the token and rebuilds the client when it was rotated.
func refreshClient() bool {
	u, err := url.Parse(config.GitLabURL)
	if err != nil || u.Hostname() == "" {
		slog.Error(Name, "token", fmt.Sprintf("invalid gitlab_url %q", config.GitLabURL))
		return false
	}

	pat, err := lookupToken(u.Hostname())
	if err != nil {
		slog.Error(Name, "token", err)
		return false
	}

	if client != nil && client.pat == pat {
		return true
	}

	client = newGitLabClient(config.GitLabURL, pat)

	user, err := client.getCurrentUser()
	if err != nil {
		slog.Error(Name, "setup", fmt.Sprintf("failed to get current user: %v", err))
	} else {
		userID = user.ID
		slog.Info(Name, "user", user.Username)
	}

	return true
}

func syncAll() {
	syncMu.Lock()
	defer syncMu.Unlock()

	if !refreshClient() {
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

func PrintDoc(write bool) {
	if !write {
		fmt.Println(readme)
		fmt.Println()
	}
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
