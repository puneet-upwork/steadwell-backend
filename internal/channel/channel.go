package channel

const (
	Telegram = "telegram"
	WhatsApp = "whatsapp"
	Line     = "line"
)

const (
	MediaText  = "text"
	MediaImage = "image"
	MediaAudio = "audio"
	MediaVideo = "video"
)

const (
	FeatureText  = "text_messages"
	FeatureImage = "image_messages"
	FeatureAudio = "audio_messages"
	FeatureVideo = "video_messages"
)

const (
	DefaultOrgSlug  = "steadwell"
	DefaultPlanSlug = "free"
)

// Identity is the channel-agnostic user key.
type Identity struct {
	Channel       string
	ParticipantID string
	DisplayName   string
}

func (id Identity) Key() (channel, participantID string) {
	return id.Channel, id.ParticipantID
}

// Inbound is one user event from any messenger.
type Inbound struct {
	Identity  Identity
	ChatID    string
	MessageID int
	Text      string
	MediaKind string
	UpdateID  int64
}

// Reply is a channel-agnostic outbound message.
type Reply struct {
	MessageBody string `json:"message_body,omitempty"`
	SkipSend    bool   `json:"skip_send,omitempty"`
}
