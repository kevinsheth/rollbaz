package rollbar

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/kevinsheth/rollbaz/internal/domain"
)

type Item struct {
	ID                      domain.ItemID   `json:"id"`
	ProjectID               uint64          `json:"project_id"`
	Counter                 uint64          `json:"counter"`
	Title                   string          `json:"title"`
	Status                  string          `json:"status"`
	Environment             string          `json:"environment"`
	Level                   string          `json:"level"`
	LastOccurrenceID        *uint64         `json:"last_occurrence_id"`
	LastOccurrenceTimestamp *uint64         `json:"last_occurrence_timestamp"`
	Occurrences             *uint64         `json:"occurrences"`
	TotalOccurrences        *uint64         `json:"total_occurrences"`
	Raw                     json.RawMessage `json:"-"`
}

type ItemPatch struct {
	Status                    string `json:"status,omitempty"`
	ResolvedInVersion         string `json:"resolved_in_version,omitempty"`
	SnoozeEnabled             *bool  `json:"snooze_enabled,omitempty"`
	SnoozeExpirationInSeconds *int64 `json:"snooze_expiration_in_seconds,omitempty"`
}

func (i *Item) UnmarshalJSON(data []byte) error {
	type itemDTO struct {
		ID                      flexibleUint64 `json:"id"`
		ProjectID               uint64         `json:"project_id"`
		Counter                 uint64         `json:"counter"`
		Title                   string         `json:"title"`
		Status                  string         `json:"status"`
		Environment             string         `json:"environment"`
		Level                   flexibleLevel  `json:"level"`
		LastOccurrenceID        *uint64        `json:"last_occurrence_id"`
		LastOccurrenceTimestamp *uint64        `json:"last_occurrence_timestamp"`
		Occurrences             *uint64        `json:"occurrences"`
		TotalOccurrences        *uint64        `json:"total_occurrences"`
	}

	var dto itemDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return fmt.Errorf("decode item json: %w", err)
	}

	i.ID = domain.ItemID(dto.ID)
	i.ProjectID = dto.ProjectID
	i.Counter = dto.Counter
	i.Title = dto.Title
	i.Status = dto.Status
	i.Environment = dto.Environment
	i.Level = string(dto.Level)
	i.LastOccurrenceID = dto.LastOccurrenceID
	i.LastOccurrenceTimestamp = dto.LastOccurrenceTimestamp
	i.Occurrences = dto.Occurrences
	i.TotalOccurrences = dto.TotalOccurrences

	return nil
}

type ItemInstance struct {
	ID        uint64          `json:"id"`
	Timestamp *uint64         `json:"timestamp"`
	Body      json.RawMessage `json:"body"`
	Data      json.RawMessage `json:"data"`
	Raw       json.RawMessage `json:"-"`
}

type itemByCounterResult struct {
	ID     domain.ItemID `json:"id"`
	ItemID domain.ItemID `json:"itemId"`
}

func (r itemByCounterResult) resolvedID() (domain.ItemID, error) {
	if r.ItemID != 0 {
		return r.ItemID, nil
	}

	if r.ID != 0 {
		return r.ID, nil
	}

	return 0, fmt.Errorf("item_by_counter result missing id")
}

type instancesEnvelope struct {
	Instances []ItemInstance `json:"instances"`
}

type itemsEnvelope struct {
	Items []Item `json:"items"`
}

type topActiveItem struct {
	Item Item `json:"item"`
}

type flexibleUint64 uint64

type flexibleLevel string

func (v *flexibleLevel) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*v = ""
		return nil
	}

	var levelString string
	if err := json.Unmarshal(data, &levelString); err == nil {
		*v = flexibleLevel(levelString)
		return nil
	}

	var levelNumber int
	if err := json.Unmarshal(data, &levelNumber); err != nil {
		return fmt.Errorf("decode level: %w", err)
	}

	switch levelNumber {
	case 10:
		*v = "debug"
	case 20:
		*v = "info"
	case 30:
		*v = "warning"
	case 40:
		*v = "error"
	case 50:
		*v = "critical"
	default:
		*v = flexibleLevel(strconv.Itoa(levelNumber))
	}

	return nil
}

func (v *flexibleUint64) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*v = 0
		return nil
	}

	if data[0] == '"' {
		parsed, err := strconv.ParseUint(string(data[1:len(data)-1]), 10, 64)
		if err != nil {
			return fmt.Errorf("parse string uint64: %w", err)
		}
		*v = flexibleUint64(parsed)
		return nil
	}

	var parsed uint64
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("decode uint64: %w", err)
	}

	*v = flexibleUint64(parsed)
	return nil
}
