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

const listeningKeyword = "LISTENING"

const udpSuffix = "UDP"

func ParseListening(line string) (string, int, bool) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return "", 0, false
	}

	suffix, found := strings.CutPrefix(fields[0], listeningKeyword)
	if !found {
		return "", 0, false
	}

	if suffix != "" && !strings.HasPrefix(suffix, "_") {
		return "", 0, false
	}

	port, err := strconv.Atoi(fields[1])
	if err != nil || port < 1 || port > 65535 {
		return "", 0, false
	}

	return strings.TrimPrefix(suffix, "_"), port, true
}

func FindListening(output string) (map[string]int, bool) {
	ports := map[string]int{}

	for _, line := range strings.Split(output, "\n") {
		suffix, port, ok := ParseListening(line)
		if !ok {
			continue
		}

		if _, already := ports[suffix]; already {
			continue
		}

		ports[suffix] = port
	}

	_, announced := ports[""]

	return ports, announced
}

func Addresses(addrEnv string, discovered map[string]int) map[string]string {
	out := make(map[string]string, len(discovered))

	for suffix, port := range discovered {
		key := addrEnv
		if suffix != "" {
			key = addrEnv + "_" + suffix
		}

		address := LoopbackHost + ":" + strconv.Itoa(port)
		if suffix != udpSuffix {
			address = "http://" + address
		}

		out[key] = address
	}

	return out
}
