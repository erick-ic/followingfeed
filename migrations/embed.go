package migrations

import (
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed *.sql
var files embed.FS

type Migration struct {
	Version string
	Script  string
}

func All() ([]Migration, error) {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	res := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		script, err := files.ReadFile(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		res = append(res, Migration{
			Version: strings.TrimSuffix(entry.Name(), ".sql"),
			Script:  string(script),
		})
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Version < res[j].Version })
	return res, nil
}
