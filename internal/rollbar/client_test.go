package rollbar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kevinsheth/rollbaz/internal/domain"
)

func TestResolveItemIDByCounterSupportsRedirectShape(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/1/item_by_counter/269" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, `{"err":0,"result":{"itemId":1755568172}}`)
	}))
	t.Cleanup(server.Close)

	client := newTestClient(t, server.URL+"/api/1")
	itemID, err := client.ResolveItemIDByCounter(context.Background(), domain.ItemCounter(269))
	if err != nil {
		t.Fatalf("ResolveItemIDByCounter() error = %v", err)
	}

	if itemID != domain.ItemID(1755568172) {
		t.Fatalf("ResolveItemIDByCounter() = %d", itemID)
	}
}

func TestResolveItemIDByCounterSupportsItemShape(t *testing.T) {
	t.Parallel()

	client := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"err":0,"result":{"id":99}}`)
	})
	itemID, err := client.ResolveItemIDByCounter(context.Background(), domain.ItemCounter(269))
	if err != nil {
		t.Fatalf("ResolveItemIDByCounter() error = %v", err)
	}

	if itemID != domain.ItemID(99) {
		t.Fatalf("ResolveItemIDByCounter() = %d", itemID)
	}
}

func TestResolveItemIDByCounterMissingID(t *testing.T) {
	t.Parallel()

	client := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"err":0,"result":{}}`)
	})
	_, err := client.ResolveItemIDByCounter(context.Background(), domain.ItemCounter(1))
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetItem(t *testing.T) {
	t.Parallel()

	client := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Rollbar-Access-Token") != "token" {
			t.Fatalf("missing access token header")
		}
		_, _ = fmt.Fprint(w, `{"err":0,"result":{"id":1755568172,"project_id":766510,"counter":269,"title":"RST_STREAM","status":"active","environment":"production","total_occurrences":7}}`)
	})
	item, err := client.GetItem(context.Background(), domain.ItemID(1755568172))
	if err != nil {
		t.Fatalf("GetItem() error = %v", err)
	}

	if item.Title != "RST_STREAM" {
		t.Fatalf("GetItem() title = %q", item.Title)
	}
}

func TestUpdateItem(t *testing.T) {
	t.Parallel()

	expiration := int64(3600)
	client := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/item/1755568172" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("missing content-type header")
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}

		var patch ItemPatch
		if err := json.Unmarshal(body, &patch); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if patch.Status != "muted" || patch.SnoozeExpirationInSeconds == nil || *patch.SnoozeExpirationInSeconds != expiration {
			t.Fatalf("unexpected patch payload: %+v", patch)
		}

		_, _ = fmt.Fprint(w, `{"err":0,"result":{}}`)
	})

	err := client.UpdateItem(context.Background(), domain.ItemID(1755568172), ItemPatch{Status: "muted", SnoozeExpirationInSeconds: &expiration})
	if err != nil {
		t.Fatalf("UpdateItem() error = %v", err)
	}
}

func TestUpdateItemEnvelopeError(t *testing.T) {
	t.Parallel()

	client := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"err":1,"message":"denied"}`)
	})
	err := client.UpdateItem(context.Background(), domain.ItemID(1), ItemPatch{Status: "resolved"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetLatestInstanceSupportsListAndWrapped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{
			name: "list shape",
			body: `{"err":0,"result":[{"id":1,"data":{"message":"x"}}]}`,
		},
		{
			name: "wrapped shape",
			body: `{"err":0,"result":{"instances":[{"id":2,"body":{"message":{"body":"y"}}}]}}`,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprint(w, tc.body)
			}))
			defer server.Close()

			client := newTestClient(t, server.URL)
			instance, err := client.GetLatestInstance(context.Background(), domain.ItemID(1))
			if err != nil {
				t.Fatalf("GetLatestInstance() error = %v", err)
			}

			if instance == nil {
				t.Fatalf("GetLatestInstance() returned nil instance")
			}
		})
	}
}

func TestGetResultReturnsEnvelopeError(t *testing.T) {
	t.Parallel()

	client := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"err":1,"message":"denied"}`)
	})
	_, err := client.GetItem(context.Background(), domain.ItemID(1))
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetItemNonSuccessStatus(t *testing.T) {
	t.Parallel()

	client := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = fmt.Fprint(w, `bad gateway`)
	})
	_, err := client.GetItem(context.Background(), domain.ItemID(1))
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetItemResponseTooLarge(t *testing.T) {
	t.Parallel()

	client := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		large := strings.Repeat("a", maxResponseBodyBytes+1024)
		_, _ = fmt.Fprintf(w, `{"err":0,"result":"%s"}`, large)
	})
	_, err := client.GetItem(context.Background(), domain.ItemID(1))
	if err == nil {
		t.Fatalf("expected response too large error")
	}
}

