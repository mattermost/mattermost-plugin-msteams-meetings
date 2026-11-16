// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	stateLength                  = 4
	trueString                   = "true"
	msteamsMeetingStateKeyPrefix = "msteamsmeetinguserstate"

	oauthStateNonceBytes     = 32
	oauthStateNonceSeparator = "."
	oauthStateExpiry         = 10 * time.Minute
)

func (p *Plugin) StoreState(userID, channelID string, justConnect bool) (string, error) {
	nonce, err := generateOAuthStateNonce()
	if err != nil {
		return "", err
	}

	key := getOAuthUserStateKey(userID)
	state := fmt.Sprintf("%v_%v_%v%v%v", key, channelID, justConnect, oauthStateNonceSeparator, nonce)

	appErr := p.API.KVSetWithExpiry(key, []byte(state), int64(oauthStateExpiry.Seconds()))
	if appErr != nil {
		return "", appErr
	}

	return state, nil
}

func (p *Plugin) GetState(key string) (string, error) {
	storedState, appErr := p.API.KVGet(key)
	if appErr != nil {
		return "", appErr
	}
	return string(storedState), nil
}

func (p *Plugin) DeleteState(key string) error {
	appErr := p.API.KVDelete(key)
	if appErr != nil {
		return appErr
	}
	return nil
}

func (p *Plugin) ParseState(state string) (key, userID, channelID string, justConnect bool, err error) {
	stateComponents := strings.Split(state, "_")

	if len(stateComponents) != stateLength || stateComponents[0] != msteamsMeetingStateKeyPrefix {
		p.API.LogDebug("complete oauth, state mismatch", "stateComponents", fmt.Sprintf("%v", stateComponents), "state", state)
		return "", "", "", false, errors.New("status mismatch")
	}

	justConnectValue, nonce, found := strings.Cut(stateComponents[3], oauthStateNonceSeparator)
	if !found || !isValidOAuthStateNonce(nonce) {
		p.API.LogDebug("complete oauth, state mismatch", "stateComponents", fmt.Sprintf("%v", stateComponents), "state", state)
		return "", "", "", false, errors.New("status mismatch")
	}

	key = fmt.Sprintf("%v_%v", stateComponents[0], stateComponents[1])
	userID = stateComponents[1]
	channelID = stateComponents[2]
	justConnect = justConnectValue == trueString

	return key, userID, channelID, justConnect, nil
}

func isValidOAuthStateNonce(nonce string) bool {
	decoded, err := hex.DecodeString(nonce)
	return err == nil && len(decoded) == oauthStateNonceBytes
}

func generateOAuthStateNonce() (string, error) {
	nonce := make([]byte, oauthStateNonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate oauth state nonce: %w", err)
	}
	return hex.EncodeToString(nonce), nil
}

// getOAuthUserStateKey generates and returns the key for storing the OAuth user state in the KV store.
func getOAuthUserStateKey(userID string) string {
	return fmt.Sprintf("%v_%v", msteamsMeetingStateKeyPrefix, userID)
}
