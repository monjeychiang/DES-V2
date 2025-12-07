package spot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// CreateListenKey creates a new user data stream listen key.
func (c *Client) CreateListenKey(ctx context.Context) (string, error) {
	if c.cfg.APIKey == "" {
		return "", errors.New("binance: API key required")
	}

	endpoint := c.baseURL + "/api/v3/userDataStream"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-MBX-APIKEY", c.cfg.APIKey)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("create listen key status %d", res.StatusCode)
	}

	var resp struct {
		ListenKey string `json:"listenKey"`
	}
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return "", err
	}
	return resp.ListenKey, nil
}

// KeepAliveListenKey extends the validity of a listen key.
func (c *Client) KeepAliveListenKey(ctx context.Context, listenKey string) error {
	if c.cfg.APIKey == "" {
		return errors.New("binance: API key required")
	}

	params := url.Values{}
	params.Set("listenKey", listenKey)
	endpoint := fmt.Sprintf("%s/api/v3/userDataStream?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-MBX-APIKEY", c.cfg.APIKey)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("keep alive listen key status %d", res.StatusCode)
	}
	return nil
}

// CloseListenKey closes a user data stream.
func (c *Client) CloseListenKey(ctx context.Context, listenKey string) error {
	if c.cfg.APIKey == "" {
		return errors.New("binance: API key required")
	}

	params := url.Values{}
	params.Set("listenKey", listenKey)
	endpoint := fmt.Sprintf("%s/api/v3/userDataStream?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-MBX-APIKEY", c.cfg.APIKey)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("close listen key status %d", res.StatusCode)
	}
	return nil
}