func TestGetLatestInstanceReturnsNilForEmptyList(t *testing.T) {
	t.Parallel()

	client := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"err":0,"result":[]}`)
	})
	instance, err := client.GetLatestInstance(context.Background(), domain.ItemID(1))
	if err != nil {
		t.Fatalf("GetLatestInstance() error = %v", err)
	}

	if instance != nil {
		t.Fatalf("expected nil instance")
	}
}

func TestGetItemMissingResult(t *testing.T) {
	t.Parallel()

	client := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"err":0,"result":null}`)
	})
	_, err := client.GetItem(context.Background(), domain.ItemID(1))
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestNewRejectsBlankToken(t *testing.T) {
	t.Parallel()

	if _, err := New("   "); err == nil {
		t.Fatalf("expected token validation error")
	}
}

func TestWrapRedactsToken(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, "https://api.rollbar.com/api/1")
	err := client.wrap(fmt.Errorf("https://x?access_token=token"), "operation")
	if strings.Contains(err.Error(), "access_token=token") {
		t.Fatalf("token should be redacted")
	}
}

func TestBuildURL(t *testing.T) {
	t.Parallel()

	got, err := buildURL("https://api.rollbar.com/api/1", "/item/1/instances?per_page=1")
	if err != nil {
		t.Fatalf("buildURL() error = %v", err)
	}

	want := "https://api.rollbar.com/api/1/item/1/instances?per_page=1"
	if got != want {
		t.Fatalf("buildURL() = %q, want %q", got, want)
	}
}

func TestBuildURLWithoutLeadingSlash(t *testing.T) {
	t.Parallel()

	got, err := buildURL("https://api.rollbar.com/api/1/", "item/1/")
	if err != nil {
		t.Fatalf("buildURL() error = %v", err)
	}

	want := "https://api.rollbar.com/api/1/item/1/"
	if got != want {
		t.Fatalf("buildURL() = %q, want %q", got, want)
	}
}

func TestBuildURLInvalidBase(t *testing.T) {
	t.Parallel()

	if _, err := buildURL("://bad", "/item/1/"); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestParseInstancesInvalid(t *testing.T) {
	t.Parallel()

	if _, err := parseInstances([]byte(`123`)); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestListActiveItems(t *testing.T) {
	t.Parallel()

	client := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/reports/top_active_items" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, `{"err":0,"result":[{"item":{"id":1,"counter":2,"title":"x","status":"active","environment":"prod","occurrences":3}}]}`)
	})
	items, err := client.ListActiveItems(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListActiveItems() error = %v", err)
	}
	if len(items) != 1 || items[0].Counter != 2 {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestListActiveItemsSupportsStringItemID(t *testing.T) {
	t.Parallel()

	client := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"err":0,"result":[{"item":{"id":"1","counter":2,"title":"x","status":"active","environment":"prod","occurrences":3}}]}`)
	})
	items, err := client.ListActiveItems(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListActiveItems() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != 1 {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestListActiveItemsFallbackToItemsParse(t *testing.T) {
	t.Parallel()

	client := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"err":0,"result":{"items":[{"id":1,"counter":3,"title":"x"}]}}`)
	})
	items, err := client.ListActiveItems(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListActiveItems() error = %v", err)
	}
	if len(items) != 1 || items[0].Counter != 3 {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestListItemsWrapped(t *testing.T) {
	t.Parallel()

	client := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/items" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, `{"err":0,"result":{"items":[{"id":1,"counter":2,"title":"x"}]}}`)
	})
	items, err := client.ListItems(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	if len(items) != 1 || items[0].Counter != 2 {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestListItemsSupportsStringID(t *testing.T) {
	t.Parallel()

	client := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"err":0,"result":{"items":[{"id":"1","counter":2,"title":"x"}]}}`)
	})
	items, err := client.ListItems(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != 1 {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestListItemsStatusAndPageQuery(t *testing.T) {
	t.Parallel()

	client := newTestClientWithHandler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "status=active&page=2" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		_, _ = fmt.Fprint(w, `{"err":0,"result":[]}`)
	})
	if _, err := client.ListItems(context.Background(), "active", 2); err != nil {
		t.Fatalf("ListItems() error = %v", err)
	}
}

func TestParseItemsInvalid(t *testing.T) {
	t.Parallel()

	if _, err := parseItems([]byte(`123`)); err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestTrimItems(t *testing.T) {
	t.Parallel()

	items := []Item{{Counter: 1}, {Counter: 2}, {Counter: 3}}
	trimmed := trimItems(items, 2)
	if len(trimmed) != 2 {
		t.Fatalf("trimItems() len = %d", len(trimmed))
	}
}

func TestWrapNil(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, "https://api.rollbar.com/api/1")
	if client.wrap(nil, "operation") != nil {
		t.Fatalf("wrap(nil) should be nil")
	}
}

func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()

	client, err := NewWithBaseURL("token", baseURL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return client
}

func newTestClientWithHandler(t *testing.T, handler func(http.ResponseWriter, *http.Request)) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(server.Close)

	return newTestClient(t, server.URL)
}
