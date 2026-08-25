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

// Package concerns renders the cross-cutting concern modules the four
// concern engines serve: logging, telemetry, resilience and delivery, one
// emitter per concern, four languages each. Paths are engine-local; the
// consuming cell's directory is the module.
//
// Every engine renders through these functions, so two cells naming the
// same concern cannot drift: one template, one renderer.
package concerns

import "fmt"

// Language selects a template set.
type Language string

// The four languages every concern covers.
const (
	LangGo         Language = "go"
	LangRust       Language = "rust"
	LangPython     Language = "python"
	LangTypeScript Language = "typescript"
)

// Binary is one shipped binary the delivery concern builds a container for.
type Binary struct {
	// Name is the binary name, e.g. golden-go-server.
	Name string
	// Kind says what the binary does: a "server" gets EXPOSE and a
	// healthcheck.
	Kind string
}

// File is one generated file, path relative to the engine directory.
type File struct {
	Path    string
	Content string
}

// ErrNoEmitterForConcern is returned for a language no concern covers.
var ErrNoEmitterForConcern = fmt.Errorf("no emitter for language")

// EmitLogging answers the logging module of one language.
func EmitLogging(lang Language) ([]File, error) {
	switch lang {
	case LangGo:
		body, err := renderLogging(goLogTmpl, map[string]any{
			"Header":  header("//", tool),
			"Package": "logging",
		}, "go")
		if err != nil {
			return nil, err
		}

		return []File{{Path: "zz_generated.logging.go", Content: body}}, nil
	case LangRust:
		body, err := renderLogging(rsLogTmpl, map[string]any{"Header": header("//", tool)}, "rust")
		if err != nil {
			return nil, err
		}

		return []File{{Path: "zz_generated_logging.rs", Content: body}}, nil
	case LangPython:
		body, err := renderLogging(pyLogTmpl, map[string]any{"Header": header("#", tool)}, "python")
		if err != nil {
			return nil, err
		}

		return []File{{Path: "zz_generated_logging.py", Content: body}}, nil
	case LangTypeScript:
		body, err := renderLogging(tsLogTmpl, map[string]any{"Header": header("//", tool)}, "typescript")
		if err != nil {
			return nil, err
		}

		return []File{{Path: "zz_generated.logging.ts", Content: body}}, nil
	default:
		return nil, fmt.Errorf("emitting logging for %q: %w", lang, ErrNoEmitterForConcern)
	}
}

// EmitTelemetry answers the telemetry module of one language. Go splits
// metrics and tracing into two files; the others hold both in one.
func EmitTelemetry(lang Language) ([]File, error) {
	switch lang {
	case LangGo:
		metrics, err := renderLogging(goTelTmpl, map[string]any{
			"Header": header("//", tool), "Package": "telemetry",
		}, "go metrics")
		if err != nil {
			return nil, err
		}

		tracing, err := renderLogging(goTraceTmpl, map[string]any{
			"Header": header("//", tool), "Package": "telemetry",
		}, "go tracing")
		if err != nil {
			return nil, err
		}

		return []File{
			{Path: "zz_generated.metrics.go", Content: metrics},
			{Path: "zz_generated.tracing.go", Content: tracing},
		}, nil
	case LangRust:
		body, err := renderLogging(rsTelTmpl, map[string]any{"Header": header("//", tool)}, "rust telemetry")
		if err != nil {
			return nil, err
		}

		return []File{{Path: "zz_generated_telemetry.rs", Content: body}}, nil
	case LangPython:
		body, err := renderLogging(pyTelTmpl, map[string]any{"Header": header("#", tool)}, "python telemetry")
		if err != nil {
			return nil, err
		}

		return []File{{Path: "zz_generated_telemetry.py", Content: body}}, nil
	case LangTypeScript:
		body, err := renderLogging(tsTelTmpl, map[string]any{"Header": header("//", tool)}, "typescript telemetry")
		if err != nil {
			return nil, err
		}

		return []File{{Path: "zz_generated.telemetry.ts", Content: body}}, nil
	default:
		return nil, fmt.Errorf("emitting telemetry for %q: %w", lang, ErrNoEmitterForConcern)
	}
}

// EmitResilience answers the resilience module of one language.
func EmitResilience(lang Language) ([]File, error) {
	switch lang {
	case LangGo:
		body, err := renderLogging(goResTmpl, map[string]any{
			"Header": header("//", tool), "Package": "resilience",
		}, "resilience")
		if err != nil {
			return nil, err
		}

		return []File{{Path: "zz_generated.resilience.go", Content: body}}, nil
	case LangRust:
		body, err := renderLogging(rsResTmpl, map[string]any{"Header": header("//", tool)}, "resilience")
		if err != nil {
			return nil, err
		}

		return []File{{Path: "zz_generated_resilience.rs", Content: body}}, nil
	case LangPython:
		body, err := renderLogging(pyResTmpl, map[string]any{"Header": header("#", tool)}, "resilience")
		if err != nil {
			return nil, err
		}

		return []File{{Path: "zz_generated_resilience.py", Content: body}}, nil
	case LangTypeScript:
		body, err := renderLogging(tsResTmpl, map[string]any{"Header": header("//", tool)}, "resilience")
		if err != nil {
			return nil, err
		}

		return []File{{Path: "zz_generated.resilience.ts", Content: body}}, nil
	default:
		return nil, fmt.Errorf("emitting resilience for %q: %w", lang, ErrNoEmitterForConcern)
	}
}

// header is the do-not-edit banner every emitter writes.
func header(comment, tool string) string {
	return fmt.Sprintf(
		"%s Code generated by %s. DO NOT EDIT.\n%s Source: forge-dev.yaml\n",
		comment, tool, comment,
	)
}

const tool = "forge-dev-codegen"
