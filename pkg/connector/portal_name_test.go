package connector

import (
	"testing"

	"github.com/stretchr/testify/require"
	"maunium.net/go/mautrix/bridgev2"

	"go.mau.fi/mautrix-telegram/pkg/gotd/tg"
)

func TestFormatPortalName(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		suffix string
		input  string
		want   string
	}{
		{
			name:  "empty prefix and suffix",
			input: "Рабочий чат",
			want:  "Рабочий чат",
		},
		{
			name:   "suffix only",
			suffix: " (tg)",
			input:  "Рабочий чат",
			want:   "Рабочий чат (tg)",
		},
		{
			name:   "prefix only",
			prefix: "[TG] ",
			input:  "Рабочий чат",
			want:   "[TG] Рабочий чат",
		},
		{
			name:   "prefix and suffix",
			prefix: "[TG] ",
			suffix: " (tg)",
			input:  "Рабочий чат",
			want:   "[TG] Рабочий чат (tg)",
		},
		{
			name:   "no duplicate suffix",
			suffix: " (tg)",
			input:  "Рабочий чат (tg)",
			want:   "Рабочий чат (tg)",
		},
		{
			name:   "no duplicate prefix",
			prefix: "[TG] ",
			input:  "[TG] Рабочий чат",
			want:   "[TG] Рабочий чат",
		},
		{
			name:   "forum full name",
			suffix: " (tg)",
			input:  "Topic - Forum Group",
			want:   "Topic - Forum Group (tg)",
		},
		{
			name:   "forum full name with prefix and suffix",
			prefix: "[TG] ",
			suffix: " (tg)",
			input:  "Topic - Forum Group",
			want:   "[TG] Topic - Forum Group (tg)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := &TelegramClient{
				main: &TelegramConnector{
					Config: TelegramConfig{
						PortalNamePrefix: tt.prefix,
						PortalNameSuffix: tt.suffix,
					},
				},
			}
			require.Equal(t, tt.want, tc.formatPortalName(tt.input))
		})
	}
}

func TestPortalNameFormatAfterTopicOverride(t *testing.T) {
	baseName := "Forum Group"
	info := &bridgev2.ChatInfo{Name: &baseName}
	tc := &TelegramClient{
		main: &TelegramConnector{
			Config: TelegramConfig{
				PortalNamePrefix: "[TG] ",
				PortalNameSuffix: " (tg)",
			},
		},
	}

	tc.overrideChatInfoWithTopic(info, &tg.ForumTopic{Title: "Topic"})
	tc.applyPortalNameFormat(info)

	require.Equal(t, "[TG] Topic - Forum Group (tg)", *info.Name)
}
