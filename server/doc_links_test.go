// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The links the app shows (the System Console, the bot's messages, the web app) go to Antimatter's
// docs, not to the dead docs.antimatter.example of the rebrand.
func TestNoDeadDocsLinks(t *testing.T) {
	checked := 0
	for _, root := range []string{"../plugin.json", ".", "../assets", "../webapp/src"} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			name := d.Name()
			if strings.HasSuffix(name, "_test.go") || strings.Contains(name, ".test.") {
				return nil
			}
			switch filepath.Ext(name) {
			case ".go", ".json", ".md", ".ts", ".tsx", ".js", ".jsx":
			default:
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			checked++
			require.False(t, strings.Contains(string(data), "docs.antimatter.example"), "%s links to docs.antimatter.example", path)
			return nil
		})
		if os.IsNotExist(err) {
			continue
		}
		require.NoError(t, err)
	}
	require.Greater(t, checked, 1)
}
