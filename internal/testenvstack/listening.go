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

package testenvstack

import (
	"strconv"
	"strings"
)

type Ports struct {
	Rest int
	Grpc int
	Udp  int
}

func (p Ports) Complete() bool {
	return p.Rest > 0 && p.Grpc > 0 && p.Udp > 0
}

var transportByKeyword = map[string]string{
	"LISTENING":      "rest",
	"LISTENING_GRPC": "grpc",
	"LISTENING_UDP":  "udp",
}

func ParseListening(line string) (string, int, bool) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return "", 0, false
	}

	transport, ok := transportByKeyword[fields[0]]
	if !ok {
		return "", 0, false
	}

	port, err := strconv.Atoi(fields[1])
	if err != nil || port < 1 || port > 65535 {
		return "", 0, false
	}

	return transport, port, true
}

func FindListening(output string) (Ports, bool) {
	var ports Ports

	for _, line := range strings.Split(output, "\n") {
		transport, port, ok := ParseListening(line)
		if !ok {
			continue
		}

		switch transport {
		case "rest":
			if ports.Rest == 0 {
				ports.Rest = port
			}
		case "grpc":
			if ports.Grpc == 0 {
				ports.Grpc = port
			}
		case "udp":
			if ports.Udp == 0 {
				ports.Udp = port
			}
		}
	}

	return ports, ports.Rest > 0
}
