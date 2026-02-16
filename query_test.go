package main

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestMultiWordFuzzyScore_RepeatBonus(t *testing.T) {
	// "res infra" should rank a path where "res" appears twice higher than
	// one where it appears only once.
	query := "res infra"

	better := "researchable/general/researchable-infrastructure" // "res" matches twice
	worse := "researchable/infrastructure"                       // "res" matches once

	scoreBetter, _, _ := multiWordFuzzyScore(query, better, false)
	scoreWorse, _, _ := multiWordFuzzyScore(query, worse, false)

	t.Logf("score for %q: %d", better, scoreBetter)
	t.Logf("score for %q: %d", worse, scoreWorse)

	if scoreBetter <= scoreWorse {
		t.Errorf("expected %q (score %d) to rank higher than %q (score %d)",
			better, scoreBetter, worse, scoreWorse)
	}
}

func TestScoreProject_NamePriority(t *testing.T) {
	// "res infra" should prefer a project whose name contains both query
	// words over one where the match is scattered across path segments.
	query := "res infra"

	better := dbProject{
		PathWithNamespace: "researchable/general/researchable-infrastructure",
		Name:              "researchable-infrastructure", // both words match in name
	}
	worse := dbProject{
		PathWithNamespace: "researchable/general/infrastructure/heroku-vsv-infrastructure",
		Name:              "heroku-vsv-infrastructure", // only "infra" matches in name
	}

	scoreBetter := scoreProject(query, better, false)
	scoreWorse := scoreProject(query, worse, false)

	t.Logf("score for %q (name %q): %d", better.PathWithNamespace, better.Name, scoreBetter)
	t.Logf("score for %q (name %q): %d", worse.PathWithNamespace, worse.Name, scoreWorse)

	if scoreBetter <= scoreWorse {
		t.Errorf("expected %q (score %d) to rank higher than %q (score %d)",
			better.PathWithNamespace, scoreBetter, worse.PathWithNamespace, scoreWorse)
	}
}

