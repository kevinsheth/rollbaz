package rollbar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kevinsheth/rollbaz/internal/domain"
	"github.com/kevinsheth/rollbaz/internal/redact"
)

const defaultBaseURL = "https://api.rollbar.com/api/1"

type Client struct {
	http        *http.Client
	baseURL     string
	accessToken string
}

type apiEnvelope struct {
	Err     int             `json:"err"`
	Result  json.RawMessage `json:"result"`
	Message string          `json:"message"`
}

func New(accessToken string) (*Client, error) {
	return NewWithBaseURL(accessToken, defaultBaseURL)
}

func NewWithBaseURL(accessToken string, baseURL string) (*Client, error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, errors.New("rollbar access token is required")
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	httpClient := &http.Client{
		Timeout:   8 * time.Second,
		Transport: transport,
	}

	return &Client{
		http:        httpClient,
		baseURL:     baseURL,
		accessToken: accessToken,
	}, nil
}

func (c *Client) ResolveItemIDByCounter(ctx context.Context, counter domain.ItemCounter) (domain.ItemID, error) {
	raw, err := c.getResult(ctx, "/item_by_counter/"+counter.String(), "item_by_counter")
	if err != nil {
		return 0, err
	}

	var result itemByCounterResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return 0, c.wrap(err, "decode item_by_counter result")
	}

	resolvedID, err := result.resolvedID()
	if err != nil {
		return 0, c.wrap(err, "resolve item_id")
	}

	return resolvedID, nil
}

func (c *Client) GetItem(ctx context.Context, itemID domain.ItemID) (Item, error) {
	raw, err := c.getResult(ctx, "/item/"+itemID.String()+"/", "item")
	if err != nil {
		return Item{}, err
	}

	var item Item
	if err := json.Unmarshal(raw, &item); err != nil {
		return Item{}, c.wrap(err, "decode item response")
	}
	item.Raw = append(json.RawMessage(nil), raw...)

	return item, nil
}

func (c *Client) UpdateItem(ctx context.Context, itemID domain.ItemID, patch ItemPatch) error {
	body, err := json.Marshal(patch)
	if err != nil {
		return c.wrap(err, "encode update item request")
	}

	raw, err := c.doPatch(ctx, "/item/"+itemID.String(), body, "update item")
	if err != nil {
		return err
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return c.wrap(err, "decode update item envelope")
	}
	if envelope.Err != 0 {
		message := envelope.Message
		if message == "" {
			message = "unknown error from Rollbar"
		}
		return c.wrap(errors.New(message), "rollbar update item")
	}

	return nil
}

func (c *Client) ListActiveItems(ctx context.Context, limit int) ([]Item, error) {
	raw, err := c.getResult(ctx, "/reports/top_active_items", "top active items")
	if err != nil {
		return nil, err
	}

	var wrapped []topActiveItem
	if err := json.Unmarshal(raw, &wrapped); err == nil {
		items := make([]Item, 0, len(wrapped))
		for _, entry := range wrapped {
			items = append(items, hydrateItem(entry.Item, "active"))
		}
		return trimItems(items, limit), nil
	}

	items, err := parseItems(raw)
	if err != nil {
		return nil, c.wrap(err, "decode top active items")
	}

	return trimItems(items, limit), nil
}

func (c *Client) ListItems(ctx context.Context, status string, page int) ([]Item, error) {
	query := "/items"
	params := make([]string, 0, 2)
	if status != "" {
		params = append(params, "status="+url.QueryEscape(status))
	}
	if page > 0 {
		params = append(params, "page="+strconv.Itoa(page))
	}
	if len(params) > 0 {
		query += "?" + strings.Join(params, "&")
	}

	raw, err := c.getResult(ctx, query, "items")
	if err != nil {
		return nil, err
	}

	items, err := parseItems(raw)
	if err != nil {
		return nil, c.wrap(err, "decode items response")
	}

	return items, nil
}

func (c *Client) GetLatestInstance(ctx context.Context, itemID domain.ItemID) (*ItemInstance, error) {
	raw, err := c.getResult(ctx, "/item/"+itemID.String()+"/instances?per_page=1", "item instances")
	if err != nil {
		return nil, err
	}

	instances, err := parseInstances(raw)
	if err != nil {
		return nil, c.wrap(err, "decode instances response")
	}

	if len(instances) == 0 {
		return nil, nil
	}

	last := instances[len(instances)-1]
	return &last, nil
}

