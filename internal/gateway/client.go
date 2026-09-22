package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	VercelEndpoint     = "https://ai-gateway.vercel.sh/typesafe/v1/systemone"
	TypeSafeEndpoint   = "https://api.typesafe.ai/v1/systemone"
	OpenRouterEndpoint = "https://openrouter.ai/api/v1/alpha/decisions"
	DefaultModel       = "jev-latest"
)

type ErrorKind string

const (
	ErrorAuth        ErrorKind = "auth"
	ErrorBilling     ErrorKind = "billing"
	ErrorRateLimit   ErrorKind = "rate_limit"
	ErrorTimeout     ErrorKind = "timeout"
	ErrorUnavailable ErrorKind = "unavailable"
)

type ProviderError struct {
	Kind    ErrorKind
	Status  int
	Message string
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s provider error (%d): %s", e.Kind, e.Status, e.Message)
}

type Question struct {
	Type         string `json:"type"`
	Instructions any    `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

type Request struct {
	State     any                 `json:"state"`
	Model     string              `json:"model"`
	Questions map[string]Question `json:"questions"`
}

type Answer struct {
	Type          string             `json:"type"`
	Noul          float64            `json:"noul,omitempty"`
	Choice        string             `json:"choice,omitempty"`
	Score         float64            `json:"score,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Confidence    float64            `json:"confidence,omitempty"`
}

type Response struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type Client struct {
	Provider   string
	APIKey     string
	Endpoint   string
	Model      string
	HTTPClient *http.Client
}

func (c Client) Available() bool { return strings.TrimSpace(c.APIKey) != "" }

func (c Client) Evaluate(ctx context.Context, state any, questions map[string]Question) (Response, error) {
	if !c.Available() {
		return Response{}, fmt.Errorf("decision provider API key is not configured")
	}
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = VercelEndpoint
	}
	modelName := c.Model
	if modelName == "" {
		modelName = DefaultModel
	}
	payload, err := json.Marshal(Request{State: state, Model: modelName, Questions: questions})
	if err != nil {
		return Response{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "whichrepo/0.1")
	if c.Provider == "jev-openrouter" {
		req.Header.Set("HTTP-Referer", "https://github.com/smallshellctw/whichrepo")
		req.Header.Set("X-Title", "WhichRepo")
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Response{}, &ProviderError{Kind: ErrorTimeout, Message: ctx.Err().Error()}
		}
		return Response{}, &ProviderError{Kind: ErrorUnavailable, Message: err.Error()}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return Response{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		kind := ErrorUnavailable
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			kind = ErrorAuth
			if bytes.Contains(bytes.ToLower(body), []byte("credit")) || bytes.Contains(bytes.ToLower(body), []byte("billing")) {
				kind = ErrorBilling
			}
		case http.StatusPaymentRequired:
			kind = ErrorBilling
		case http.StatusTooManyRequests:
			kind = ErrorRateLimit
		case http.StatusRequestTimeout, http.StatusGatewayTimeout:
			kind = ErrorTimeout
		}
		return Response{}, &ProviderError{Kind: kind, Status: resp.StatusCode, Message: strings.TrimSpace(string(body))}
	}
	var result Response
	if err := json.Unmarshal(body, &result); err != nil {
		return Response{}, fmt.Errorf("decode Vercel AI Gateway response: %w", err)
	}
	if len(result.Answers) == 0 {
		return Response{}, fmt.Errorf("Vercel AI Gateway returned no answers")
	}
	return result, nil
}
