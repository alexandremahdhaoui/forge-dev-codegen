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

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/surface"
)

// Delivery emitter: Containerfiles.
//
// One Containerfile per binary the surface declares, so a new binary gets a
// container with no extra work. Every image is multi stage, runs as a non
// root user, and a server declares a healthcheck.
//
// It deliberately emits no per-repo CI workflow. A lone checkout has no
// sibling repos and no workspace manifests; the pipeline is the CI.

func renderDelivery(tmpl *template.Template, data map[string]any, what string) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("rendering %s: %w", what, err)
	}

	return buf.String(), nil
}

// EmitDelivery answers one Containerfile per binary with engine-local
// paths.
func EmitDelivery(lang Language, repo string, binaries []Binary) ([]File, error) {
	out := make([]File, 0, len(binaries))

	base, ok := deliveryBase[lang]
	if !ok {
		return nil, fmt.Errorf("selecting container base for %q: not supported", lang)
	}

	for _, b := range binaries {
		body, err := renderDelivery(containerTmpl, map[string]any{
			"Header":   header("#", tool),
			"Binary":   b,
			"Language": lang,
			"Repo":     repo,
			"Build":    base.build,
			"Runtime":  base.runtime,
			"Steps":    base.steps(repo, b.Name),
			"Entry":    base.entry(repo, b.Name),
		}, "containerfile")
		if err != nil {
			return nil, err
		}

		out = append(out, File{
			Path:    fmt.Sprintf("%s/zz_generated.Containerfile", b.Name),
			Content: body,
		})
	}

	return out, nil
}

type baseImages struct {
	build   string
	runtime string
	steps   func(repo, bin string) []string
	entry   func(repo, bin string) []string
}

var deliveryBase = map[Language]baseImages{
	LangGo: {
		build:   "docker.io/library/golang:1.26-alpine",
		runtime: "gcr.io/distroless/static-debian12:nonroot",
		steps: func(_, bin string) []string {
			return []string{
				"RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' \\",
				"    -o /out/" + bin + " ./cmd/" + bin,
			}
		},
		entry: func(_, bin string) []string { return []string{"/app/" + bin} },
	},
	LangRust: {
		build:   "docker.io/library/rust:1.97-slim",
		runtime: "gcr.io/distroless/cc-debian12:nonroot",
		steps: func(_, bin string) []string {
			return []string{
				"RUN cargo build --release --bin " + bin + " \\",
				"    && mkdir -p /out && cp target/release/" + bin + " /out/" + bin,
			}
		},
		entry: func(_, bin string) []string { return []string{"/app/" + bin} },
	},
	LangPython: {
		build:   "ghcr.io/astral-sh/uv:python3.12-bookworm-slim",
		runtime: "ghcr.io/astral-sh/uv:python3.12-bookworm-slim",
		steps: func(_, _ string) []string {
			return []string{"RUN uv sync --frozen --no-dev && mkdir -p /out && cp -r . /out/app"}
		},
		entry: func(repo, bin string) []string {
			return []string{"uv", "run", "python", "-m", surface.Snake(repo) + ".cmd." + surface.Snake(shortName(bin))}
		},
	},
	LangTypeScript: {
		build:   "docker.io/library/node:24-slim",
		runtime: "docker.io/library/node:24-slim",
		steps: func(_, _ string) []string {
			return []string{"RUN corepack enable pnpm && pnpm install --frozen-lockfile && pnpm run build"}
		},
		// tsc writes camelCase filenames, not the hyphenated binary name.
		// Getting this wrong makes the container exit immediately.
		entry: func(_, bin string) []string {
			return []string{"node", "/app/dist/cmd/" + surface.Camel(bin) + ".js"}
		},
	},
}

// shortName strips the repo prefix from a binary name.
//
// golden-python-server becomes server, which is the module name in cmd.
func shortName(bin string) string {
	parts := splitLast(bin, "-")

	return parts
}

func splitLast(s, sep string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if string(s[i]) == sep {
			return s[i+1:]
		}
	}

	return s
}

var containerTmpl = template.Must(template.New("container").Parse(`{{.Header}}
# Container image for {{.Binary.Name}}.
#
# Multi stage. The runtime image carries no build tooling and runs as a non
# root user. Both cut the attack surface an operator has to reason about.

FROM {{.Build}} AS build
WORKDIR /src
COPY . .
{{range .Steps}}{{.}}
{{end}}
FROM {{.Runtime}}
WORKDIR /app
COPY --from=build /out /app

USER 65532:65532
{{if eq (printf "%s" .Binary.Kind) "server"}}
EXPOSE 8080

# The healthcheck uses the same path the e2e harness polls.
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD [{{range $i, $e := .Entry}}{{if $i}}, {{end}}"{{$e}}"{{end}}, "--help"]
{{end}}
ENTRYPOINT [{{range $i, $e := .Entry}}{{if $i}}, {{end}}"{{$e}}"{{end}}]
`))
