package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// sortedKeys returns the keys of a field map, sorted — the canonical set of
// valid --json field names for a resource.
func sortedKeys(fieldMap map[string]interface{}) []string {
	keys := make([]string, 0, len(fieldMap))
	for k := range fieldMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// parseJSONFields validates a comma-separated --json field list against valid.
// An empty string, "*" or "all" means "all fields" (nil map).
func parseJSONFields(fields string, valid []string) (map[string]bool, error) {
	if fields == "" || fields == "*" || fields == "all" {
		return nil, nil
	}
	validSet := map[string]bool{}
	for _, k := range valid {
		validSet[k] = true
	}
	requested := map[string]bool{}
	for _, f := range strings.Split(fields, ",") {
		name := strings.TrimSpace(f)
		if name == "" {
			continue
		}
		if !validSet[name] {
			return nil, fmt.Errorf("unknown --json field %q (valid: %s)", name, strings.Join(valid, ", "))
		}
		requested[name] = true
	}
	return requested, nil
}

// printJSON marshals rows to indented JSON, keeping only requested fields
// (nil/empty requested = all fields).
func printJSON(rows []map[string]interface{}, requested map[string]bool) error {
	if len(requested) > 0 {
		for i, row := range rows {
			filtered := map[string]interface{}{}
			for k, v := range row {
				if requested[k] {
					filtered[k] = v
				}
			}
			rows[i] = filtered
		}
	}
	out, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

// printJSONObject marshals one object to indented JSON, keeping only requested
// fields (nil/empty requested = all fields).
func printJSONObject(row map[string]interface{}, requested map[string]bool) error {
	if len(requested) > 0 {
		filtered := map[string]interface{}{}
		for k, v := range row {
			if requested[k] {
				filtered[k] = v
			}
		}
		row = filtered
	}
	out, err := json.MarshalIndent(row, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}
