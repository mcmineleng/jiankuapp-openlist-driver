package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Source struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type App struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Sources     []Source `json:"sources"`
}

type Input struct {
	Query      string `json:"query"`
	RequestURL string `json:"request_url"`
	Page       int    `json:"page"`
	TotalPages int    `json:"total_pages"`
	TotalItems int    `json:"total_items"`
	HasNext    bool   `json:"has_next"`
	Apps       []App  `json:"apps"`
}

type AllInput struct {
	Query      string  `json:"query"`
	TotalPages int     `json:"total_pages"`
	TotalItems int     `json:"total_items"`
	Pages      []Input `json:"pages"`
	AllApps    []App   `json:"all_apps"`
}

func buildResult(apps []App) map[string]map[string]string {
	result := make(map[string]map[string]string)

	for _, app := range apps {
		name := fmt.Sprintf("%s(%s)", app.Title, app.ID)
		entry := make(map[string]string)

		if app.Description != "" {
			entry["README"] = app.Description
		}

		for _, s := range app.Sources {
			if s.Name != "" {
				entry[s.Name] = s.URL
			}
		}

		result[name] = entry
	}

	return result
}

func emit(result map[string]map[string]string) {
	out, err := json.MarshalIndent([]map[string]map[string]string{result}, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "序列化失败:", err)
		os.Exit(1)
	}

	fmt.Println(string(out))
}

func main() {
	dec := json.NewDecoder(os.Stdin)

	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		fmt.Fprintln(os.Stderr, "解析输入失败:", err)
		os.Exit(1)
	}

	var all AllInput
	if err := json.Unmarshal(raw, &all); err == nil && len(all.Pages) > 0 {
		apps := all.AllApps
		if len(apps) == 0 {
			for _, p := range all.Pages {
				apps = append(apps, p.Apps...)
			}
		}
		emit(buildResult(apps))
		return
	}

	var input Input
	if err := json.Unmarshal(raw, &input); err != nil {
		fmt.Fprintln(os.Stderr, "解析输入失败:", err)
		os.Exit(1)
	}

	emit(buildResult(input.Apps))
}
