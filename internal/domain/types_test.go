package domain

import "testing"

func TestParseItemCounter(t *testing.T) {
	t.Parallel()

	counter, err := ParseItemCounter("269")
	if err != nil {
		t.Fatalf("ParseItemCounter() error = %v", err)
	}

	if counter != ItemCounter(269) {
		t.Fatalf("ParseItemCounter() = %d", counter)
	}
}

func TestParseItemCounterInvalid(t *testing.T) {
	t.Parallel()

	if _, err := ParseItemCounter("invalid"); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestStringers(t *testing.T) {
	t.Parallel()

	if ItemCounter(12).String() != "12" {
		t.Fatalf("unexpected counter string")
	}

	if ItemID(99).String() != "99" {
		t.Fatalf("unexpected item id string")
	}
}
