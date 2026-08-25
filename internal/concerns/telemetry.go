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

import "text/template"

// mustTmpl parses a template or panics at init.
func mustTmpl(name, body string) *template.Template {
	return template.Must(template.New(name).Parse(body))
}

// Telemetry emitters: metrics and tracing.
//
// Metrics render in the Prometheus text exposition format. Every language must
// produce byte identical output for the same observations. That means fixed
// bucket bounds, sorted metric names, sorted labels, and one agreed float
// format. Float formatting is the usual divergence: Go prints 0.5, some
// languages print 0.50 or 5e-1.
//
// Tracing follows W3C Trace Context. A traceparent is
// 00-<32 hex trace id>-<16 hex span id>-<2 hex flags>. A server that receives
// one must keep the trace id and start a new span id. That is what makes a
// trace join across a Go client and a Rust server.

var goTelTmpl = mustTmpl("gotel", `{{.Header}}
package {{.Package}}

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Buckets are the fixed histogram bounds. Fixed so four languages agree.
var Buckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

// FormatFloat renders a float the one agreed way.
//
// Shortest round trip form with no exponent below 1e21. Every language is
// forced to this shape or the exposition text differs.
func FormatFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if s == "-0" {
		return "0"
	}

	return s
}

// Registry holds counters and histograms.
type Registry struct {
	mu         sync.Mutex
	counters   map[string]float64
	histograms map[string][]float64
	help       map[string]string
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		counters:   map[string]float64{},
		histograms: map[string][]float64{},
		help:       map[string]string{},
	}
}

// key renders a metric name with sorted labels.
func key(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}

	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%q", k, labels[k]))
	}

	return name + "{" + strings.Join(parts, ",") + "}"
}

// Inc adds to a counter.
func (r *Registry) Inc(name string, labels map[string]string, by float64, help string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.counters[key(name, labels)] += by
	r.help[name] = help
}

// Observe records a histogram sample.
func (r *Registry) Observe(name string, labels map[string]string, v float64, help string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	k := key(name, labels)
	r.histograms[k] = append(r.histograms[k], v)
	r.help[name] = help
}

// baseName strips the label set from a rendered key.
func baseName(k string) string {
	if i := strings.Index(k, "{"); i >= 0 {
		return k[:i]
	}

	return k
}

// withLabel injects an extra label into a rendered key.
func withLabel(k, label, value string) string {
	base := baseName(k)
	inner := ""
	if i := strings.Index(k, "{"); i >= 0 {
		inner = k[i+1 : len(k)-1]
	}

	pair := fmt.Sprintf("%s=%q", label, value)
	if inner == "" {
		return base + "{" + pair + "}"
	}

	parts := append(strings.Split(inner, ","), pair)
	sort.Strings(parts)

	return base + "{" + strings.Join(parts, ",") + "}"
}

// Render produces the Prometheus text exposition format.
//
// Metric families are sorted so two processes emit identical bytes.
func (r *Registry) Render() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var b strings.Builder

	names := make([]string, 0, len(r.counters))
	for k := range r.counters {
		names = append(names, k)
	}
	sort.Strings(names)

	seen := map[string]bool{}
	for _, k := range names {
		base := baseName(k)
		if !seen[base] {
			fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s counter\n", base, r.help[base], base)
			seen[base] = true
		}
		fmt.Fprintf(&b, "%s %s\n", k, FormatFloat(r.counters[k]))
	}

	hnames := make([]string, 0, len(r.histograms))
	for k := range r.histograms {
		hnames = append(hnames, k)
	}
	sort.Strings(hnames)

	for _, k := range hnames {
		base := baseName(k)
		if !seen[base] {
			fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s histogram\n", base, r.help[base], base)
			seen[base] = true
		}

		samples := r.histograms[k]
		sum := 0.0
		for _, s := range samples {
			sum += s
		}

		cumulative := 0
		for _, bound := range Buckets {
			for _, s := range samples {
				if s <= bound {
					cumulative++
				}
			}
			fmt.Fprintf(&b, "%s %d\n", withLabel(k, "le", FormatFloat(bound)), cumulative)
			cumulative = 0
		}
		fmt.Fprintf(&b, "%s %d\n", withLabel(k, "le", "+Inf"), len(samples))
		fmt.Fprintf(&b, "%s_sum %s\n", baseName(k), FormatFloat(sum))
		fmt.Fprintf(&b, "%s_count %d\n", baseName(k), len(samples))
	}

	return b.String()
}
`)

