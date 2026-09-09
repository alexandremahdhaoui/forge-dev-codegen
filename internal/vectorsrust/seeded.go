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

package vectorsrust

import (
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/alexandremahdhaoui/forge-dev-codegen/internal/udprust"
	"github.com/alexandremahdhaoui/forge-dev-codegen/pkg/rustname"
)

const CellConfigFile = "forge-dev.yaml"

type cellConfig struct {
	Layout struct {
		Ports []struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
		} `json:"ports"`
	} `json:"layout"`
}

func SeededPortOfCell(crateDir, cell string) (*RngPort, error) {
	path := filepath.Join(crateDir, "src", cell, CellConfigFile)

	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("reading the declaration of cell %q at %q: %w", cell, path, err)
	}

	var config cellConfig
	if err := yaml.Unmarshal(body, &config); err != nil {
		return nil, fmt.Errorf("reading the declaration of cell %q at %q: %w", cell, path, err)
	}

	seeded := []*RngPort{}

	for _, port := range config.Layout.Ports {
		if port.Kind != udprust.CounterPortKind {
			continue
		}

		seeded = append(seeded, &RngPort{
			Trait:   port.Name,
			Module:  cell + "::port::" + rustname.Snake(port.Name),
			Method:  "next",
			Returns: "u64",
		})
	}

	if len(seeded) == 0 {
		return nil, nil
	}

	if len(seeded) > 1 {
		return nil, fmt.Errorf(
			"reading the declaration of cell %q: it declares %d counter ports and a seeded vector names one, give the cell one counter",
			cell, len(seeded),
		)
	}

	return seeded[0], nil
}
