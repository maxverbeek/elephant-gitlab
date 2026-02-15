package main

import (
	"fmt"
	"net"

	"github.com/abenz1267/elephant/v2/pkg/common"
	"github.com/abenz1267/elephant/v2/pkg/common/history"
	"github.com/abenz1267/elephant/v2/pkg/pb/pb"
)

func Query(conn net.Conn, query string, _ bool, exact bool, _ uint8) []*pb.QueryResponse_Item {
	if db == nil {
		return nil
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
			scoreNs, posNs, startNs := common.FuzzyScore(query, p.PathWithNamespace, exact)
			scoreName, posName, startName := common.FuzzyScore(query, p.Name, exact)

			if scoreName >= scoreNs {
				entry.Score = scoreName
				entry.Fuzzyinfo = &pb.QueryResponse_Item_FuzzyInfo{
					Start:     startName,
					Field:     "text",
					Positions: posName,
				}
			} else {
				entry.Score = scoreNs
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

	mrs := queryMergeRequests(query)
	for k, mr := range mrs {
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
			Score:      int32(1000 - k),
		}

		if query != "" {
			score, pos, start := common.FuzzyScore(query, mr.Title, exact)
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
