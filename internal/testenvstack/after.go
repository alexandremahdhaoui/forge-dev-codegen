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
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type After struct {
	Command  string
	Args     []string
	Export   string
	Regex    string
	JSONPath string
}

func (a After) Validate() error {
	if a.Export == "" {
		return fmt.Errorf("the after command %q: export is required and names the environment variable the captured value lands in", a.Command)
	}

	if a.Regex == "" && a.JSONPath == "" {
		return fmt.Errorf("the after command %q: one of regex and jsonPath is required and neither is set", a.Command)
	}

	if a.Regex != "" && a.JSONPath != "" {
		return fmt.Errorf("the after command %q: regex and jsonPath both name the value to capture and only one may be set", a.Command)
	}

	if a.Regex == "" {
		return nil
	}

	pattern, err := regexp.Compile(a.Regex)
	if err != nil {
		return fmt.Errorf("the after command %q: reading the regex %q: %w", a.Command, a.Regex, err)
	}

	if pattern.NumSubexp() != 1 {
		return fmt.Errorf("the after command %q: the regex %q has %d capturing groups and exactly one is required", a.Command, a.Regex, pattern.NumSubexp())
	}

	return nil
}

func RunAfter(ctx context.Context, dir string, env map[string]string, placeholders Placeholders, after After) (string, error) {
	if err := after.Validate(); err != nil {
		return "", err
	}

	args, err := placeholders.SubstituteAll(after.Args)
	if err != nil {
		return "", fmt.Errorf("reading the args of the after command %q: %w", after.Command, err)
	}

	command := exec.CommandContext(ctx, after.Command, args...)
	command.Dir = dir
	command.Env = sortedEnv(mergedEnv(env))

	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("running the after command %q with %v: %w", after.Command, args, err)
	}

	value, err := after.extract(string(output))
	if err != nil {
		return "", fmt.Errorf("exporting %s: %w", after.Export, err)
	}

	return value, nil
}

func (a After) extract(output string) (string, error) {
	if a.Regex != "" {
		pattern, err := regexp.Compile(a.Regex)
		if err != nil {
			return "", fmt.Errorf("reading the regex %q: %w", a.Regex, err)
		}

		match := pattern.FindStringSubmatch(output)
		if match == nil {
			return "", fmt.Errorf("the regex %q matched nothing in the answer %q", a.Regex, output)
		}

		return match[1], nil
	}

	var document any
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		return "", fmt.Errorf("reading the answer %q as json: %w", output, err)
	}

	return walk(document, a.JSONPath)
}

func walk(document any, path string) (string, error) {
	current := document

	for _, segment := range strings.Split(path, ".") {
		next, err := step(current, segment, path)
		if err != nil {
			return "", err
		}

		current = next
	}

	return render(current, path)
}

func step(current any, segment string, path string) (any, error) {
	switch holder := current.(type) {
	case map[string]any:
		value, ok := holder[segment]
		if !ok {
			return nil, fmt.Errorf("the json path %q: %q names nothing; the keys are %s", path, segment, keysOf(holder))
		}

		return value, nil
	case []any:
		index, err := strconv.Atoi(segment)
		if err != nil || index < 0 || index >= len(holder) {
			return nil, fmt.Errorf("the json path %q: %q is not an index of the %d item array", path, segment, len(holder))
		}

		return holder[index], nil
	default:
		return nil, fmt.Errorf("the json path %q: %q reaches into a %T which holds no field", path, segment, current)
	}
}

func render(value any, path string) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(typed), nil
	default:
		return "", fmt.Errorf("the json path %q answers a %T and an environment variable takes a string, a number or a bool", path, value)
	}
}

func keysOf(holder map[string]any) string {
	if len(holder) == 0 {
		return "none"
	}

	keys := make([]string, 0, len(holder))
	for key := range holder {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	return strings.Join(keys, ", ")
}