var goTraceTmpl = mustTmpl("gotrace", `{{.Header}}
package {{.Package}}

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// TraceParent is a parsed W3C traceparent header.
type TraceParent struct {
	TraceID string
	SpanID  string
	Flags   string
}

// Header renders the value for the traceparent HTTP header.
func (t TraceParent) Header() string {
	return fmt.Sprintf("00-%s-%s-%s", t.TraceID, t.SpanID, t.Flags)
}

// NewTrace starts a brand new trace.
func NewTrace() TraceParent {
	return TraceParent{TraceID: randHex(16), SpanID: randHex(8), Flags: "01"}
}

// Child keeps the trace id and starts a new span id.
//
// Keeping the trace id is what makes a trace join across languages.
func (t TraceParent) Child() TraceParent {
	return TraceParent{TraceID: t.TraceID, SpanID: randHex(8), Flags: t.Flags}
}

// ParseTraceParent reads a traceparent header.
//
// An unparseable header starts a new trace rather than failing the request.
// A dropped trace is better than a dropped request.
func ParseTraceParent(s string) (TraceParent, bool) {
	parts := strings.Split(strings.TrimSpace(s), "-")
	if len(parts) != 4 || parts[0] != "00" {
		return TraceParent{}, false
	}
	if len(parts[1]) != 32 || len(parts[2]) != 16 || len(parts[3]) != 2 {
		return TraceParent{}, false
	}
	if !isHex(parts[1]) || !isHex(parts[2]) || !isHex(parts[3]) {
		return TraceParent{}, false
	}
	if strings.Trim(parts[1], "0") == "" || strings.Trim(parts[2], "0") == "" {
		return TraceParent{}, false
	}

	return TraceParent{TraceID: parts[1], SpanID: parts[2], Flags: parts[3]}, true
}

func isHex(s string) bool {
	_, err := hex.DecodeString(s)

	return err == nil
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = 1
		}
	}

	return hex.EncodeToString(b)
}
`)

var pyTelTmpl = mustTmpl("pytel", `{{.Header}}
"""Metrics and W3C trace context generated from spec.yaml."""

from __future__ import annotations

import secrets
from collections import defaultdict

BUCKETS = [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]


def format_float(f: float) -> str:
    """Render a float the one agreed way.

    Shortest round trip form with no exponent and no trailing zeros.
    """
    if f == int(f) and abs(f) < 1e16:
        return str(int(f))

    s = repr(float(f))
    if "e" in s or "E" in s:
        s = f"{f:.17f}".rstrip("0").rstrip(".")

    return s


def _key(name: str, labels: dict[str, str] | None) -> str:
    if not labels:
        return name

    parts = [f'{k}="{labels[k]}"' for k in sorted(labels)]

    return name + "{" + ",".join(parts) + "}"


def _base_name(k: str) -> str:
    return k.split("{", 1)[0]


def _with_label(k: str, label: str, value: str) -> str:
    base = _base_name(k)
    inner = k[len(base) + 1 : -1] if "{" in k else ""
    pair = f'{label}="{value}"'
    if not inner:
        return base + "{" + pair + "}"

    parts = sorted([*inner.split(","), pair])

    return base + "{" + ",".join(parts) + "}"


class Registry:
    """Holds counters and histograms."""

    def __init__(self) -> None:
        self._counters: dict[str, float] = defaultdict(float)
        self._histograms: dict[str, list[float]] = defaultdict(list)
        self._help: dict[str, str] = {}

    def inc(self, name: str, labels: dict[str, str] | None, by: float, help: str) -> None:
        """Add to a counter."""
        self._counters[_key(name, labels)] += by
        self._help[name] = help

    def observe(self, name: str, labels: dict[str, str] | None, v: float, help: str) -> None:
        """Record a histogram sample."""
        self._histograms[_key(name, labels)].append(v)
        self._help[name] = help

    def render(self) -> str:
        """Produce the Prometheus text exposition format."""
        out: list[str] = []
        seen: set[str] = set()

        for k in sorted(self._counters):
            base = _base_name(k)
            if base not in seen:
                out.append(f"# HELP {base} {self._help.get(base, '')}")
                out.append(f"# TYPE {base} counter")
                seen.add(base)
            out.append(f"{k} {format_float(self._counters[k])}")

        for k in sorted(self._histograms):
            base = _base_name(k)
            if base not in seen:
                out.append(f"# HELP {base} {self._help.get(base, '')}")
                out.append(f"# TYPE {base} histogram")
                seen.add(base)

            samples = self._histograms[k]
            for bound in BUCKETS:
                count = sum(1 for s in samples if s <= bound)
                out.append(f"{_with_label(k, 'le', format_float(bound))} {count}")
            out.append(f"{_with_label(k, 'le', '+Inf')} {len(samples)}")
            out.append(f"{base}_sum {format_float(sum(samples))}")
            out.append(f"{base}_count {len(samples)}")

        return "\n".join(out) + ("\n" if out else "")


class TraceParent:
    """A parsed W3C traceparent header."""

    def __init__(self, trace_id: str, span_id: str, flags: str) -> None:
        self.trace_id = trace_id
        self.span_id = span_id
        self.flags = flags

    def header(self) -> str:
        """Render the traceparent header value."""
        return f"00-{self.trace_id}-{self.span_id}-{self.flags}"

    def child(self) -> "TraceParent":
        """Keep the trace id and start a new span id."""
        return TraceParent(self.trace_id, secrets.token_hex(8), self.flags)


def new_trace() -> TraceParent:
    """Start a brand new trace."""
    return TraceParent(secrets.token_hex(16), secrets.token_hex(8), "01")


def parse_trace_parent(s: str) -> TraceParent | None:
    """Read a traceparent header. Returns None when unparseable."""
    parts = s.strip().split("-")
    if len(parts) != 4 or parts[0] != "00":
        return None
    if len(parts[1]) != 32 or len(parts[2]) != 16 or len(parts[3]) != 2:
        return None
    try:
        bytes.fromhex(parts[1])
        bytes.fromhex(parts[2])
        bytes.fromhex(parts[3])
    except ValueError:
        return None
    if parts[1].strip("0") == "" or parts[2].strip("0") == "":
        return None

    return TraceParent(parts[1], parts[2], parts[3])
`)

