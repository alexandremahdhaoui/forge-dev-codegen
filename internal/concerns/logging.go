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

import (
	"bytes"
	"fmt"
	"text/template"
)

// Logging emitters.
//
// Every language produces a logger with the same wire contract.
//
//   - One JSON object per line on stderr.
//   - Keys sorted so two languages produce byte identical lines.
//   - Always present: level, msg, service.
//   - Extra fields come from the call site and sort in with the rest.
//   - Levels are debug, info, warn, error. Lower levels are dropped.
//
// The timestamp is deliberately NOT part of the line. A timestamp cannot match
// across processes, and a conformance test that strips it is weaker than one
// that never emits it. Callers that need time add it as an explicit field.

func renderLogging(tmpl *template.Template, data map[string]any, what string) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("rendering %s logging: %w", what, err)
	}

	return buf.String(), nil
}

var goLogTmpl = template.Must(template.New("golog").Parse(`{{.Header}}
package {{.Package}}

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Level is a log severity.
type Level int

// Levels in increasing severity.
const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// ParseLevel maps a name to a Level. Unknown names fall back to info.
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// String renders a Level as it appears on the wire.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "info"
	}
}

// Logger writes one sorted JSON object per line.
type Logger struct {
	out     io.Writer
	level   Level
	service string
}

// NewLogger builds a logger for a service at a level.
func NewLogger(out io.Writer, level, service string) *Logger {
	return &Logger{out: out, level: ParseLevel(level), service: service}
}

// Debug logs at debug.
func (l *Logger) Debug(msg string, fields map[string]any) { l.log(LevelDebug, msg, fields) }

// Info logs at info.
func (l *Logger) Info(msg string, fields map[string]any) { l.log(LevelInfo, msg, fields) }

// Warn logs at warn.
func (l *Logger) Warn(msg string, fields map[string]any) { l.log(LevelWarn, msg, fields) }

// Error logs at error.
func (l *Logger) Error(msg string, fields map[string]any) { l.log(LevelError, msg, fields) }

// log drops anything below the configured level then writes a sorted line.
func (l *Logger) log(level Level, msg string, fields map[string]any) {
	if level < l.level {
		return
	}

	merged := map[string]any{
		"level":   level.String(),
		"msg":     msg,
		"service": l.service,
	}
	for k, v := range fields {
		merged[k] = v
	}

	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(",")
		}
		kb, _ := json.Marshal(k)   //nolint:errcheck // a string always marshals
		vb, err := json.Marshal(merged[k])
		if err != nil {
			vb = []byte("null")
		}
		b.Write(kb)
		b.WriteString(":")
		b.Write(vb)
	}
	b.WriteString("}")

	fmt.Fprintln(l.out, b.String()) //nolint:errcheck // logging must not fail the caller
}
`))

var pyLogTmpl = template.Must(template.New("pylog").Parse(`{{.Header}}
"""Structured logging generated from spec.yaml.

One sorted JSON object per line. Keys are sorted so two languages produce
byte identical lines.
"""

from __future__ import annotations

import json
import sys
from typing import Any, TextIO

_LEVELS = {"debug": 0, "info": 1, "warn": 2, "error": 3}


def parse_level(s: str) -> int:
    """Map a level name to a rank. Unknown names fall back to info."""
    name = s.strip().lower()
    if name == "warning":
        name = "warn"

    return _LEVELS.get(name, 1)


def level_name(rank: int) -> str:
    """Render a rank as it appears on the wire."""
    for k, v in _LEVELS.items():
        if v == rank:
            return k

    return "info"


class Logger:
    """Writes one sorted JSON object per line."""

    def __init__(self, out: TextIO, level: str, service: str) -> None:
        self._out = out
        self._level = parse_level(level)
        self._service = service

    def debug(self, msg: str, fields: dict[str, Any] | None = None) -> None:
        """Log at debug."""
        self._log(0, msg, fields)

    def info(self, msg: str, fields: dict[str, Any] | None = None) -> None:
        """Log at info."""
        self._log(1, msg, fields)

    def warn(self, msg: str, fields: dict[str, Any] | None = None) -> None:
        """Log at warn."""
        self._log(2, msg, fields)

    def error(self, msg: str, fields: dict[str, Any] | None = None) -> None:
        """Log at error."""
        self._log(3, msg, fields)

    def _log(self, rank: int, msg: str, fields: dict[str, Any] | None) -> None:
        if rank < self._level:
            return

        merged: dict[str, Any] = {
            "level": level_name(rank),
            "msg": msg,
            "service": self._service,
        }
        if fields:
            merged.update(fields)

        # ensure_ascii=False keeps raw UTF-8. The default escapes non-ASCII
        # to \uXXXX which no other language does. See divergences D7.
        line = json.dumps(
            merged, sort_keys=True, separators=(",", ":"), ensure_ascii=False
        )
        print(line, file=self._out, flush=True)


def new_logger(level: str, service: str, out: TextIO | None = None) -> Logger:
    """Build a logger writing to stderr by default."""
    return Logger(out if out is not None else sys.stderr, level, service)
`))

