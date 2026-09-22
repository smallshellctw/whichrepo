package webhooks

// RetryPolicy controls checkout webhook retries and backoff.
type RetryPolicy struct {
	MaxAttempts int
	BackoffMS   int
}
