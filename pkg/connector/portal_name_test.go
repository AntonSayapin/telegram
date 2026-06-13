package connector

import (
	"testing"

	"github.com/stretchr/testify/require"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"

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

func TestDMPortalNameFormatFromGhost(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		suffix     string
		ghostName  string
		wantName   *string
		wantGhost  string
		preSetName *string
	}{
		{
			name:      "suffix only",
			suffix:    " (tg)",
			ghostName: "John Doe",
			wantName:  ptrString("John Doe (tg)"),
			wantGhost: "John Doe",
		},
		{
			name:      "prefix and suffix",
			prefix:    "[TG] ",
			suffix:    " (tg)",
			ghostName: "John Doe",
			wantName:  ptrString("[TG] John Doe (tg)"),
			wantGhost: "John Doe",
		},
		{
			name:      "no duplicate suffix",
			suffix:    " (tg)",
			ghostName: "John Doe (tg)",
			wantName:  ptrString("John Doe (tg)"),
			wantGhost: "John Doe (tg)",
		},
		{
			name:      "no configured format leaves implicit DM name",
			ghostName: "John Doe",
			wantGhost: "John Doe",
		},
		{
			name:       "pre-set name is preserved",
			suffix:     " (tg)",
			ghostName:  "John Doe",
			preSetName: ptrString("Telegram Saved Messages"),
			wantName:   ptrString("Telegram Saved Messages (tg)"),
			wantGhost:  "John Doe",
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
			info := &bridgev2.ChatInfo{Name: tt.preSetName}
			ghost := &bridgev2.Ghost{Ghost: &database.Ghost{Name: tt.ghostName}}

			tc.setDMPortalNameFromGhost(info, ghost)
			tc.applyPortalNameFormat(info)

			if tt.wantName == nil {
				require.Nil(t, info.Name)
			} else {
				require.NotNil(t, info.Name)
				require.Equal(t, *tt.wantName, *info.Name)
			}
			require.Equal(t, tt.wantGhost, ghost.Name)
		})
	}
}

func ptrString(val string) *string {
	return &val
}