// setupTestDB creates a temporary SQLite database with representative project
// and merge request data so that Query can be tested end-to-end.
func setupTestDB(t *testing.T) {
	t.Helper()

	// Initialize globals that Query depends on.
	config = &Config{}
	h = nil

	tmpFile, err := os.CreateTemp(t.TempDir(), "gitlab-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	var openErr error
	db, openErr = sql.Open("sqlite3", tmpFile.Name())
	if openErr != nil {
		t.Fatal(openErr)
	}
	t.Cleanup(func() {
		db.Close()
		db = nil
	})

	// Create tables
	for _, ddl := range []string{
		`CREATE TABLE projects (
			id INTEGER PRIMARY KEY,
			path_with_namespace TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			web_url TEXT NOT NULL,
			namespace TEXT DEFAULT '',
			last_activity_at INTEGER DEFAULT 0
		)`,
		`CREATE TABLE merge_requests (
			id INTEGER PRIMARY KEY,
			iid INTEGER NOT NULL,
			title TEXT NOT NULL,
			description TEXT DEFAULT '',
			web_url TEXT NOT NULL,
			state TEXT DEFAULT 'opened',
			source_branch TEXT DEFAULT '',
			target_branch TEXT DEFAULT '',
			project_path TEXT DEFAULT '',
			author TEXT DEFAULT '',
			role TEXT DEFAULT '',
			created_at INTEGER DEFAULT 0
		)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}

	// Insert projects — paths mirror real structure, names are last segment.
	projects := []struct {
		id   int
		path string
		name string
	}{
		{1, "researchable/infrastructure", "infrastructure"},
		{2, "researchable/sport-data-valley/sdv/sdv-infrastructure", "sdv-infrastructure"},
		{3, "researchable/general/researchable-infrastructure", "researchable-infrastructure"},
		{4, "researchable/general/development-infrastructure", "development-infrastructure"},
		{5, "researchable/projects/alpha/infrastructure", "infrastructure"},
		{6, "researchable/projects/beta/infrastructure", "infrastructure"},
		{7, "researchable/general/infrastructure/heroku-vsv-infrastructure", "heroku-vsv-infrastructure"},
		{8, "legalcorp/legalcorp-app", "legalcorp-app"},
	}
	for _, p := range projects {
		_, err := db.Exec(`INSERT INTO projects (id, path_with_namespace, name, web_url, last_activity_at) VALUES (?, ?, ?, ?, ?)`,
			p.id, p.path, p.name, "https://git.example.com/"+p.path, 1000+p.id)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Insert merge requests — titles are generic/obfuscated.
	mrs := []struct {
		id          int
		iid         int
		title       string
		projectPath string
		branch      string
	}{
		{100, 620, "fix: bump retry timeout for background jobs", "researchable/general/researchable-infrastructure", "fix/retry-timeout"},
		{101, 621, "fix: correct permission flags on shared volumes", "researchable/general/researchable-infrastructure", "fix/volume-perms"},
		{102, 17, "chore: update helm chart values", "researchable/projects/alpha/infrastructure", "chore/helm"},
		{103, 27, "feat: add integration test suite", "researchable/projects/beta/infrastructure", "feat/integration-tests"},
		{104, 38, "feat: enable daily snapshots", "researchable/projects/beta/infrastructure", "feat/snapshots"},
		{105, 3, "chore: pin base image version", "researchable/general/infrastructure/heroku-vsv-infrastructure", "chore/pin-image"},
	}
	for _, mr := range mrs {
		_, err := db.Exec(`INSERT INTO merge_requests (id, iid, title, web_url, project_path, source_branch, role, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			mr.id, mr.iid, mr.title, "https://git.example.com/"+mr.projectPath+"/-/merge_requests/"+string(rune(mr.iid+'0')),
			mr.projectPath, mr.branch, "authored", 1000+mr.id)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestQuery_ProjectRanking(t *testing.T) {
	setupTestDB(t)

	// "res infra" through the full Query path should rank
	// researchable-infrastructure (name matches both words) above
	// heroku-vsv-infrastructure (name only matches "infra").
	results := Query(nil, "res infra", false, false, 0)

	if len(results) == 0 {
		t.Fatal("expected project results, got none")
	}

	for _, r := range results {
		t.Logf("project: text=%q sub=%q score=%d", r.Text, r.Subtext, r.Score)
	}

	// Find positions of the two competing projects.
	var posTarget, posVSV int = -1, -1
	for i, r := range results {
		switch r.Subtext {
		case "researchable/general/researchable-infrastructure":
			posTarget = i
		case "researchable/general/infrastructure/heroku-vsv-infrastructure":
			posVSV = i
		}
	}

	if posTarget < 0 {
		t.Fatal("researchable-infrastructure not found in results")
	}
	if posVSV < 0 {
		t.Fatal("heroku-vsv-infrastructure not found in results")
	}

	targetScore := results[posTarget].Score
	vsvScore := results[posVSV].Score

	if targetScore <= vsvScore {
		t.Errorf("expected researchable-infrastructure (score %d) to rank above heroku-vsv-infrastructure (score %d)",
			targetScore, vsvScore)
	}
}

func TestDrillDown_BestProject(t *testing.T) {
	setupTestDB(t)

	// "res infra!" should drill into the best-matching project for "res infra",
	// which is "researchable/general/researchable-infrastructure" (name contains
	// both words), and return only its MRs.
	results := Query(nil, "res infra!", false, false, 0)

	if len(results) == 0 {
		t.Fatal("expected MR results from drill-down, got none")
	}

	for _, r := range results {
		t.Logf("result: text=%q sub=%q score=%d", r.Text, r.Subtext, r.Score)
	}

	for _, r := range results {
		if !strings.Contains(r.Subtext, "researchable/general/researchable-infrastructure") {
			t.Errorf("unexpected project in result: %q", r.Subtext)
		}
	}
}

func TestDrillDown_WithMRQuery(t *testing.T) {
	setupTestDB(t)

	// "res infra!retry" should drill into researchable-infrastructure and
	// filter MRs to those matching "retry".
	results := Query(nil, "res infra!retry", false, false, 0)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if !strings.Contains(results[0].Text, "retry") {
		t.Errorf("expected MR about retry, got %q", results[0].Text)
	}
}

func TestDrillDown_ByIID(t *testing.T) {
	setupTestDB(t)

	// "res infra!620" should match MR with IID 620
	results := Query(nil, "res infra!620", false, false, 0)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if !strings.Contains(results[0].Subtext, "!620") {
		t.Errorf("expected MR !620, got %q", results[0].Subtext)
	}
}
