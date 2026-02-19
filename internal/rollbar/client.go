package rollbar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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

	return item, nil
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

	return &instances[len(instances)-1], nil
}

func parseInstances(raw json.RawMessage) ([]ItemInstance, error) {
	var list []ItemInstance
	if err := json.Unmarshal(raw, &list); err == nil {
		return list, nil
	}

	var wrapped instancesEnvelope
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, fmt.Errorf("decode wrapped instances: %w", err)
	}

	return wrapped.Instances, nil
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
	requestURL, err := buildURL(c.baseURL, endpointPath)
	if err != nil {
		return nil, c.wrap(err, "build "+op+" URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, c.wrap(err, "build "+op+" request")
	}

	req.Header.Set("X-Rollbar-Access-Token", c.accessToken)

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

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, c.wrap(err, "read "+op+" response")
	}

	return body, nil
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
