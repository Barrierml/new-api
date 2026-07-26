package service

import (
	"cmp"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const maxUpstreamQuotaFileSize = 2 << 20

type UpstreamQuotaWindow struct {
	Key             string   `json:"key,omitempty"`
	Kind            string   `json:"kind,omitempty"`
	Label           string   `json:"label,omitempty"`
	Unit            string   `json:"unit,omitempty"`
	Limit           *float64 `json:"limit,omitempty"`
	Used            *float64 `json:"used,omitempty"`
	Remaining       *float64 `json:"remaining,omitempty"`
	RemainingPct    *float64 `json:"remaining_pct,omitempty"`
	ResetAt         *int64   `json:"reset_at,omitempty"`
	DurationSeconds *int64   `json:"duration_seconds,omitempty"`
}

type UpstreamQuotaOwnership struct {
	AccountIDs []int64 `json:"account_ids,omitempty"`
	GroupIDs   []int64 `json:"group_ids,omitempty"`
	ChannelIDs []int64 `json:"channel_ids,omitempty"`
}

type UpstreamQuotaChannel struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Priority int64  `json:"priority"`
	Status   int    `json:"status"`
}

type UpstreamQuotaEntity struct {
	EntityID              string                 `json:"entity_id"`
	DisplayName           string                 `json:"display_name"`
	Provider              string                 `json:"provider"`
	Status                string                 `json:"status"`
	StatusMessage         string                 `json:"status_message"`
	AccountIDs            []int64                `json:"account_ids,omitempty"`
	GroupIDs              []int64                `json:"group_ids,omitempty"`
	ChannelIDs            []int64                `json:"channel_ids,omitempty"`
	Channels              []UpstreamQuotaChannel `json:"channels,omitempty"`
	Windows               []UpstreamQuotaWindow  `json:"windows,omitempty"`
	FetchedAt             time.Time              `json:"fetched_at,omitempty"`
	AvailableAccountCount *int                   `json:"available_account_count,omitempty"`
	Stale                 bool                   `json:"stale"`
}

type UpstreamQuotaCounts struct {
	Available int `json:"available"`
	Limited   int `json:"limited"`
	Exhausted int `json:"exhausted"`
	Unknown   int `json:"unknown"`
	Error     int `json:"error"`
	Deferred  int `json:"deferred"`
	Stale     int `json:"stale"`
}

type UpstreamQuotaDashboard struct {
	GeneratedAt time.Time             `json:"generated_at"`
	EntityCount int                   `json:"entity_count"`
	Counts      UpstreamQuotaCounts   `json:"counts"`
	Entities    []UpstreamQuotaEntity `json:"entities"`
	Stale       bool                  `json:"stale"`
}

type upstreamQuotaReport struct {
	GeneratedAt time.Time `json:"generated_at"`
	Entities    []struct {
		EntityID              string                `json:"entity_id"`
		DisplayName           string                `json:"display_name"`
		Channel               string                `json:"channel"`
		Status                string                `json:"status"`
		Windows               []UpstreamQuotaWindow `json:"windows"`
		FetchedAt             time.Time             `json:"fetched_at"`
		AvailableAccountCount *int                  `json:"available_account_count"`
	} `json:"entities"`
}

type upstreamQuotaOwnershipFile struct {
	Entities map[string]UpstreamQuotaOwnership `json:"entities"`
}

