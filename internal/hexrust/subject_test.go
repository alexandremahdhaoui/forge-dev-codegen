package hexrust_test

import (
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/hexrust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
)

func TestEveryPortErrorAnswersTheIdentifierItCarriesThroughSubject(t *testing.T) {
	root := t.TempDir()

	writeRestCell(t, root, "rest", guardedHelloSpec, restrust.SideBoth, false)
	writeRestCell(t, root, "account", accountSpec, restrust.SideClient, false)

	files, err := hexrust.Generate(hexrust.Options{
		Service: "songe-hello",
		SrcDir:  root,
		Cells:   []string{"account", "rest"},
		Ports:   guardedCrateRootPorts,
		Wiring:  []byte("binary: songe-hello-node\ndrivers:\n  rest: { enabled: true }\n"),
	})
	if err != nil {
		t.Fatalf("generating: %v", err)
	}

	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = f.Content
	}

	cases := []struct {
		port string
		path string
		want string
	}{
		{
			port: "ticket_verifier",
			path: "src/port/zz_generated_ticket_verifier.rs",
			want: "impl TicketVerifierError {\n    pub fn subject(&self) -> String {\n        \"ticket_verifier\".to_string()\n    }\n}",
		},
		{
			port: "token_source",
			path: "src/port/zz_generated_token_source.rs",
			want: "impl TokenSourceError {\n    pub fn subject(&self) -> String {\n        \"token_source\".to_string()\n    }\n}",
		},
	}

	for _, tc := range cases {
		t.Run(tc.port, func(t *testing.T) {
			content, emitted := byPath[tc.path]
			if !emitted {
				t.Fatalf("%s was not emitted at the crate root", tc.path)
			}

			if !strings.Contains(content, tc.want) {
				t.Errorf("%s lacks %q\n%s", tc.path, tc.want, content)
			}
		})
	}
}
