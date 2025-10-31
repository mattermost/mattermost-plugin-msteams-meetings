// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"errors"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	msgraph "github.com/yaegashi/msgraph.go/beta"
)

var mockPost = mock.AnythingOfType("*model.Post")

func TestGetUserInfo(t *testing.T) {
	p, api, client := SetupPluginMocks()

	info := &UserInfo{
		Email: "testEmail",
	}
	p.setConfiguration(&configuration{
		EncryptionKey: "demo_encrypt_key",
	})
	key := []byte(p.getConfiguration().EncryptionKey)
	encryptedUserInfo, err := info.EncryptedJSON(key)
	mockJoinURL := "testJoinURL"
	require.NoError(t, err)

	tests := []struct {
		name          string
		creator       *model.User
		expectedError string
		setup         func()
	}{
		{
			name:          "User not connected",
			creator:       &model.User{Id: "testUserID"},
			expectedError: "Your Mattermost account is not connected to any Microsoft Teams account",
			setup: func() {
				api.On("KVGet", tokenKey+"testUserID").Return(nil, nil).Once()
			},
		},
		{
			name:          "Unauthorized to create post in channel",
			creator:       &model.User{Id: "testUserID"},
			expectedError: "cannot create post in this channel",
			setup: func() {
				api.On("KVGet", tokenKey+"testUserID").Return(encryptedUserInfo, nil).Once()
				api.On("HasPermissionToChannel", "testUserID", "testChannelID", model.PermissionCreatePost).Return(false)
			},
		},
		{
			name:          "Error getting the channel",
			creator:       &model.User{Id: "testUserID"},
			expectedError: "error occurred getting the channel",
			setup: func() {
				api.On("KVGet", tokenKey+"testUserID").Return(encryptedUserInfo, nil).Once()
				api.On("HasPermissionToChannel", "testUserID", "testChannelID", model.PermissionCreatePost).Return(true)
				api.On("GetChannel", "testChannelID").Return(nil, &model.AppError{Message: "error occurred getting the channel"})
			},
		},
		{
			name:          "Error getting channel members",
			creator:       &model.User{Id: "testUserID"},
			expectedError: "error occurred getting channel members",
			setup: func() {
				api.On("KVGet", tokenKey+"testUserID").Return(encryptedUserInfo, nil).Once()
				api.On("HasPermissionToChannel", "testUserID", "testChannelID", model.PermissionCreatePost).Return(true)
				api.On("GetChannel", "testChannelID").Return(&model.Channel{Id: "testChannelID", Type: model.ChannelTypeDirect}, nil)
				api.On("GetChannelMembers", "testChannelID", 0, 100).Return(nil, &model.AppError{Message: "error occurred getting channel members"})
			},
		},
		{
			name:          "Channel members is nil",
			creator:       &model.User{Id: "testUserID"},
			expectedError: "returned members is nil",
			setup: func() {
				api.On("KVGet", tokenKey+"testUserID").Return(encryptedUserInfo, nil).Once()
				api.On("HasPermissionToChannel", "testUserID", "testChannelID", model.PermissionCreatePost).Return(true)
				api.On("GetChannel", "testChannelID").Return(&model.Channel{Id: "testChannelID", Type: model.ChannelTypeDirect}, nil)
				api.On("GetChannelMembers", "testChannelID", 0, 100).Return(nil, nil)
			},
		},
		{
			name:          "Error creating the meeting",
			creator:       &model.User{Id: "testUserID"},
			expectedError: "error creating the meeting",
			setup: func() {
				api.On("KVGet", tokenKey+"testUserID").Return(encryptedUserInfo, nil).Once()
				api.On("HasPermissionToChannel", "testUserID", "testChannelID", model.PermissionCreatePost).Return(true)
				api.On("GetChannel", "testChannelID").Return(&model.Channel{Id: "testChannelID", Type: model.ChannelTypeDirect}, nil)
				api.On("GetChannelMembers", "testChannelID", 0, 100).Return(model.ChannelMembers{}, nil)
				client.On("CreateMeeting").Return(&msgraph.OnlineMeeting{}, errors.New("error creating the meeting"))
			},
		},
		{
			name:          "Error creating the meeting post",
			creator:       &model.User{Id: "testUserID", Username: "testUsername"},
			expectedError: "error creating the post",
			setup: func() {
				require.NoError(t, err)
				api.On("KVGet", tokenKey+"testUserID").Return(encryptedUserInfo, nil).Once()
				api.On("HasPermissionToChannel", "testUserID", "testChannelID", model.PermissionCreatePost).Return(true)
				api.On("GetChannel", "testChannelID").Return(&model.Channel{Id: "testChannelID", Type: model.ChannelTypeDirect}, nil)
				api.On("GetChannelMembers", "testChannelID", 0, 100).Return(model.ChannelMembers{}, nil)
				api.On("CreatePost", mockPost).Return(nil, &model.AppError{Message: "error creating the post"})
				client.On("CreateMeeting").Return(&msgraph.OnlineMeeting{JoinURL: &mockJoinURL}, nil)
			},
		},
		{
			name:    "Meeting posted successfully",
			creator: &model.User{Id: "testUserID", Username: "testUsername"},
			setup: func() {
				api.On("KVGet", tokenKey+"testUserID").Return(encryptedUserInfo, nil).Once()
				api.On("HasPermissionToChannel", "testUserID", "testChannelID", model.PermissionCreatePost).Return(true)
				api.On("GetChannel", "testChannelID").Return(&model.Channel{Id: "testChannelID", Type: model.ChannelTypeDirect}, nil)
				api.On("GetChannelMembers", "testChannelID", 0, 100).Return(model.ChannelMembers{}, nil)
				api.On("CreatePost", mockPost).Return(&model.Post{}, nil)
				client.On("CreateMeeting").Return(&msgraph.OnlineMeeting{JoinURL: &mockJoinURL}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api.ExpectedCalls = nil
			client.ExpectedCalls = nil

			tt.setup()

			_, _, err := p.postMeeting(tt.creator, "testChannelID", "testTopic")

			if tt.expectedError != "" {
				require.Error(t, err)
				require.Equal(t, tt.expectedError, err.Error())
			} else {
				require.NoError(t, err)
			}

			api.AssertExpectations(t)
		})
	}
}

func TestPostConfirmCreateOrJoin(t *testing.T) {
	p, api, _ := SetupPluginMocks()

	testCases := []struct {
		name            string
		provider        string
		expectedMessage string
	}{
		{
			name:            "Provider is msteams",
			provider:        "msteams",
			expectedMessage: "There is another recent meeting created on this channel.",
		},
		{
			name:            "Provider is dummyProvider",
			provider:        "dummyProvider",
			expectedMessage: "There is another recent meeting created on this channel with dummyProvider.",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			channelID := "test_channel_id"
			meetingURL := "https://example.com/meeting"
			topic := "Test Meeting"
			userID := "test_user_id"
			creatorName := "test_creator"

			expectedPost := &model.Post{
				UserId:    p.botUserID,
				ChannelId: channelID,
				Message:   tt.expectedMessage,
				Type:      "custom_mstmeetings",
				Props: map[string]interface{}{
					"type":                     "custom_mstmeetings",
					"meeting_link":             meetingURL,
					"meeting_status":           postTypeConfirm,
					"meeting_personal":         true,
					"meeting_topic":            topic,
					"meeting_creator_username": creatorName,
					"meeting_provider":         tt.provider,
				},
			}
			api.ExpectedCalls = nil
			api.On("SendEphemeralPost", userID, mockPost).Return(expectedPost, nil)

			post := p.postConfirmCreateOrJoin(meetingURL, channelID, topic, userID, creatorName, tt.provider)
			require.Equal(t, tt.expectedMessage, post.Message)
		})
	}
}

