package entitlements

import (
	"steadwell/internal/store"
)

func Allows(cat store.Cataloger, orgID, mediaKind string) bool {
	if cat == nil {
		return false
	}
	return cat.FeatureEnabled(orgID, store.FeatureKeyForMedia(mediaKind))
}
