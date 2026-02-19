package rollbar

import (
	"encoding/json"
	"fmt"

	"github.com/kevinsheth/rollbaz/internal/domain"
)

type Item struct {
	ID               domain.ItemID `json:"id"`
	ProjectID        uint64        `json:"project_id"`
	Counter          uint64        `json:"counter"`
	Title            string        `json:"title"`
	Status           string        `json:"status"`
	Environment      string        `json:"environment"`
	TotalOccurrences *uint64       `json:"total_occurrences"`
}

type ItemInstance struct {
	ID        uint64          `json:"id"`
	Timestamp *uint64         `json:"timestamp"`
	Body      json.RawMessage `json:"body"`
	Data      json.RawMessage `json:"data"`
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
