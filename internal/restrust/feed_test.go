package restrust_test

import (
	"os"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/restrust"
)

const chatLineFeedFixture = "testdata/chat_line_feed.openapi.yaml"

func chatLineFeedSpec(t *testing.T) string {
	t.Helper()

	doc, err := os.ReadFile(chatLineFeedFixture)
	if err != nil {
		t.Fatalf("reading the fixture %s: %v", chatLineFeedFixture, err)
	}

	return string(doc)
}

func TestAStoreFeedingAStreamOfItsOwnTypeImportsEachTypeOnceInBothStoreAdapters(t *testing.T) {
	files := generateSpec(t, chatLineFeedSpec(t), restrust.Options{Service: "songe-social"})

	for _, path := range []string{"adapter/zz_generated_chat_line_sqlite.rs", "adapter/zz_generated_chat_line_memory.rs"} {
		file, emitted := files[path]
		if !emitted {
			t.Fatalf("%s was not emitted, the emitted paths are %q", path, paths(files))
		}

		if !strings.Contains(file.Content, "use crate::rest::types::chat_line::ChatLine;\n") {
			t.Errorf("%s never imports ChatLine\n%s", path, file.Content)
		}

		seen := map[string]bool{}

		for _, line := range strings.Split(file.Content, "\n") {
			if !strings.HasPrefix(line, "use ") {
				continue
			}

			if seen[line] {
				t.Errorf("%s carries %q twice\n%s", path, line, file.Content)
			}

			seen[line] = true
		}
	}
}
