package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/abenz1267/elephant/v2/pkg/common"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func openDB() error {
	path := common.CacheFile("gitlab.db")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create cache dir: %v", err)
	}

	var err error
	db, err = sql.Open("sqlite3", path+"?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=10000&_temp_store=memory&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("sql open: %v", err)
	}

	db.SetMaxOpenConns(1)

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS projects (
		id INTEGER PRIMARY KEY,
		path_with_namespace TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		web_url TEXT NOT NULL,
		namespace TEXT DEFAULT '',
		last_activity_at INTEGER DEFAULT 0
	)`)
	if err != nil {
		return fmt.Errorf("create projects table: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS merge_requests (
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
	)`)
	if err != nil {
		return fmt.Errorf("create merge_requests table: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS meta (
		key TEXT PRIMARY KEY,
		value TEXT
	)`)
	if err != nil {
		return fmt.Errorf("create meta table: %v", err)
	}

	return nil
}

func closeDB() {
	if db != nil {
		db.Close()
	}
}

func upsertProjects(projects []Project) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO projects
		(id, path_with_namespace, name, description, web_url, namespace, last_activity_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range projects {
		_, err = stmt.Exec(p.ID, p.PathWithNamespace, p.Name, p.Description, p.WebURL, p.Namespace.FullPath, p.LastActivityAt.Unix())
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func upsertMergeRequests(mrs []MergeRequest, role string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO merge_requests
		(id, iid, title, description, web_url, state, source_branch, target_branch, project_path, author, role, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, mr := range mrs {
		projectPath := ""
		if mr.References.Full != "" {
			// Extract project path from full reference like "group/project!123"
			ref := mr.References.Full
			if idx := lastIndex(ref, '!'); idx > 0 {
				projectPath = ref[:idx]
			}
		}

		_, err = stmt.Exec(mr.ID, mr.IID, mr.Title, mr.Description, mr.WebURL, mr.State,
			mr.SourceBranch, mr.TargetBranch, projectPath, mr.Author.Username, role, mr.CreatedAt.Unix())
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func clearMergeRequests() error {
	_, err := db.Exec("DELETE FROM merge_requests")
	return err
}

func lastIndex(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

type dbProject struct {
	ID                int64
	PathWithNamespace string
	Name              string
	Description       string
	WebURL            string
	Namespace         string
	LastActivityAt    int64
}

type dbMergeRequest struct {
	ID           int64
	IID          int64
	Title        string
	Description  string
	WebURL       string
	State        string
	SourceBranch string
	TargetBranch string
	ProjectPath  string
	Author       string
	Role         string
	CreatedAt    int64
}

func queryProjects(query string) []dbProject {
	var rows *sql.Rows
	var err error

	if query != "" {
		like := "%" + query + "%"
		rows, err = db.Query(`SELECT id, path_with_namespace, name, description, web_url, namespace, last_activity_at
			FROM projects WHERE path_with_namespace LIKE ? OR name LIKE ?
			ORDER BY last_activity_at DESC LIMIT 200`, like, like)
	} else {
		rows, err = db.Query(`SELECT id, path_with_namespace, name, description, web_url, namespace, last_activity_at
			FROM projects ORDER BY last_activity_at DESC LIMIT 50`)
	}

	if err != nil {
		slog.Error(Name, "queryprojects", err)
		return nil
	}
	defer rows.Close()

	var result []dbProject
	for rows.Next() {
		var p dbProject
		if err := rows.Scan(&p.ID, &p.PathWithNamespace, &p.Name, &p.Description, &p.WebURL, &p.Namespace, &p.LastActivityAt); err != nil {
			continue
		}
		result = append(result, p)
	}

	return result
}

func queryMergeRequests(query string) []dbMergeRequest {
	var rows *sql.Rows
	var err error

	if query != "" {
		like := "%" + query + "%"
		rows, err = db.Query(`SELECT id, iid, title, description, web_url, state, source_branch, target_branch, project_path, author, role, created_at
			FROM merge_requests WHERE title LIKE ? OR project_path LIKE ? OR source_branch LIKE ?
			ORDER BY created_at DESC LIMIT 200`, like, like, like)
	} else {
		rows, err = db.Query(`SELECT id, iid, title, description, web_url, state, source_branch, target_branch, project_path, author, role, created_at
			FROM merge_requests ORDER BY created_at DESC LIMIT 50`)
	}

	if err != nil {
		slog.Error(Name, "querymergerequests", err)
		return nil
	}
	defer rows.Close()

	var result []dbMergeRequest
	for rows.Next() {
		var mr dbMergeRequest
		if err := rows.Scan(&mr.ID, &mr.IID, &mr.Title, &mr.Description, &mr.WebURL, &mr.State,
			&mr.SourceBranch, &mr.TargetBranch, &mr.ProjectPath, &mr.Author, &mr.Role, &mr.CreatedAt); err != nil {
			continue
		}
		result = append(result, mr)
	}

	return result
}

func getProjectWebURL(id string) string {
	var url string
	err := db.QueryRow("SELECT web_url FROM projects WHERE id = ?", id).Scan(&url)
	if err != nil {
		return ""
	}
	return url
}

func getMRWebURL(id string) string {
	var url string
	err := db.QueryRow("SELECT web_url FROM merge_requests WHERE id = ?", id).Scan(&url)
	if err != nil {
		return ""
	}
	return url
}
