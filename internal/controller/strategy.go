/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import "fmt"

// Strategy defines the behavior when target secret already exists
type Strategy string

const (
	// StrategyOverwrite updates existing secrets (default)
	StrategyOverwrite Strategy = "overwrite"
	// StrategyIgnore skips existing secrets without updating
	StrategyIgnore Strategy = "ignore"
)

// ParseStrategy parses and validates strategy from annotation value.
// Returns StrategyOverwrite if value is empty.
func ParseStrategy(value string) (Strategy, error) {
	if value == "" {
		return StrategyOverwrite, nil
	}
	s := Strategy(value)
	if s != StrategyOverwrite && s != StrategyIgnore {
		return "", fmt.Errorf("invalid strategy %q, expected %q or %q", value, StrategyOverwrite, StrategyIgnore)
	}
	return s, nil
}