func LoadUpstreamQuotaDashboard(reportPath, ownershipPath string, now time.Time, maxAge time.Duration) (*UpstreamQuotaDashboard, error) {
	if maxAge <= 0 {
		return nil, fmt.Errorf("quota max age must be positive")
	}
	if now.IsZero() {
		return nil, fmt.Errorf("current time is required")
	}

	reportData, err := readUpstreamQuotaFile(reportPath)
	if err != nil {
		return nil, fmt.Errorf("read quota report: %w", err)
	}
	var report upstreamQuotaReport
	if err := common.Unmarshal(reportData, &report); err != nil {
		return nil, fmt.Errorf("decode quota report: %w", err)
	}
	if report.GeneratedAt.IsZero() {
		return nil, fmt.Errorf("quota report has no generated_at")
	}

	ownership := upstreamQuotaOwnershipFile{Entities: map[string]UpstreamQuotaOwnership{}}
	if strings.TrimSpace(ownershipPath) != "" {
		ownershipData, err := readUpstreamQuotaFile(ownershipPath)
		if err != nil {
			return nil, fmt.Errorf("read quota ownership: %w", err)
		}
		if err := common.Unmarshal(ownershipData, &ownership); err != nil {
			return nil, fmt.Errorf("decode quota ownership: %w", err)
		}
	}

	dashboard := &UpstreamQuotaDashboard{GeneratedAt: report.GeneratedAt}
	dashboard.Stale = report.GeneratedAt.After(now.Add(time.Minute)) || now.Sub(report.GeneratedAt) > maxAge
	seenEntityIDs := make(map[string]struct{}, len(report.Entities))
	for _, source := range report.Entities {
		if strings.TrimSpace(source.EntityID) == "" {
			return nil, fmt.Errorf("quota report contains an entity with no entity_id")
		}
		if _, exists := seenEntityIDs[source.EntityID]; exists {
			return nil, fmt.Errorf("quota report contains duplicate entity_id %q", source.EntityID)
		}
		seenEntityIDs[source.EntityID] = struct{}{}

		status := source.Status
		if !slices.Contains([]string{"available", "limited", "exhausted", "unknown", "error", "deferred"}, status) {
			status = "unknown"
		}
		entityOwnership := ownership.Entities[source.EntityID]
		channels, err := loadUpstreamQuotaChannels(entityOwnership.ChannelIDs)
		if err != nil {
			return nil, fmt.Errorf("load channels for entity %q: %w", source.EntityID, err)
		}
		entity := UpstreamQuotaEntity{
			EntityID: source.EntityID, DisplayName: source.DisplayName, Provider: source.Channel,
			Status: status, StatusMessage: upstreamQuotaStatusMessage(status),
			AccountIDs: entityOwnership.AccountIDs, GroupIDs: entityOwnership.GroupIDs, ChannelIDs: entityOwnership.ChannelIDs,
			Channels: channels, Windows: source.Windows, FetchedAt: source.FetchedAt,
			AvailableAccountCount: source.AvailableAccountCount,
		}
		entity.Stale = dashboard.Stale || source.FetchedAt.IsZero() || source.FetchedAt.After(now.Add(time.Minute)) || now.Sub(source.FetchedAt) > maxAge
		if entity.Stale {
			entity.StatusMessage = "Quota snapshot is stale"
			dashboard.Counts.Stale++
		} else {
			incrementUpstreamQuotaCount(&dashboard.Counts, status)
		}
		dashboard.Entities = append(dashboard.Entities, entity)
	}
	dashboard.EntityCount = len(dashboard.Entities)
	return dashboard, nil
}

func loadUpstreamQuotaChannels(ids []int64) ([]UpstreamQuotaChannel, error) {
	if len(ids) == 0 || model.DB == nil {
		return nil, nil
	}
	rows, err := model.GetChannelPublicSummariesByIDs(ids)
	if err != nil {
		return nil, err
	}
	channels := make([]UpstreamQuotaChannel, 0, len(rows))
	for _, row := range rows {
		channels = append(channels, UpstreamQuotaChannel{
			ID: row.ID, Name: row.Name, Priority: row.Priority, Status: row.Status,
		})
	}
	slices.SortFunc(channels, func(a, b UpstreamQuotaChannel) int {
		if priorityOrder := cmp.Compare(b.Priority, a.Priority); priorityOrder != 0 {
			return priorityOrder
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return channels, nil
}

func readUpstreamQuotaFile(path string) ([]byte, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("file path is not configured")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > maxUpstreamQuotaFileSize {
		return nil, fmt.Errorf("file exceeds %d bytes", maxUpstreamQuotaFileSize)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func upstreamQuotaStatusMessage(status string) string {
	switch status {
	case "available":
		return "Quota is available"
	case "limited":
		return "Quota is limited"
	case "exhausted":
		return "Quota is exhausted"
	case "deferred":
		return "Collection is deferred"
	case "error":
		return "Quota collection failed"
	default:
		return "Quota data is unavailable"
	}
}

func incrementUpstreamQuotaCount(counts *UpstreamQuotaCounts, status string) {
	switch status {
	case "available":
		counts.Available++
	case "limited":
		counts.Limited++
	case "exhausted":
		counts.Exhausted++
	case "error":
		counts.Error++
	case "deferred":
		counts.Deferred++
	default:
		counts.Unknown++
	}
}