var tsTelTmpl = mustTmpl("tstel", `{{.Header}}
// Metrics and W3C trace context.

import { randomBytes } from "node:crypto";

export const BUCKETS = [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10];

/** Render a float the one agreed way. No exponent. No trailing zeros. */
export function formatFloat(f: number): string {
  if (Number.isInteger(f) && Math.abs(f) < 1e16) return String(f);

  const s = String(f);
  if (!s.includes("e") && !s.includes("E")) return s;

  return f.toFixed(17).replace(/0+$/, "").replace(/\.$/, "");
}

function keyOf(name: string, labels?: Record<string, string>): string {
  if (!labels || Object.keys(labels).length === 0) return name;

  const parts = Object.keys(labels)
    .sort()
    .map((k) => `+"`${k}=\"${labels[k]}\"`"+`);

  return `+"`${name}{${parts.join(\",\")}}`"+`;
}

function baseName(k: string): string {
  const i = k.indexOf("{");

  return i === -1 ? k : k.slice(0, i);
}

function withLabel(k: string, label: string, value: string): string {
  const base = baseName(k);
  const i = k.indexOf("{");
  const inner = i === -1 ? "" : k.slice(i + 1, -1);
  const pair = `+"`${label}=\"${value}\"`"+`;
  if (inner === "") return `+"`${base}{${pair}}`"+`;

  const parts = [...inner.split(","), pair].sort();

  return `+"`${base}{${parts.join(\",\")}}`"+`;
}

/** Holds counters and histograms. */
export class Registry {
  private readonly counters = new Map<string, number>();
  private readonly histograms = new Map<string, number[]>();
  private readonly help = new Map<string, string>();

  inc(name: string, labels: Record<string, string> | undefined, by: number, help: string): void {
    const k = keyOf(name, labels);
    this.counters.set(k, (this.counters.get(k) ?? 0) + by);
    this.help.set(name, help);
  }

  observe(name: string, labels: Record<string, string> | undefined, v: number, help: string): void {
    const k = keyOf(name, labels);
    const arr = this.histograms.get(k) ?? [];
    arr.push(v);
    this.histograms.set(k, arr);
    this.help.set(name, help);
  }

  /** Produce the Prometheus text exposition format. */
  render(): string {
    const out: string[] = [];
    const seen = new Set<string>();

    for (const k of [...this.counters.keys()].sort()) {
      const base = baseName(k);
      if (!seen.has(base)) {
        out.push(`+"`# HELP ${base} ${this.help.get(base) ?? \"\"}`"+`);
        out.push(`+"`# TYPE ${base} counter`"+`);
        seen.add(base);
      }
      out.push(`+"`${k} ${formatFloat(this.counters.get(k)!)}`"+`);
    }

    for (const k of [...this.histograms.keys()].sort()) {
      const base = baseName(k);
      if (!seen.has(base)) {
        out.push(`+"`# HELP ${base} ${this.help.get(base) ?? \"\"}`"+`);
        out.push(`+"`# TYPE ${base} histogram`"+`);
        seen.add(base);
      }

      const samples = this.histograms.get(k)!;
      for (const bound of BUCKETS) {
        const count = samples.filter((s) => s <= bound).length;
        out.push(`+"`${withLabel(k, \"le\", formatFloat(bound))} ${count}`"+`);
      }
      out.push(`+"`${withLabel(k, \"le\", \"+Inf\")} ${samples.length}`"+`);
      out.push(`+"`${base}_sum ${formatFloat(samples.reduce((a, b) => a + b, 0))}`"+`);
      out.push(`+"`${base}_count ${samples.length}`"+`);
    }

    return out.length > 0 ? `+"`${out.join(\"\\n\")}\\n`"+` : "";
  }
}

/** A parsed W3C traceparent header. */
export class TraceParent {
  constructor(
    readonly traceId: string,
    readonly spanId: string,
    readonly flags: string,
  ) {}

  header(): string {
    return `+"`00-${this.traceId}-${this.spanId}-${this.flags}`"+`;
  }

  /** Keep the trace id and start a new span id. */
  child(): TraceParent {
    return new TraceParent(this.traceId, randomBytes(8).toString("hex"), this.flags);
  }
}

/** Start a brand new trace. */
export function newTrace(): TraceParent {
  return new TraceParent(randomBytes(16).toString("hex"), randomBytes(8).toString("hex"), "01");
}

/** Read a traceparent header. Returns undefined when unparseable. */
export function parseTraceParent(s: string): TraceParent | undefined {
  const parts = s.trim().split("-");
  if (parts.length !== 4 || parts[0] !== "00") return undefined;
  if (parts[1]!.length !== 32 || parts[2]!.length !== 16 || parts[3]!.length !== 2) return undefined;
  if (!/^[0-9a-f]+$/.test(parts[1]! + parts[2]! + parts[3]!)) return undefined;
  if (/^0+$/.test(parts[1]!) || /^0+$/.test(parts[2]!)) return undefined;

  return new TraceParent(parts[1]!, parts[2]!, parts[3]!);
}
`)

