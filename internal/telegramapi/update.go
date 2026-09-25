package telegramapi

import (
	"encoding/json"
	"strconv"
	"strings"

	"steadwell/internal/channel"
	"steadwell/internal/replyfmt"
)

type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    *User    `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

type Message struct {
	MessageID int     `json:"message_id"`
	From      *User   `json:"from"`
	Chat      *Chat   `json:"chat"`
	Text      string  `json:"text"`
	Caption   string  `json:"caption"`
	Photo     []Photo `json:"photo"`
	Voice     *File   `json:"voice"`
	Audio     *File   `json:"audio"`
	Video     *File   `json:"video"`
	VideoNote *File   `json:"video_note"`
}

type Photo struct {
	FileID string `json:"file_id"`
}

type File struct {
	FileID string `json:"file_id"`
}

type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
}

type Chat struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
}

func ParseJSON(raw []byte) (Update, error) {
	var u Update
	err := json.Unmarshal(raw, &u)
	return u, err
}

func Inbound(u Update) channel.Inbound {
	in := channel.Inbound{
		Identity:  channel.Identity{Channel: channel.Telegram},
		UpdateID:  u.UpdateID,
		MediaKind: channel.MediaText,
	}
	if u.CallbackQuery != nil {
		cb := u.CallbackQuery
		in.CallbackID = cb.ID
		in.Text = replyfmt.CallbackUserText(cb.Data)
		if cb.From != nil {
			in.Identity.ParticipantID = strconv.FormatInt(cb.From.ID, 10)
			in.Identity.DisplayName = cb.From.FirstName
		}
		if cb.Message != nil && cb.Message.Chat != nil {
			in.ChatID = strconv.FormatInt(cb.Message.Chat.ID, 10)
			in.MessageID = cb.Message.MessageID
		}
		return in
	}
	if u.Message == nil {
		return in
	}
	msg := u.Message
	in.MessageID = msg.MessageID
	in.Text = strings.TrimSpace(msg.Text)
	if in.Text == "" {
		in.Text = strings.TrimSpace(msg.Caption)
	}
	in.MediaKind = mediaKind(msg)
	if msg.From != nil {
		in.Identity.ParticipantID = strconv.FormatInt(msg.From.ID, 10)
		in.Identity.DisplayName = msg.From.FirstName
	}
	if msg.Chat != nil {
		in.ChatID = strconv.FormatInt(msg.Chat.ID, 10)
		if in.Identity.DisplayName == "" {
			in.Identity.DisplayName = msg.Chat.FirstName
		}
	}
	return in
}

func mediaKind(msg *Message) string {
	if msg == nil {
		return channel.MediaText
	}
	if len(msg.Photo) > 0 {
		return channel.MediaImage
	}
	if msg.Voice != nil || msg.Audio != nil {
		return channel.MediaAudio
	}
	if msg.Video != nil || msg.VideoNote != nil {
		return channel.MediaVideo
	}
	return channel.MediaText
}