func TestPostConnect(t *testing.T) {
	p, api, _ := SetupPluginMocks()

	tests := []struct {
		name          string
		userID        string
		channelID     string
		expectedError string
		setup         func()
	}{
		{
			name:          "Error getting oauth message",
			channelID:     "testChannelID",
			userID:        "testUserID",
			expectedError: "error fetching siteURL",
			setup: func() {
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: nil}})
				api.On("LogError", "postConnect, cannot get oauth message", "error", "error fetching siteURL")
			},
		},
		{
			name:      "Error getting oauth message",
			channelID: "testChannelID",
			userID:    "testUserID",
			setup: func() {
				testSiteURL := "testSiteURL"
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: &testSiteURL}})
				api.On("SendEphemeralPost", "testUserID", mockPost).Return(&model.Post{}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api.ExpectedCalls = nil

			tt.setup()

			_, err := p.postConnect(tt.channelID, tt.userID)

			if tt.expectedError != "" {
				require.Error(t, err)
				require.Equal(t, tt.expectedError, err.Error())
			} else {
				require.NoError(t, err)
			}
			api.AssertExpectations(t)
		})
	}
}

func SetupPluginMocks() (*Plugin, *plugintest.API, *MockClient) {
	api := &plugintest.API{}
	client := &MockClient{}
	p := &Plugin{
		MattermostPlugin: plugin.MattermostPlugin{
			API: api,
		},
		client: client,
	}

	return p, api, client
}
