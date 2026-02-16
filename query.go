package main

import (
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/abenz1267/elephant/v2/pkg/common"
	"github.com/abenz1267/elephant/v2/pkg/common/history"
	"github.com/abenz1267/elephant/v2/pkg/pb/pb"
)

// multiWordFuzzyScore scores each word in the query independently against the
// target and sums the results. Additional occurrences of a word in the target
// earn a bonus so that "res infra" ranks
// "researchable/general/researchable-infrastructure" above
// "researchable/infrastructure".
func multiWordFuzzyScore(query, target string, exact bool) (int32, []int32, int32) {
	words := strings.Fields(query)
	if len(words) <= 1 {
		return common.FuzzyScore(query, target, exact)
	}

	var totalScore int32
	var allPositions []int32
	var minStart int32 = -1

	lowerTarget := strings.ToLower(target)

	for _, word := range words {
		score, pos, start := common.FuzzyScore(word, target, exact)
		totalScore += score
		allPositions = append(allPositions, pos...)
		if minStart < 0 || (start >= 0 && start < minStart) {
			minStart = start
		}

		// Bonus for each additional occurrence of the word in the target.
		// The first match is already accounted for by FuzzyScore; every
		// extra hit adds half the base score as a bonus.
		occurrences := int32(strings.Count(lowerTarget, strings.ToLower(word)))
		if occurrences > 1 {
			totalScore += (occurrences - 1) * (score / 2)
		}
	}

	slices.Sort(allPositions)

	return totalScore, allPositions, minStart
}

// scoreProject returns a combined score for a project by scoring both the full
// path and the repo name. The name score is doubled so that matches in the
// last path segment (the actual repo name) are prioritised.
func scoreProject(query string, p dbProject, exact bool) int32 {
	pathScore, _, _ := multiWordFuzzyScore(query, p.PathWithNamespace, exact)
	nameScore, _, _ := multiWordFuzzyScore(query, p.Name, exact)
	return pathScore + nameScore*2
}

func Query(conn net.Conn, query string, _ bool, exact bool, _ uint8) []*pb.QueryResponse_Item {
	if db == nil {
		return nil
	}

	if idx := strings.Index(query, "!"); idx >= 0 {
		projectQuery := query[:idx]
		mrQuery := query[idx+1:]

		projects := queryProjects(projectQuery)
		if len(projects) == 0 {
			return nil
		}

		// Pick the single best-matching project.
		best := projects[0]
		if projectQuery != "" {
			bestScore := scoreProject(projectQuery, best, exact)
			for _, p := range projects[1:] {
				s := scoreProject(projectQuery, p, exact)
				if s > bestScore {
					bestScore = s
					best = p
				}
			}
		}
		paths := []string{best.PathWithNamespace}

		mrs := queryMergeRequestsForProjects(paths, mrQuery)
		var entries []*pb.QueryResponse_Item
		for _, mr := range mrs {
			identifier := fmt.Sprintf("mr:%d", mr.ID)
			subtext := fmt.Sprintf("!%d · %s · %s", mr.IID, mr.ProjectPath, mr.Role)

			entry := &pb.QueryResponse_Item{
				Identifier: identifier,
				Text:       mr.Title,
				Subtext:    subtext,
				Icon:       config.Icon,
				Provider:   Name,
				Type:       pb.QueryResponse_REGULAR,
				Actions:    []string{"open", "copy_url"},
			}

			if mrQuery != "" {
				score, pos, start := multiWordFuzzyScore(mrQuery, mr.Title, exact)
				entry.Score = score
				entry.Fuzzyinfo = &pb.QueryResponse_Item_FuzzyInfo{
					Start:     start,
					Field:     "text",
					Positions: pos,
				}
			}

			if config.History {
				usageScore := h.CalcUsageScore(query, identifier)
				if usageScore != 0 {
					entry.State = append(entry.State, history.StateHistory)
				}
				entry.Score += usageScore
			}

			entries = append(entries, entry)
		}

		return entries
	}

	var entries []*pb.QueryResponse_Item

	projects := queryProjects(query)
	for k, p := range projects {
		identifier := fmt.Sprintf("project:%d", p.ID)
		entry := &pb.QueryResponse_Item{
			Identifier: identifier,
			Text:       p.Name,
			Subtext:    p.PathWithNamespace,
			Icon:       config.Icon,
			Provider:   Name,
			Type:       pb.QueryResponse_REGULAR,
			Actions:    []string{"open", "copy_url"},
			Score:      int32(1000 - k),
		}

		if query != "" {
			entry.Score = scoreProject(query, p, exact)

			scoreNs, posNs, startNs := multiWordFuzzyScore(query, p.PathWithNamespace, exact)
			scoreName, posName, startName := multiWordFuzzyScore(query, p.Name, exact)

			if scoreName >= scoreNs {
				entry.Fuzzyinfo = &pb.QueryResponse_Item_FuzzyInfo{
					Start:     startName,
					Field:     "text",
					Positions: posName,
				}
			} else {
				entry.Fuzzyinfo = &pb.QueryResponse_Item_FuzzyInfo{
					Start:     startNs,
					Field:     "subtext",
					Positions: posNs,
				}
			}
		}

		if config.History {
			usageScore := h.CalcUsageScore(query, identifier)
			if usageScore != 0 {
				entry.State = append(entry.State, history.StateHistory)
			}
			entry.Score += usageScore
		}

		entries = append(entries, entry)
	}

	return entries
}