func parseInstances(raw json.RawMessage) ([]ItemInstance, error) {
	var list []ItemInstance
	if err := json.Unmarshal(raw, &list); err == nil {
		for index := range list {
			list[index] = hydrateInstance(list[index])
		}
		return list, nil
	}

	var wrapped instancesEnvelope
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, fmt.Errorf("decode wrapped instances: %w", err)
	}
	for index := range wrapped.Instances {
		wrapped.Instances[index] = hydrateInstance(wrapped.Instances[index])
	}

	return wrapped.Instances, nil
}

func parseItems(raw json.RawMessage) ([]Item, error) {
	var list []Item
	if err := json.Unmarshal(raw, &list); err == nil {
		for index := range list {
			list[index] = hydrateItem(list[index], "")
		}
		return list, nil
	}

	var wrapped itemsEnvelope
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, fmt.Errorf("decode wrapped items: %w", err)
	}
	for index := range wrapped.Items {
		wrapped.Items[index] = hydrateItem(wrapped.Items[index], "")
	}

	return wrapped.Items, nil
}

func trimItems(items []Item, limit int) []Item {
	if limit <= 0 || len(items) <= limit {
		return items
	}

	return items[:limit]
}

func hydrateItem(item Item, defaultStatus string) Item {
	if defaultStatus != "" && item.Status == "" {
		item.Status = defaultStatus
	}
	if item.TotalOccurrences == nil {
		item.TotalOccurrences = item.Occurrences
	}

	encoded, err := json.Marshal(item)
	if err == nil {
		item.Raw = encoded
	}

	return item
}

func hydrateInstance(instance ItemInstance) ItemInstance {
	encoded, err := json.Marshal(instance)
	if err == nil {
		instance.Raw = encoded
	}

	return instance
}

func (c *Client) getResult(ctx context.Context, endpointPath string, op string) (json.RawMessage, error) {
	body, err := c.doGet(ctx, endpointPath, op)
	if err != nil {
		return nil, err
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, c.wrap(err, "decode "+op+" envelope")
	}

	if envelope.Err != 0 {
		message := envelope.Message
		if message == "" {
			message = "unknown error from Rollbar"
		}
		return nil, c.wrap(errors.New(message), "rollbar "+op)
	}

	if len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return nil, c.wrap(errors.New("missing result"), op+" response")
	}

	return envelope.Result, nil
}

func (c *Client) doGet(ctx context.Context, endpointPath string, op string) ([]byte, error) {
	return c.doRequest(ctx, http.MethodGet, endpointPath, nil, "", op)
}

func (c *Client) doPatch(ctx context.Context, endpointPath string, body []byte, op string) ([]byte, error) {
	return c.doRequest(ctx, http.MethodPatch, endpointPath, bytes.NewReader(body), "application/json", op)
}

func (c *Client) doRequest(ctx context.Context, method string, endpointPath string, requestBody io.Reader, contentType string, op string) ([]byte, error) {
	requestURL, err := buildURL(c.baseURL, endpointPath)
	if err != nil {
		return nil, c.wrap(err, "build "+op+" URL")
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, requestBody)
	if err != nil {
		return nil, c.wrap(err, "build "+op+" request")
	}

	req.Header.Set("X-Rollbar-Access-Token", c.accessToken)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	response, err := c.http.Do(req)
	if err != nil {
		return nil, c.wrap(err, "request "+op)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		limited, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return nil, c.wrap(fmt.Errorf("status %d: %s", response.StatusCode, strings.TrimSpace(string(limited))), op+" returned non-success status")
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, c.wrap(err, "read "+op+" response")
	}

	return responseBody, nil
}

func buildURL(baseURL string, endpointPath string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}

	query := ""

	parts := strings.SplitN(endpointPath, "?", 2)
	rawPath := parts[0]
	if len(parts) == 2 {
		query = parts[1]
	}

	parsed.Path = joinPathPreserveTrailing(parsed.Path, rawPath)
	parsed.RawQuery = query
	return parsed.String(), nil
}

func joinPathPreserveTrailing(base string, suffix string) string {
	trimmedBase := strings.TrimRight(base, "/")
	if strings.HasPrefix(suffix, "/") {
		return trimmedBase + suffix
	}

	return trimmedBase + "/" + suffix
}

func (c *Client) wrap(err error, operation string) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %s", operation, redact.String(err.Error(), c.accessToken))
}
