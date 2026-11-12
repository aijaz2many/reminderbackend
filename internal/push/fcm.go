package push

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type serviceAccount struct {
	ProjectID   string `json:"project_id"`
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

type Client struct {
	account     serviceAccount
	privateKey  *rsa.PrivateKey
	httpClient  *http.Client
	endpointURL string

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

func NewClient(credentials []byte) (*Client, error) {
	var account serviceAccount
	if err := json.Unmarshal(credentials, &account); err != nil {
		return nil, fmt.Errorf("decode Firebase service account credentials: %w", err)
	}
	if account.ProjectID == "" || account.ClientEmail == "" || account.PrivateKey == "" || account.TokenURI == "" {
		return nil, errors.New("Firebase service account credentials are missing required fields")
	}
	tokenURL, err := url.Parse(account.TokenURI)
	if err != nil || tokenURL.Scheme != "https" || tokenURL.Host == "" {
		return nil, errors.New("Firebase service account token_uri must be a valid HTTPS URL")
	}
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(account.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("parse Firebase service account private key: %w", err)
	}

	return &Client{
		account:     account,
		privateKey:  privateKey,
		httpClient:  &http.Client{Timeout: 15 * time.Second},
		endpointURL: "https://fcm.googleapis.com/v1/projects/" + url.PathEscape(account.ProjectID) + "/messages:send",
	}, nil
}

func (c *Client) Send(ctx context.Context, deviceToken, title, body string) (string, error) {
	if strings.TrimSpace(deviceToken) == "" {
		return "", errors.New("FCM device token is empty")
	}
	accessToken, err := c.getAccessToken(ctx)
	if err != nil {
		return "", err
	}
	payload := struct {
		Message struct {
			Token        string `json:"token"`
			Notification struct {
				Title string `json:"title"`
				Body  string `json:"body"`
			} `json:"notification"`
		} `json:"message"`
	}{}
	payload.Message.Token = deviceToken
	payload.Message.Notification.Title = title
	payload.Message.Notification.Body = body
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode FCM notification: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpointURL, strings.NewReader(string(jsonPayload)))
	if err != nil {
		return "", fmt.Errorf("create FCM request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("send FCM notification: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var result struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result)
		if result.Error.Message != "" {
			return "", fmt.Errorf("FCM rejected notification with HTTP %d: %s", response.StatusCode, result.Error.Message)
		}
		return "", fmt.Errorf("FCM rejected notification with HTTP %d", response.StatusCode)
	}
	var result struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return "", fmt.Errorf("decode FCM response: %w", err)
	}
	if result.Name == "" {
		return "", errors.New("FCM response did not include a message ID")
	}
	return result.Name, nil
}

func (c *Client) getAccessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken != "" && time.Until(c.tokenExpiry) > time.Minute {
		return c.accessToken, nil
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   c.account.ClientEmail,
		"scope": "https://www.googleapis.com/auth/firebase.messaging",
		"aud":   c.account.TokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	assertion, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(c.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign Firebase access token assertion: %w", err)
	}
	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.account.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create Firebase token request: %w", err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("request Firebase access token: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return "", fmt.Errorf("Firebase token endpoint returned HTTP %d", response.StatusCode)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return "", fmt.Errorf("decode Firebase access token response: %w", err)
	}
	if result.AccessToken == "" || result.ExpiresIn <= 0 {
		return "", errors.New("Firebase token endpoint returned an invalid access token")
	}
	c.accessToken = result.AccessToken
	c.tokenExpiry = now.Add(time.Duration(result.ExpiresIn) * time.Second)
	return c.accessToken, nil
}
