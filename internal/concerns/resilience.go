// Copyright 2024 Alexandre Mahdhaoui
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package concerns

// Resilience emitters.
//
// Timeouts and retries and a circuit breaker, driven by spec.yaml so all four
// languages behave the same under failure.
//
// The rules every language follows.
//
//   - Retry only on a retryable outcome. 5xx and 429 and a transport error.
//     Never retry a 4xx other than 429. Retrying a 400 just burns budget.
//   - Backoff is deterministic: base * 2^attempt, capped. No jitter, because
//     jitter cannot be compared across languages.
//   - The breaker opens after N consecutive failures and rejects immediately
//     until a cooldown passes.
//
// A conformance test drives these rules with a fault injecting server.

var goResTmpl = mustTmpl("gores", `{{.Header}}
package {{.Package}}

import (
	"fmt"
	"time"
)

// Policy is the resilience policy for a client.
type Policy struct {
	Timeout        time.Duration
	MaxAttempts    int
	BackoffBase    time.Duration
	BackoffMax     time.Duration
	BreakerFailures int
	BreakerCooldown time.Duration
}

// ErrBreakerOpen is returned while the breaker is open.
var ErrBreakerOpen = fmt.Errorf("circuit breaker is open")

// Retryable reports whether an outcome deserves another attempt.
//
// status 0 means a transport error, which is always retryable.
func Retryable(status int) bool {
	switch {
	case status == 0:
		return true
	case status == 429:
		return true
	case status >= 500:
		return true
	default:
		return false
	}
}

// BackoffFor returns the wait before the given zero based attempt.
//
// Deterministic on purpose. Jitter cannot be compared across languages.
func BackoffFor(p Policy, attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	d := p.BackoffBase
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= p.BackoffMax {
			return p.BackoffMax
		}
	}

	if d > p.BackoffMax {
		return p.BackoffMax
	}

	return d
}

// Breaker counts consecutive failures and opens after the threshold.
type Breaker struct {
	policy   Policy
	failures int
	openedAt time.Time
}

// NewBreaker builds a breaker for a policy.
func NewBreaker(p Policy) *Breaker { return &Breaker{policy: p} }

// Allow reports whether a call may proceed at time now.
func (b *Breaker) Allow(now time.Time) bool {
	if b.failures < b.policy.BreakerFailures {
		return true
	}

	return now.Sub(b.openedAt) >= b.policy.BreakerCooldown
}

// Record updates the breaker with the outcome of a call.
func (b *Breaker) Record(now time.Time, ok bool) {
	if ok {
		b.failures = 0

		return
	}

	b.failures++
	if b.failures == b.policy.BreakerFailures {
		b.openedAt = now
	}
}

// Failures reports the consecutive failure count.
func (b *Breaker) Failures() int { return b.failures }
`)

var pyResTmpl = mustTmpl("pyres", `{{.Header}}
"""Resilience policy generated from spec.yaml.

Timeouts and retries and a circuit breaker. Durations are milliseconds.
"""

from __future__ import annotations

from dataclasses import dataclass


class BreakerOpenError(Exception):
    """Raised while the breaker is open."""


@dataclass(frozen=True)
class Policy:
    """Resilience policy for a client."""

    timeout_ms: int
    max_attempts: int
    backoff_base_ms: int
    backoff_max_ms: int
    breaker_failures: int
    breaker_cooldown_ms: int


def retryable(status: int) -> bool:
    """Report whether an outcome deserves another attempt.

    status 0 means a transport error which is always retryable.
    """
    if status == 0:
        return True
    if status == 429:
        return True

    return status >= 500


def backoff_for(p: Policy, attempt: int) -> int:
    """Return the wait in milliseconds before a zero based attempt."""
    if attempt <= 0:
        return 0

    d = p.backoff_base_ms
    for _ in range(1, attempt):
        d *= 2
        if d >= p.backoff_max_ms:
            return p.backoff_max_ms

    return min(d, p.backoff_max_ms)


class Breaker:
    """Counts consecutive failures and opens after the threshold."""

    def __init__(self, policy: Policy) -> None:
        self._policy = policy
        self._failures = 0
        self._opened_at_ms = 0

    def allow(self, now_ms: int) -> bool:
        """Report whether a call may proceed."""
        if self._failures < self._policy.breaker_failures:
            return True

        return now_ms - self._opened_at_ms >= self._policy.breaker_cooldown_ms

    def record(self, now_ms: int, ok: bool) -> None:
        """Update the breaker with the outcome of a call."""
        if ok:
            self._failures = 0
            return

        self._failures += 1
        if self._failures == self._policy.breaker_failures:
            self._opened_at_ms = now_ms

    @property
    def failures(self) -> int:
        """Consecutive failure count."""
        return self._failures
`)

