// Package docs holds the guides the CLI prints (r3v help agents).
package docs

import _ "embed"

// Agents is agents.md: how AI agents use R3V.
//
//go:embed agents.md
var Agents string

// AgentsSnippet is agents-snippet.md: a few lines for a project's AGENTS.md.
//
//go:embed agents-snippet.md
var AgentsSnippet string

// Rules is profiles.md: the project's rules (.r3v.yaml), for r3v help rules.
//
//go:embed profiles.md
var Rules string