var rsTelTmpl = mustTmpl("rstel", `{{.Header}}
//! Metrics and W3C trace context generated from spec.yaml.

#![allow(dead_code)]

use std::collections::BTreeMap;

pub const BUCKETS: [f64; 11] = [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0];

/// Render a float the one agreed way. No exponent. No trailing zeros.
pub fn format_float(f: f64) -> String {
    if f == f.trunc() && f.abs() < 1e16 {
        return format!("{}", f as i64);
    }

    let mut s = format!("{f}");
    if s.contains('e') || s.contains('E') {
        s = format!("{f:.17}");
    }
    if s.contains('.') {
        s = s.trim_end_matches('0').trim_end_matches('.').to_string();
    }

    s
}

/// Join a metric name and a label set.
///
/// Built by hand rather than with format!. Braces in a format string collide
/// with the Go template that generates this file.
fn braced(base: &str, inner: &str) -> String {
    let mut out = String::with_capacity(base.len() + inner.len() + 2);
    out.push_str(base);
    out.push('{');
    out.push_str(inner);
    out.push('}');

    out
}

fn key_of(name: &str, labels: &[(&str, &str)]) -> String {
    if labels.is_empty() {
        return name.to_string();
    }

    let sorted: BTreeMap<&str, &str> = labels.iter().copied().collect();
    let parts: Vec<String> = sorted.iter().map(|(k, v)| format!("{k}=\"{v}\"")).collect();

    braced(name, &parts.join(","))
}

fn base_name(k: &str) -> String {
    match k.find('{') {
        Some(i) => k[..i].to_string(),
        None => k.to_string(),
    }
}

fn with_label(k: &str, label: &str, value: &str) -> String {
    let base = base_name(k);
    let inner = match k.find('{') {
        Some(i) => k[i + 1..k.len() - 1].to_string(),
        None => String::new(),
    };
    let pair = format!("{label}=\"{value}\"");

    if inner.is_empty() {
        return braced(&base, &pair);
    }

    let mut parts: Vec<String> = inner.split(',').map(str::to_string).collect();
    parts.push(pair);
    parts.sort();

    braced(&base, &parts.join(","))
}

/// Holds counters and histograms.
#[derive(Default)]
pub struct Registry {
    counters: BTreeMap<String, f64>,
    histograms: BTreeMap<String, Vec<f64>>,
    help: BTreeMap<String, String>,
}

impl Registry {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn inc(&mut self, name: &str, labels: &[(&str, &str)], by: f64, help: &str) {
        *self.counters.entry(key_of(name, labels)).or_insert(0.0) += by;
        self.help.insert(name.to_string(), help.to_string());
    }

    pub fn observe(&mut self, name: &str, labels: &[(&str, &str)], v: f64, help: &str) {
        self.histograms.entry(key_of(name, labels)).or_default().push(v);
        self.help.insert(name.to_string(), help.to_string());
    }

    /// Produce the Prometheus text exposition format.
    pub fn render(&self) -> String {
        let mut out: Vec<String> = Vec::new();
        let mut seen: Vec<String> = Vec::new();

        for (k, v) in &self.counters {
            let base = base_name(k);
            if !seen.contains(&base) {
                let help = self.help.get(&base).cloned().unwrap_or_default();
                out.push(format!("# HELP {base} {help}"));
                out.push(format!("# TYPE {base} counter"));
                seen.push(base.clone());
            }
            out.push(format!("{} {}", k, format_float(*v)));
        }

        for (k, samples) in &self.histograms {
            let base = base_name(k);
            if !seen.contains(&base) {
                let help = self.help.get(&base).cloned().unwrap_or_default();
                out.push(format!("# HELP {base} {help}"));
                out.push(format!("# TYPE {base} histogram"));
                seen.push(base.clone());
            }

            for bound in BUCKETS {
                let count = samples.iter().filter(|s| **s <= bound).count();
                out.push(format!("{} {}", with_label(k, "le", &format_float(bound)), count));
            }
            out.push(format!("{} {}", with_label(k, "le", "+Inf"), samples.len()));
            let sum: f64 = samples.iter().sum();
            out.push(format!("{}_sum {}", base, format_float(sum)));
            out.push(format!("{}_count {}", base, samples.len()));
        }

        if out.is_empty() {
            return String::new();
        }

        let mut s = out.join("\n");
        s.push('\n');

        s
    }
}

/// A parsed W3C traceparent header.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TraceParent {
    pub trace_id: String,
    pub span_id: String,
    pub flags: String,
}

impl TraceParent {
    /// Render the traceparent header value.
    pub fn header(&self) -> String {
        format!("00-{}-{}-{}", self.trace_id, self.span_id, self.flags)
    }

    /// Keep the trace id and start a new span id.
    pub fn child(&self) -> Self {
        Self {
            trace_id: self.trace_id.clone(),
            span_id: rand_hex(8),
            flags: self.flags.clone(),
        }
    }
}

/// Start a brand new trace.
pub fn new_trace() -> TraceParent {
    TraceParent {
        trace_id: rand_hex(16),
        span_id: rand_hex(8),
        flags: "01".to_string(),
    }
}

/// Read a traceparent header. Returns None when unparseable.
pub fn parse_trace_parent(s: &str) -> Option<TraceParent> {
    let parts: Vec<&str> = s.trim().split('-').collect();
    if parts.len() != 4 || parts[0] != "00" {
        return None;
    }
    if parts[1].len() != 32 || parts[2].len() != 16 || parts[3].len() != 2 {
        return None;
    }
    if !parts[1..].iter().all(|p| p.chars().all(|c| c.is_ascii_hexdigit() && !c.is_ascii_uppercase())) {
        return None;
    }
    if parts[1].trim_matches('0').is_empty() || parts[2].trim_matches('0').is_empty() {
        return None;
    }

    Some(TraceParent {
        trace_id: parts[1].to_string(),
        span_id: parts[2].to_string(),
        flags: parts[3].to_string(),
    })
}

/// Random hex without pulling in a random crate.
fn rand_hex(n: usize) -> String {
    use std::time::{SystemTime, UNIX_EPOCH};

    let mut seed = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|d| d.as_nanos() as u64)
        .unwrap_or(0x9E3779B97F4A7C15);

    let mut out = String::with_capacity(n * 2);
    for _ in 0..n {
        // xorshift64. Good enough for a span id and it adds no dependency.
        seed ^= seed << 13;
        seed ^= seed >> 7;
        seed ^= seed << 17;
        out.push_str(&format!("{:02x}", (seed & 0xff) as u8));
    }

    out
}
`)