var tsResTmpl = mustTmpl("tsres", `{{.Header}}
// Resilience policy. Timeouts and retries and a circuit breaker.
// Durations are milliseconds.

export class BreakerOpenError extends Error {}

export type Policy = {
  readonly timeoutMs: number;
  readonly maxAttempts: number;
  readonly backoffBaseMs: number;
  readonly backoffMaxMs: number;
  readonly breakerFailures: number;
  readonly breakerCooldownMs: number;
};

/**
 * Report whether an outcome deserves another attempt.
 *
 * status 0 means a transport error which is always retryable.
 */
export function retryable(status: number): boolean {
  if (status === 0) return true;
  if (status === 429) return true;

  return status >= 500;
}

/** Return the wait in milliseconds before a zero based attempt. */
export function backoffFor(p: Policy, attempt: number): number {
  if (attempt <= 0) return 0;

  let d = p.backoffBaseMs;
  for (let i = 1; i < attempt; i += 1) {
    d *= 2;
    if (d >= p.backoffMaxMs) return p.backoffMaxMs;
  }

  return Math.min(d, p.backoffMaxMs);
}

/** Counts consecutive failures and opens after the threshold. */
export class Breaker {
  private failureCount = 0;
  private openedAtMs = 0;

  constructor(private readonly policy: Policy) {}

  allow(nowMs: number): boolean {
    if (this.failureCount < this.policy.breakerFailures) return true;

    return nowMs - this.openedAtMs >= this.policy.breakerCooldownMs;
  }

  record(nowMs: number, ok: boolean): void {
    if (ok) {
      this.failureCount = 0;

      return;
    }

    this.failureCount += 1;
    if (this.failureCount === this.policy.breakerFailures) {
      this.openedAtMs = nowMs;
    }
  }

  get failures(): number {
    return this.failureCount;
  }
}
`)

var rsResTmpl = mustTmpl("rsres", `{{.Header}}
//! Resilience policy generated from spec.yaml.
//!
//! Timeouts and retries and a circuit breaker. Durations are milliseconds.

#![allow(dead_code)]

/// Resilience policy for a client.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Policy {
    pub timeout_ms: i64,
    pub max_attempts: i64,
    pub backoff_base_ms: i64,
    pub backoff_max_ms: i64,
    pub breaker_failures: i64,
    pub breaker_cooldown_ms: i64,
}

/// Report whether an outcome deserves another attempt.
///
/// status 0 means a transport error which is always retryable.
pub fn retryable(status: i64) -> bool {
    if status == 0 || status == 429 {
        return true;
    }

    status >= 500
}

/// Return the wait in milliseconds before a zero based attempt.
pub fn backoff_for(p: &Policy, attempt: i64) -> i64 {
    if attempt <= 0 {
        return 0;
    }

    let mut d = p.backoff_base_ms;
    let mut i = 1;
    while i < attempt {
        d *= 2;
        if d >= p.backoff_max_ms {
            return p.backoff_max_ms;
        }
        i += 1;
    }

    d.min(p.backoff_max_ms)
}

/// Counts consecutive failures and opens after the threshold.
pub struct Breaker {
    policy: Policy,
    failures: i64,
    opened_at_ms: i64,
}

impl Breaker {
    pub fn new(policy: Policy) -> Self {
        Self {
            policy,
            failures: 0,
            opened_at_ms: 0,
        }
    }

    /// Report whether a call may proceed.
    pub fn allow(&self, now_ms: i64) -> bool {
        if self.failures < self.policy.breaker_failures {
            return true;
        }

        now_ms - self.opened_at_ms >= self.policy.breaker_cooldown_ms
    }

    /// Update the breaker with the outcome of a call.
    pub fn record(&mut self, now_ms: i64, ok: bool) {
        if ok {
            self.failures = 0;
            return;
        }

        self.failures += 1;
        if self.failures == self.policy.breaker_failures {
            self.opened_at_ms = now_ms;
        }
    }

    pub fn failures(&self) -> i64 {
        self.failures
    }
}
`)
