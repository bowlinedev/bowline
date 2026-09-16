package watch

import (
	"cmp"
	"context"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type Change struct {
	Path    string
	Removed bool
}

type stamp struct {
	mod  time.Time
	size int64
}

func Run(ctx context.Context, root string, interval time.Duration, fn func([]Change)) {
	previous := snapshot(root)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		current := snapshot(root)
		changes := diff(previous, current)
		if len(changes) == 0 {
			continue
		}
		time.Sleep(interval / 2)
		settled := snapshot(root)
		changes = diff(previous, settled)
		previous = settled
		if len(changes) > 0 {
			fn(changes)
		}
	}
}

func snapshot(root string) map[string]stamp {
	out := map[string]stamp{}
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (name == "node_modules" || name == "vendor" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		out[path] = stamp{mod: info.ModTime(), size: info.Size()}
		return nil
	})
	return out
}

func diff(previous, current map[string]stamp) []Change {
	var changes []Change
	for path, st := range current {
		old, ok := previous[path]
		if !ok || old != st {
			changes = append(changes, Change{Path: path})
		}
	}
	for path := range previous {
		if _, ok := current[path]; !ok {
			changes = append(changes, Change{Path: path, Removed: true})
		}
	}
	slices.SortFunc(changes, func(a, b Change) int { return cmp.Compare(a.Path, b.Path) })
	return changes
}
