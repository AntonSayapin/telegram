// mautrix-telegram - A Matrix-Telegram puppeting bridge.
// Copyright (C) 2026 Tulir Asokan
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go.mau.fi/mautrix-telegram/pkg/connector/ids"
)

const (
	PeerFilterOverrideAllow = "allow"
	PeerFilterOverrideDeny  = "deny"
)

const (
	getPeerFilterOverrideQuery = `
		SELECT state FROM telegram_peer_filter_override
		WHERE telegram_user_id=$1 AND peer_type=$2 AND peer_id=$3
	`
	setPeerFilterOverrideQuery = `
		INSERT INTO telegram_peer_filter_override
			(telegram_user_id, peer_type, peer_id, state, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
		ON CONFLICT (telegram_user_id, peer_type, peer_id)
		DO UPDATE SET state=excluded.state, updated_at=excluded.updated_at
	`
	clearPeerFilterOverrideQuery = `
		DELETE FROM telegram_peer_filter_override
		WHERE telegram_user_id=$1 AND peer_type=$2 AND peer_id=$3
	`
)

func validatePeerFilterOverrideState(state string) error {
	switch state {
	case PeerFilterOverrideAllow, PeerFilterOverrideDeny:
		return nil
	default:
		return fmt.Errorf("invalid peer filter override state %q", state)
	}
}

func (s *ScopedStore) GetPeerFilterOverride(
	ctx context.Context,
	peerType ids.PeerType,
	peerID int64,
) (state string, ok bool, err error) {
	err = s.db.QueryRow(ctx, getPeerFilterOverrideQuery, s.telegramUserID, peerType, peerID).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if err = validatePeerFilterOverrideState(state); err != nil {
		return "", false, err
	}
	return state, true, nil
}

func (s *ScopedStore) SetPeerFilterOverride(
	ctx context.Context,
	peerType ids.PeerType,
	peerID int64,
	state string,
) error {
	if err := validatePeerFilterOverrideState(state); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx, setPeerFilterOverrideQuery, s.telegramUserID, peerType, peerID, state, time.Now().Unix())
	return err
}

func (s *ScopedStore) ClearPeerFilterOverride(
	ctx context.Context,
	peerType ids.PeerType,
	peerID int64,
) error {
	_, err := s.db.Exec(ctx, clearPeerFilterOverrideQuery, s.telegramUserID, peerType, peerID)
	return err
}
