package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type Namespace struct {
	FullPath string `json:"full_path"`
}

type Project struct {
	ID                int64     `json:"id"`
	PathWithNamespace string    `json:"path_with_namespace"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	WebURL            string    `json:"web_url"`
	Namespace         Namespace `json:"namespace"`
	LastActivityAt    time.Time `json:"last_activity_at"`
}

type MRAuthor struct {
	Username string `json:"username"`
}

type MRReferences struct {
	Full string `json:"full"`
}

type MergeRequest struct {
	ID           int64        `json:"id"`
	IID          int64        `json:"iid"`
	Title        string       `json:"title"`
	Description  string       `json:"description"`
	WebURL       string       `json:"web_url"`
	State        string       `json:"state"`
	SourceBranch string       `json:"source_branch"`
	TargetBranch string       `json:"target_branch"`
	Author       MRAuthor     `json:"author"`
	References   MRReferences `json:"references"`
	CreatedAt    time.Time    `json:"created_at"`
}

type GitLabUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type gitlabClient struct {
	baseURL    string
	pat        string
	httpClient *http.Client
}

func newGitLabClient(baseURL, pat string) *gitlabClient {
	return &gitlabClient{
		baseURL: baseURL,
		pat:     pat,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *gitlabClient) request(endpoint string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.baseURL+endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("PRIVATE-TOKEN", c.pat)
	return c.httpClient.Do(req)
}

func (c *gitlabClient) getCurrentUser() (*GitLabUser, error) {
	resp, err := c.request("/api/v4/user")
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var user GitLabUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	return &user, nil
}

func (c *gitlabClient) fetchProjects(maxProjects int, membershipOnly bool) []Project {
	var all []Project
	page := 1

	for len(all) < maxProjects {
		endpoint := fmt.Sprintf("/api/v4/projects?per_page=100&page=%d&order_by=last_activity_at", page)
		if membershipOnly {
			endpoint += "&membership=true"
		}

		resp, err := c.request(endpoint)
		if err != nil {
			slog.Error(Name, "fetchprojects", err)
			break
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			slog.Error(Name, "fetchprojects", fmt.Sprintf("status %d", resp.StatusCode))
			break
		}

		if err != nil {
			slog.Error(Name, "fetchprojects", err)
			break
		}

		var projects []Project
		if err := json.Unmarshal(body, &projects); err != nil {
			slog.Error(Name, "fetchprojects", err)
			break
		}

		if len(projects) == 0 {
			break
		}

		all = append(all, projects...)

		nextPage := resp.Header.Get("X-Next-Page")
		if nextPage == "" {
			break
		}
		page++
	}

	if len(all) > maxProjects {
		all = all[:maxProjects]
	}

	return all
}

func (c *gitlabClient) fetchMergeRequests(endpoint string) []MergeRequest {
	var all []MergeRequest
	page := 1

	for {
		sep := "&"
		if page == 1 && len(endpoint) > 0 && endpoint[len(endpoint)-1] == '?' {
			sep = ""
		}

		url := fmt.Sprintf("%s%sper_page=100&page=%d", endpoint, sep, page)
		resp, err := c.request(url)
		if err != nil {
			slog.Error(Name, "fetchmrs", err)
			break
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			slog.Error(Name, "fetchmrs", fmt.Sprintf("status %d", resp.StatusCode))
			break
		}

		if err != nil {
			slog.Error(Name, "fetchmrs", err)
			break
		}

		var mrs []MergeRequest
		if err := json.Unmarshal(body, &mrs); err != nil {
			slog.Error(Name, "fetchmrs", err)
			break
		}

		if len(mrs) == 0 {
			break
		}

		all = append(all, mrs...)

		nextPage := resp.Header.Get("X-Next-Page")
		if nextPage == "" {
			break
		}
		page++
	}

	return all
}

func (c *gitlabClient) fetchAssignedMRs() []MergeRequest {
	return c.fetchMergeRequests("/api/v4/merge_requests?scope=assigned_to_me&state=opened")
}

func (c *gitlabClient) fetchAuthoredMRs() []MergeRequest {
	return c.fetchMergeRequests("/api/v4/merge_requests?scope=created_by_me&state=opened")
}

func (c *gitlabClient) fetchReviewingMRs(userID int64) []MergeRequest {
	return c.fetchMergeRequests(fmt.Sprintf("/api/v4/merge_requests?reviewer_id=%d&scope=all&state=opened", userID))
}
