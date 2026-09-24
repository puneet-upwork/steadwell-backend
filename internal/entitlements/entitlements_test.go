package entitlements

import (
	"testing"

	"steadwell/internal/channel"
	"steadwell/internal/store"
)

func TestDefaultOrgAllowsTextOnly(t *testing.T) {
	cat := store.NewSeeded()
	orgID := cat.DefaultOrgID()

	if !Allows(cat, orgID, channel.MediaText) {
		t.Fatal("text should be allowed on default/free")
	}
	if Allows(cat, orgID, channel.MediaImage) {
		t.Fatal("image should be off on default/free")
	}
	if Allows(cat, orgID, channel.MediaAudio) {
		t.Fatal("audio should be off on default/free")
	}
	if Allows(cat, orgID, channel.MediaVideo) {
		t.Fatal("video should be off on default/free")
	}
}

func TestOrgOverrideEnablesImage(t *testing.T) {
	cat := store.NewSeeded()
	orgID := cat.DefaultOrgID()
	cat.SetOrgFeature(orgID, channel.FeatureImage, true)

	if !Allows(cat, orgID, channel.MediaImage) {
		t.Fatal("org override should enable image")
	}
	if !Allows(cat, orgID, channel.MediaText) {
		t.Fatal("text still allowed")
	}
}

func TestOrgOverrideEnablesAudioAndVideo(t *testing.T) {
	cat := store.NewSeeded()
	orgID := cat.DefaultOrgID()
	cat.SetOrgFeature(orgID, channel.FeatureAudio, true)
	cat.SetOrgFeature(orgID, channel.FeatureVideo, true)

	if !Allows(cat, orgID, channel.MediaAudio) {
		t.Fatal("audio override")
	}
	if !Allows(cat, orgID, channel.MediaVideo) {
		t.Fatal("video override")
	}
}
