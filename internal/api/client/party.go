package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type PubKey struct {
	X string `json:"x"`
	Y string `json:"y"`
}

type SignatureData struct {
	R         string `json:"r"`
	S         string `json:"s"`
	Signature string `json:"signature"`
}

type PartyClient struct {
	baseURL string
	http    *http.Client
}

func NewPartyClient(baseURL string) *PartyClient {
	return &PartyClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *PartyClient) InitKeygen(ctx context.Context, userID string) error {
	body, err := json.Marshal(map[string]string{"user_id": userID})
	if err != nil {
		return fmt.Errorf("marshal keygen request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/keygen", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build keygen request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("keygen to %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("party %s keygen: unexpected status %d", c.baseURL, resp.StatusCode)
	}
	return nil
}

func (c *PartyClient) Sign(ctx context.Context, message, userID string) (*SignatureData, error) {
	body, err := json.Marshal(map[string]string{"message": message, "user_id": userID})
	if err != nil {
		return nil, fmt.Errorf("marshal sign request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sign",
		bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build sign request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sign to %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("party %s sign: unexpected status %d", c.baseURL, resp.StatusCode)
	}

	var sig SignatureData
	if err := json.NewDecoder(resp.Body).Decode(&sig); err != nil {
		return nil, fmt.Errorf("decode sign response from %s: %w", c.baseURL, err)
	}
	return &sig, nil
}

func (c *PartyClient) GetPubKey(ctx context.Context, userID string) (*PubKey, error) {
	params := url.Values{"user_id": {userID}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/pubkey?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build pubkey request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pubkey from %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("party %s pubkey: unexpected status %d", c.baseURL, resp.StatusCode)
	}

	var pubKey PubKey
	if err := json.NewDecoder(resp.Body).Decode(&pubKey); err != nil {
		return nil, fmt.Errorf("decode pubkey from %s: %w", c.baseURL, err)
	}
	return &pubKey, nil
}
