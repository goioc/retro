// Package retro retries functions with configurable backoff strategies for
// individual errors or all errors. It supports retry limits, jitter, capped
// delays, and context cancellation. Configured callers can be reused and called
// concurrently.
package retro
