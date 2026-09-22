export interface RetryPolicy {
  maxAttempts: number;
  backoffMs: number;
}
