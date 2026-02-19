package domain

import (
	"fmt"
	"strconv"
)

type ItemCounter uint64

func ParseItemCounter(value string) (ItemCounter, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse item counter: %w", err)
	}

	return ItemCounter(parsed), nil
}

func (c ItemCounter) String() string {
	return strconv.FormatUint(uint64(c), 10)
}

type ItemID uint64

func (id ItemID) String() string {
	return strconv.FormatUint(uint64(id), 10)
}
