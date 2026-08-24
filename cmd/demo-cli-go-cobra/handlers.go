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

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// NewCLIHandlers is byte-for-byte the handlers file the builtin cli cell
// would take: the dispatcher around it comes from cli-go-cobra instead, and
// nothing here knows or cares. greet prints the canonical JSON every cli
// demo prints, so golden-e2e can compare the four languages' cells.
func NewCLIHandlers() CLIHandlers {
	return CLIHandlers{
		Greet: func(args []string) int {
			if args == nil {
				args = []string{}
			}

			out, err := json.Marshal(map[string]any{"command": "greet", "args": args})
			if err != nil {
				fmt.Fprintf(os.Stderr, "demo-cli-go-cobra: rendering the answer: %v\n", err)

				return 1
			}

			fmt.Println(string(out))

			return 0
		},
		Fail: func(args []string) int {
			if len(args) == 0 {
				return 1
			}

			code, err := strconv.Atoi(args[0])
			if err != nil {
				return 1
			}

			return code
		},
	}
}
