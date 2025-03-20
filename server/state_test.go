// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestStoreState(t *testing.T) {
	testCases := []struct {
		name           string
		userID         string
		channelID      string
		justConnect    bool<<<<<<< MM-822
2
 
=======
		returnError    error
		expectError    bool
		expectedState  string
		expectedErrMsg string
	}{
		{
			name:           "Error occurred while storing state",
			userID:         "mockUserID1",
			channelID:      "mockChannelID1",
			returnError:    &model.AppError{Message: "error occurred while storing state"},
			expectError:    true,
			expectedErrMsg: "error occurred while storing state",
		},
		{
			name:          "Store state successful",
			userID:        "mockUserID2",
			channelID:     "mockChannelID2",
			justConnect:   true,
			expectedState: "msteamsmeetinguserstate_mockUserID2_mockChannelID2_true",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI := &plugintest.API{}
			p := SetupMockPlugin(mockAPI, nil, nil)

			mockAPI.On("KVSet", getOAuthUserStateKey(tc.userID), mock.Anything).Return(tc.returnError)

			state, err := p.StoreState(tc.userID, tc.channelID, tc.justConnect)

			if tc.expectError {
				require.Error(t, err)
				require.Equal(t, tc.expectedErrMsg, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedState, state)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

func TestGetState(t *testing.T) {
	testCases := []struct {
		name           string
		key            string
		getStateValue  []byte
		getStateError  error
		expectedState  string
		expectError    bool
		expectedErrMsg string
	}{
		{
			name:           "Error occurred while getting stored state",
			key:            "mockKey",
			getStateValue:  []byte(""),
			getStateError:  &model.AppError{Message: "error occurred while getting stored state"},
			expectError:    true,
			expectedErrMsg: "error occurred while getting stored state",
		},
		{
			name:          "Valid state retrieved",
			key:           "mockKey",
			getStateValue: []byte("mockState"),
			expectedState: "mockState",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			api := &plugintest.API{}
			p := SetupMockPlugin(api, nil, nil)

			api.On("KVGet", tc.key).Return(tc.getStateValue, tc.getStateError)

			state, err := p.GetState(tc.key)
			if tc.expectError {
				require.Error(t, err)
				require.Equal(t, tc.expectedState, state)
				require.Equal(t, tc.expectedErrMsg, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedState, state)
			}
			api.AssertExpectations(t)
		})
	}
}

func TestDeleteState(t *testing.T) {
	testCases := []struct {
		name           string
		key            string
		returnError    error
		expectError    bool
		expectedErrMsg string
	}{
		{
			name:           "Error occurred while deleting state",
			key:            "mockKey",
			returnError:    &model.AppError{Message: "error occurred while deleting state"},
			expectError:    true,
			expectedErrMsg: "error occurred while deleting state",
		},
		{
			name: "Delete state successful",
			key:  "mockKey",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI := &plugintest.API{}
			p := SetupMockPlugin(mockAPI, nil, nil)

			mockAPI.On("KVDelete", tc.key).Return(tc.returnError)

			err := p.DeleteState(tc.key)
			if tc.expectError {
				require.Error(t, err)
				require.Equal(t, tc.expectedErrMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

func TestParseState(t *testing.T) {
	testCases := []struct {
		name                string
		state               string
		expectedKey         string
		expectedUserID      string
		expectedChannelID   string
		expectedJustConnect bool
		expectError         bool
		expectedErrMsg      string
	}{
		{
			name:           "State length mismatch",
			state:          "key1_userID1_channelID1",
			expectError:    true,
			expectedErrMsg: "status mismatch",
		},
		{
			name:           "Invalid state format",
			state:          "key1_userID1",
			expectError:    true,
			expectedErrMsg: "status mismatch",
		},
		{
			name:                "Parse state successful",
			state:               "key1_userID1_channelID1_true",
			expectedKey:         "key1_userID1",
			expectedUserID:      "userID1",
			expectedChannelID:   "channelID1",
			expectedJustConnect: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAPI := &plugintest.API{}
			p := SetupMockPlugin(mockAPI, nil, nil)

			if tc.expectError {
				mockAPI.On("LogDebug", "complete oauth, state mismatch", "stateComponents", mock.Anything, "state", tc.state).Return()
			}

			key, userID, channelID, justConnect, err := p.ParseState(tc.state)

			if tc.expectError {
				require.Error(t, err)
				require.Equal(t, tc.expectedErrMsg, err.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedKey, key)
				require.Equal(t, tc.expectedUserID, userID)
				require.Equal(t, tc.expectedChannelID, channelID)
				require.Equal(t, tc.expectedJustConnect, justConnect)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

func SetupMockPlugin(mockAPI *plugintest.API, mockTracker *MockTracker, mockClient *MockClient) *Plugin {
	return &Plugin{
		MattermostPlugin: plugin.MattermostPlugin{
			API: mockAPI,
		},
		tracker: mockTracker,
		client:  mockClient,
	}
}
