package prompts

import (
	"strings"

	"steadwell/internal/store"
)

func Assemble(cat store.Cataloger, orgID string) string {
	if cat == nil {
		return ""
	}
	return strings.Join(cat.PromptBodies(orgID), "\n\n")
}
