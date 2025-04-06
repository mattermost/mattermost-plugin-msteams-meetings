// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"errors"
	"fmt"
	"strings"
)

const (
	stateLength                  = 4
	trueString                   = "true"
	msteamsMeetingStateKeyPrefix = "msteamsmeetinguserstate"
)

func (p *Plugin) StoreState(userID, channelID string, justConnect bool) (string, error) {
	key := getOAuthUserStateKey(userID)
	state := fmt.Sprintf("%v_%v_%v", key, channelID, justConnect)

	appErr := p.API.KVSet(key, []byte(state))
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

	if len(stateComponents) != stateLength {
		p.API.LogDebug("complete oauth, state mismatch", "stateComponents", fmt.Sprintf("%v", stateComponents), "state", state)
		return "", "", "", false, errors.New("status mismatch")
	}

	key = fmt.Sprintf("%v_%v", stateComponents[0], stateComponents[1])
	userID = stateComponents[1]
	channelID = stateComponents[2]
	justConnect = stateComponents[3] == trueString

	return key, userID, channelID, justConnect, nil
}

// getOAuthUserStateKey generates and returns the key for storing the OAuth user state in the KV store.
func getOAuthUserStateKey(userID string) string {
	return fmt.Sprintf("%v_%v", msteamsMeetingStateKeyPrefix, userID)
}
