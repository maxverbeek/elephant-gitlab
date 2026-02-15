package main

import (
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"strings"
	"syscall"

	"github.com/abenz1267/elephant/v2/pkg/common"
	"github.com/abenz1267/elephant/v2/pkg/common/history"
)

const (
	ActionOpen     = "open"
	ActionCopyURL  = "copy_url"
	ActionRefresh  = "refresh"
)

func Activate(single bool, identifier, action string, query string, args string, format uint8, conn net.Conn) {
	if action == "" {
		action = ActionOpen
	}

	switch action {
	case history.ActionDelete:
		h.Remove(identifier)
		return
	case ActionRefresh:
		go syncAll()
		return
	case ActionOpen:
		url := resolveURL(identifier)
		if url == "" {
			slog.Error(Name, "activate", "url not found", "identifier", identifier)
			return
		}

		run := strings.TrimSpace(fmt.Sprintf("%s %s '%s'", common.LaunchPrefix(), config.Command, url))
		cmd := exec.Command("sh", "-c", run)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

		if err := cmd.Start(); err != nil {
			slog.Error(Name, "actionopen", err)
		} else {
			go cmd.Wait()
		}
	case ActionCopyURL:
		url := resolveURL(identifier)
		if url == "" {
			slog.Error(Name, "activate", "url not found", "identifier", identifier)
			return
		}

		cmd := exec.Command("wl-copy", url)
		if err := cmd.Start(); err != nil {
			slog.Error(Name, "actioncopyurl", err)
		} else {
			go cmd.Wait()
		}
	default:
		slog.Error(Name, "activate", fmt.Sprintf("unknown action: %s", action))
		return
	}

	if config.History {
		h.Save(query, identifier)
	}
}

func resolveURL(identifier string) string {
	if strings.HasPrefix(identifier, "project:") {
		return getProjectWebURL(strings.TrimPrefix(identifier, "project:"))
	}
	if strings.HasPrefix(identifier, "mr:") {
		return getMRWebURL(strings.TrimPrefix(identifier, "mr:"))
	}
	return ""
}