var tsLogTmpl = template.Must(template.New("tslog").Parse(`{{.Header}}
// Structured logging. One sorted JSON object per line.

export type Fields = Record<string, unknown>;

const LEVELS: Record<string, number> = { debug: 0, info: 1, warn: 2, error: 3 };

/** Map a level name to a rank. Unknown names fall back to info. */
export function parseLevel(s: string): number {
  const name = s.trim().toLowerCase();
  const key = name === "warning" ? "warn" : name;

  return LEVELS[key] ?? 1;
}

/** Render a rank as it appears on the wire. */
export function levelName(rank: number): string {
  for (const [k, v] of Object.entries(LEVELS)) {
    if (v === rank) return k;
  }

  return "info";
}

/** Writes one sorted JSON object per line. */
export class Logger {
  constructor(
    private readonly write: (line: string) => void,
    private readonly level: number,
    private readonly service: string,
  ) {}

  debug(msg: string, fields: Fields = {}): void {
    this.log(0, msg, fields);
  }

  info(msg: string, fields: Fields = {}): void {
    this.log(1, msg, fields);
  }

  warn(msg: string, fields: Fields = {}): void {
    this.log(2, msg, fields);
  }

  error(msg: string, fields: Fields = {}): void {
    this.log(3, msg, fields);
  }

  private log(rank: number, msg: string, fields: Fields): void {
    if (rank < this.level) return;

    const merged: Fields = {
      level: levelName(rank),
      msg,
      service: this.service,
      ...fields,
    };

    const keys = Object.keys(merged).sort();
    const pairs = keys.map((k) => ` + "`${JSON.stringify(k)}:${JSON.stringify(merged[k])}`" + `);

    this.write(` + "`{${pairs.join(\",\")}}`" + `);
  }
}

/** Build a logger writing to stderr. */
export function newLogger(level: string, service: string): Logger {
  return new Logger((line) => process.stderr.write(` + "`${line}\\n`" + `), parseLevel(level), service);
}
`))

var rsLogTmpl = template.Must(template.New("rslog").Parse(`{{.Header}}
//! Structured logging generated from spec.yaml.
//!
//! One sorted JSON object per line. Keys are sorted so two languages produce
//! byte identical lines.

#![allow(dead_code)]

use std::collections::BTreeMap;

/// A log severity.
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub enum Level {
    Debug,
    Info,
    Warn,
    Error,
}

impl Level {
    /// Map a level name to a Level. Unknown names fall back to info.
    pub fn parse(s: &str) -> Self {
        match s.trim().to_ascii_lowercase().as_str() {
            "debug" => Level::Debug,
            "warn" | "warning" => Level::Warn,
            "error" => Level::Error,
            _ => Level::Info,
        }
    }

    /// Render as it appears on the wire.
    pub fn as_str(self) -> &'static str {
        match self {
            Level::Debug => "debug",
            Level::Info => "info",
            Level::Warn => "warn",
            Level::Error => "error",
        }
    }
}

/// A field value the logger can render.
#[derive(Debug, Clone)]
pub enum Value {
    Str(String),
    Int(i64),
    Bool(bool),
}

impl Value {
    fn render(&self) -> String {
        match self {
            Value::Str(s) => format!("\"{}\"", escape(s)),
            Value::Int(i) => i.to_string(),
            Value::Bool(b) => b.to_string(),
        }
    }
}

fn escape(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    for c in s.chars() {
        match c {
            '"' => out.push_str("\\\""),
            '\\' => out.push_str("\\\\"),
            '\n' => out.push_str("\\n"),
            '\t' => out.push_str("\\t"),
            '\r' => out.push_str("\\r"),
            c => out.push(c),
        }
    }

    out
}

/// Writes one sorted JSON object per line.
pub struct Logger {
    level: Level,
    service: String,
}

impl Logger {
    /// Build a logger for a service at a level.
    pub fn new(level: &str, service: &str) -> Self {
        Self {
            level: Level::parse(level),
            service: service.to_string(),
        }
    }

    pub fn debug(&self, msg: &str, fields: &[(&str, Value)]) {
        self.log(Level::Debug, msg, fields);
    }

    pub fn info(&self, msg: &str, fields: &[(&str, Value)]) {
        self.log(Level::Info, msg, fields);
    }

    pub fn warn(&self, msg: &str, fields: &[(&str, Value)]) {
        self.log(Level::Warn, msg, fields);
    }

    pub fn error(&self, msg: &str, fields: &[(&str, Value)]) {
        self.log(Level::Error, msg, fields);
    }

    fn log(&self, level: Level, msg: &str, fields: &[(&str, Value)]) {
        if level < self.level {
            return;
        }

        // BTreeMap keeps keys sorted, which is the cross language contract.
        let mut merged: BTreeMap<String, Value> = BTreeMap::new();
        merged.insert("level".to_string(), Value::Str(level.as_str().to_string()));
        merged.insert("msg".to_string(), Value::Str(msg.to_string()));
        merged.insert("service".to_string(), Value::Str(self.service.clone()));
        for (k, v) in fields {
            merged.insert((*k).to_string(), v.clone());
        }

        let pairs: Vec<String> = merged
            .iter()
            .map(|(k, v)| format!("\"{}\":{}", escape(k), v.render()))
            .collect();

        let mut line = String::from("{");
        line.push_str(&pairs.join(","));
        line.push('}');

        eprintln!("{line}");
    }
}
`))
