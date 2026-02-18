package cmd

import _ "embed"

// skillTemplateEmbedded is the built-in SKILL.md template distributed with the binary.
//
//go:embed skill_template.md
var skillTemplateEmbedded []byte
