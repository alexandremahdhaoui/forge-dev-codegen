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

package authzgen

import (
	"encoding/json"
	"fmt"

	"github.com/openfga/language/pkg/go/transformer"
	"google.golang.org/protobuf/encoding/protojson"
)

const combinedWirePath = "zz_generated_fga.model.json"

func wirePath(m Module) string {
	return "zz_generated_" + m.Name + ".model.json"
}

func emitWire(m Module, model string) (string, error) {
	proto, _, err := transformer.TransformModularDSLToProto(model)
	if err != nil {
		return "", fmt.Errorf("transforming the module file of %q into type definitions: %w", m.Name, err)
	}

	proto.SchemaVersion = schemaVersion

	encoded, err := protojson.Marshal(proto)
	if err != nil {
		return "", fmt.Errorf("encoding the type definitions of %q: %w", m.Name, err)
	}

	return stableJSON(m.Name, encoded)
}

func composeWire(name, schema string, loaded []loadedModule) (string, error) {
	files := make([]transformer.ModuleFile, 0, len(loaded))

	for _, one := range loaded {
		files = append(files, transformer.ModuleFile{Name: one.manifest.Model, Contents: one.model})
	}

	proto, err := transformer.TransformModuleFilesToModel(files, schema)
	if err != nil {
		return "", fmt.Errorf("composing the type definitions of %q: %w", name, err)
	}

	encoded, err := protojson.Marshal(proto)
	if err != nil {
		return "", fmt.Errorf("encoding the type definitions of %q: %w", name, err)
	}

	return stableJSON(name, encoded)
}

func stableJSON(name string, encoded []byte) (string, error) {
	var read any

	if err := json.Unmarshal(encoded, &read); err != nil {
		return "", fmt.Errorf("reading back the type definitions of %q: %w", name, err)
	}

	ordered, err := json.MarshalIndent(read, "", "  ")
	if err != nil {
		return "", fmt.Errorf("ordering the type definitions of %q: %w", name, err)
	}

	return string(ordered) + "\n", nil
}
