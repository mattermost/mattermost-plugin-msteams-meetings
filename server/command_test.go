// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	msgraph "github.com/yaegashi/msgraph.go/beta"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/pluginapi/experimental/telemetry"
)

type MockTracker struct {
	mock.Mock
}

func (m *MockTracker) TrackEvent(event string, properties map[string]interface{}) error {
	args := m.Called(event, properties)
	return args.Error(0)
}

func (m *MockTracker) TrackError(err error, properties map[string]interface{}) {
	m.Called(err, properties)
}

func (m *MockTracker) ReloadConfig(telemetry.TrackerConfig) {
	m.Called()
}

func (m *MockTracker) TrackUserEvent(userID string, event string, properties map[string]interface{}) error {
	args := m.Called(userID, event, properties)
	return args.Error(0)
}

type MockClient struct {
	mock.Mock
}

func (m *MockClient) GetMe() (*msgraph.User, error) {
	args := m.Called()
	return args.Get(0).(*msgraph.User), args.Error(1)
}

func (m *MockClient) CreateMeeting(_ *UserInfo, _ []*UserInfo, _ string) (*msgraph.OnlineMeeting, error) {
	args := m.Called()
	return args.Get(0).(*msgraph.OnlineMeeting), args.Error(1)
}

// mockClientFactory returns a ClientFactory that always returns the given mock client
func mockClientFactory(mockClient *MockClient) ClientFactory {
	return func(_ *oauth2.Config, _ *oauth2.Token) ClientInterface {
		return mockClient
	}
}

func TestHandleConnect(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		commandArgs    *model.CommandArgs
		mockSetup      func(api *plugintest.API, encryptedUserInfo []byte, mockClient *MockClient)
		expectedOutput string
		expectError    bool
		expectedError  string
	}{
		{
			name:        "Too many parameters",
			args:        []string{"param1", "param2"},
			commandArgs: &model.CommandArgs{UserId: "demoUserID"},
			mockSetup: func(_ *plugintest.API, _ []byte, _ *MockClient) {
			},
			expectedOutput: tooManyParametersText,
			expectError:    false,
		},
		{
			name:        "Error connecting user",
			args:        []string{"param"},
			commandArgs: &model.CommandArgs{UserId: "demoUserID", ChannelId: "demoChannelID"},
			mockSetup: func(api *plugintest.API, encryptedUserInfo []byte, mockClient *MockClient) {
				api.On("KVGet", "token_demoUserID").Return(encryptedUserInfo, nil)
				api.On("KVSet", "msteamsmeetinguserstate_demoUserID", []byte("msteamsmeetinguserstate_demoUserID_demoChannelID_true")).Return(nil)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewPointer("https://example.com")}})
				mockClient.On("GetMe").Return(&msgraph.User{}, errors.New("error getting user details"))
			},
			expectedOutput: "",
			expectError:    true,
			expectedError:  "error getting user details",
		},
		{
			name:        "Successful connection",
			args:        []string{"param"},
			commandArgs: &model.CommandArgs{UserId: "demoUserID", ChannelId: "demoChannelID"},
			mockSetup: func(api *plugintest.API, encryptedUserInfo []byte, mockClient *MockClient) {
				api.On("KVGet", "token_demoUserID").Return(encryptedUserInfo, nil)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewPointer("https://example.com")}})
				mockClient.On("GetMe").Return(&msgraph.User{}, nil)
			},
			expectedOutput: "",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &plugintest.API{}
			mockClient := &MockClient{}

			p := &Plugin{
				MattermostPlugin: plugin.MattermostPlugin{
					API: api,
				},
			}

			p.setConfiguration(&configuration{
				EncryptionKey:      "demo_encrypt_key",
				OAuth2ClientID:     "demo_oauth2_client_id",
				OAuth2ClientSecret: "demo_oauth2_client_secret",
				OAuth2Authority:    "demo_oauth2_authority",
			})

			userInfo := &UserInfo{
				Email:    "dummy@email.com",
				RemoteID: "demo_remote_id",
				UserID:   "dummy_user_id",
				UPN:      "dummy_upn",
			}

			encryptedUserInfo, err := userInfo.EncryptedJSON([]byte("demo_encrypt_key"))
			require.NoError(t, err)

			tt.mockSetup(api, encryptedUserInfo, mockClient)

			resp, err := p.handleConnectWithDeps(tt.args, tt.commandArgs, mockClientFactory(mockClient))
			if tt.expectError {
				require.ErrorContains(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
				require.Contains(t, resp, tt.expectedOutput)
			}

			api.AssertExpectations(t)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestHandleDisconnect(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		commandArgs    *model.CommandArgs
		mockSetup      func(api *plugintest.API, encryptedUserInfo []byte, mockTracker *MockTracker)
		expectedOutput string
	}{
		{
			name:           "Too many parameters",
			args:           []string{"param1", "param2"},
			commandArgs:    &model.CommandArgs{UserId: "demoUserID"},
			mockSetup:      func(_ *plugintest.API, _ []byte, _ *MockTracker) {},
			expectedOutput: tooManyParametersText,
		},
		{
			name:        "Error while disconnecting",
			args:        []string{"param"},
			commandArgs: &model.CommandArgs{UserId: "demoUserID"},
			mockSetup: func(api *plugintest.API, encryptedUserInfo []byte, _ *MockTracker) {
				api.On("KVGet", "token_demoUserID").Return(encryptedUserInfo, nil)
				api.On("KVDelete", "token_demoUserID").Return(&model.AppError{Message: "deletion error"})
				api.On("KVDelete", "tbyrid_demo_remote_id").Return(nil)
			},
			expectedOutput: "Failed to disconnect user, deletion error",
		},
		{
			name:        "Successful disconnection",
			args:        []string{"param"},
			commandArgs: &model.CommandArgs{UserId: "demoUserID"},
			mockSetup: func(api *plugintest.API, encryptedUserInfo []byte, mockTracker *MockTracker) {
				api.On("KVGet", "token_demoUserID").Return(encryptedUserInfo, nil)
				api.On("KVDelete", "token_demoUserID").Return(nil)
				api.On("KVDelete", "tbyrid_demo_remote_id").Return(nil)
				mockTracker.On("TrackUserEvent", "disconnect", "demoUserID", mock.Anything).Return(nil)
			},
			expectedOutput: "You have successfully disconnected from MS Teams Meetings.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &plugintest.API{}
			mockTracker := &MockTracker{}

			p := &Plugin{
				MattermostPlugin: plugin.MattermostPlugin{
					API: api,
				},
				tracker: mockTracker,
			}

			p.setConfiguration(&configuration{
				EncryptionKey: "demo_encrypt_key",
			})

			userInfo := &UserInfo{
				Email:    "dummy@email.com",
				RemoteID: "demo_remote_id",
				UserID:   "dummy_user_id",
				UPN:      "dummy_upn",
			}

			encryptedUserInfo, err := userInfo.EncryptedJSON([]byte("demo_encrypt_key"))
			require.NoError(t, err)

			tt.mockSetup(api, encryptedUserInfo, mockTracker)

			resp, err := p.handleDisconnect(tt.args, tt.commandArgs)
			require.NoError(t, err)
			require.Contains(t, resp, tt.expectedOutput)

			api.AssertExpectations(t)
			mockTracker.AssertExpectations(t)
		})
	}
}

func TestHandleStart(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		commandArgs    *model.CommandArgs
		mockSetup      func(api *plugintest.API, encryptedUserInfo []byte, mockTracker *MockTracker, mockClient *MockClient)
		expectedOutput string
		expectError    bool
		expectedError  string
	}{
		{
			name:        "Error getting user",
			args:        []string{"param1", "param2"},
			commandArgs: &model.CommandArgs{UserId: "demoUserID"},
			mockSetup: func(api *plugintest.API, _ []byte, _ *MockTracker, _ *MockClient) {
				api.On("GetUser", "demoUserID").Return(nil, &model.AppError{Message: "error getting user for the userID"})
			},
			expectError:   true,
			expectedError: "error getting user for the userID",
		},
		{
			name:        "Error getting channel",
			args:        []string{"param1", "param2"},
			commandArgs: &model.CommandArgs{UserId: "demoUserID", ChannelId: "demoChannelID"},
			mockSetup: func(api *plugintest.API, _ []byte, _ *MockTracker, _ *MockClient) {
				api.On("GetUser", "demoUserID").Return(&model.User{Id: "demoUserID"}, nil)
				api.On("GetChannelMember", "demoChannelID", "demoUserID").Return(nil, &model.AppError{Message: "error getting channel for the channelID"})
			},
			expectError:   true,
			expectedError: "error getting channel for the channelID",
		},
		{
			name:        "Error getting previous messages",
			args:        []string{"param1", "param2"},
			commandArgs: &model.CommandArgs{UserId: "demoUserID", ChannelId: "demoChannelID"},
			mockSetup: func(api *plugintest.API, _ []byte, _ *MockTracker, _ *MockClient) {
				api.On("GetUser", "demoUserID").Return(&model.User{Id: "demoUserID"}, nil)
				api.On("GetChannelMember", "demoChannelID", "demoUserID").Return(&model.ChannelMember{ChannelId: "demoChannelID"}, nil)
				api.On("GetPostsSince", "demoChannelID", (time.Now().Unix()-30)*1000).Return(nil, &model.AppError{Message: "error getting previous post for channel"})
			},
			expectError:   true,
			expectedError: "error getting previous post for channel",
		},
		{
			name:        "Recent meeting exists",
			args:        []string{"param1", "param2"},
			commandArgs: &model.CommandArgs{UserId: "demoUserID", ChannelId: "demoChannelID"},
			mockSetup: func(api *plugintest.API, _ []byte, mockTracker *MockTracker, _ *MockClient) {
				postList := &model.PostList{
					Order: []string{"post1"},
					Posts: map[string]*model.Post{
						"post1": {
							Id:        "post1",
							ChannelId: "demoChannelID",
							CreateAt:  time.Now().UnixNano() / int64(time.Millisecond),
							UpdateAt:  time.Now().UnixNano() / int64(time.Millisecond),
							Props: map[string]interface{}{
								"meeting_provider":         "meetingProvider",
								"meeting_link":             "meetingLink",
								"meeting_creator_username": "creatorName",
							},
						},
					},
				}
				api.On("GetUser", "demoUserID").Return(&model.User{Id: "demoUserID"}, nil)
				api.On("GetChannelMember", "demoChannelID", "demoUserID").Return(&model.ChannelMember{ChannelId: "demoChannelID"}, nil)
				api.On("GetPostsSince", "demoChannelID", (time.Now().Unix()-30)*1000).Return(postList, nil)
				api.On("SendEphemeralPost", "demoUserID", mock.Anything).Return(&model.Post{})
				mockTracker.On("TrackUserEvent", mock.Anything, "demoUserID", mock.Anything).Return(nil)
			},
			expectError:    false,
			expectedOutput: "",
		},
		{
			name:        "Authentication error",
			args:        []string{"param1", "param2"},
			commandArgs: &model.CommandArgs{UserId: "demoUserID", ChannelId: "demoChannelID"},
			mockSetup: func(api *plugintest.API, _ []byte, _ *MockTracker, _ *MockClient) {
				api.On("GetUser", "demoUserID").Return(&model.User{Id: "demoUserID"}, nil)
				api.On("GetChannelMember", "demoChannelID", "demoUserID").Return(&model.ChannelMember{ChannelId: "demoChannelID"}, nil)
				postList := &model.PostList{
					Order: []string{"post1"},
					Posts: map[string]*model.Post{
						"post1": {
							Id:        "post1",
							ChannelId: "demoChannelID",
							CreateAt:  time.Now().UnixNano() / int64(time.Millisecond),
							UpdateAt:  time.Now().UnixNano() / int64(time.Millisecond),
						},
					},
				}
				api.On("GetPostsSince", "demoChannelID", (time.Now().Unix()-30)*1000).Return(postList, nil)
				api.On("KVGet", "token_demoUserID").Return(nil, &model.AppError{Message: "deletion error"})
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewPointer("https://example.com")}})
				api.On("KVSet", "msteamsmeetinguserstate_demoUserID", []byte("msteamsmeetinguserstate_demoUserID_demoChannelID_false")).Return(nil)
			},
			expectError:   true,
			expectedError: "Your Mattermost account is not connected to any Microsoft Teams account",
		},
		{
			name:        "Meeting started successfully",
			args:        []string{"param1", "param2"},
			commandArgs: &model.CommandArgs{UserId: "demoUserID", ChannelId: "demoChannelID"},
			mockSetup: func(api *plugintest.API, encryptedUserInfo []byte, mockTracker *MockTracker, mockClient *MockClient) {
				api.On("GetUser", "demoUserID").Return(&model.User{Id: "demoUserID"}, nil)
				api.On("GetChannelMember", "demoChannelID", "demoUserID").Return(&model.ChannelMember{ChannelId: "demoChannelID"}, nil)

				postList := &model.PostList{
					Order: []string{"post1"},
					Posts: map[string]*model.Post{
						"post1": {
							Id:        "post1",
							ChannelId: "demoChannelID",
							CreateAt:  time.Now().UnixNano() / int64(time.Millisecond),
							UpdateAt:  time.Now().UnixNano() / int64(time.Millisecond),
						},
					},
				}

				joinURL := "demoJoinURL"

				api.On("GetPostsSince", "demoChannelID", (time.Now().Unix()-30)*1000).Return(postList, nil)
				api.On("KVGet", "token_demoUserID").Return(encryptedUserInfo, nil)
				api.On("GetConfig").Return(&model.Config{ServiceSettings: model.ServiceSettings{SiteURL: model.NewPointer("https://example.com")}})
				api.On("HasPermissionToChannel", "demoUserID", "demoChannelID", model.PermissionCreatePost).Return(true)
				api.On("GetChannel", "demoChannelID").Return(&model.Channel{Id: "demoChannelID", Type: model.ChannelTypeOpen}, nil)
				api.On("CreatePost", mock.Anything).Return(&model.Post{Id: "demoPostID"}, nil)
				mockClient.On("GetMe").Return(&msgraph.User{}, nil)
				mockClient.On("CreateMeeting", mock.Anything, mock.Anything, mock.Anything).Return(&msgraph.OnlineMeeting{JoinURL: &joinURL}, nil)
				mockTracker.On("TrackUserEvent", "meeting_started", "demoUserID", mock.Anything).Return(nil)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &plugintest.API{}
			mockTracker := &MockTracker{}
			mockClient := &MockClient{}

			p := &Plugin{
				MattermostPlugin: plugin.MattermostPlugin{
					API: api,
				},
				tracker: mockTracker,
			}

			p.setConfiguration(&configuration{
				EncryptionKey: "demo_encrypt_key",
			})

			userInfo := &UserInfo{
				Email:    "dummy@email.com",
				RemoteID: "demo_remote_id",
				UserID:   "dummy_user_id",
				UPN:      "dummy_upn",
			}

			encryptedUserInfo, err := userInfo.EncryptedJSON([]byte("demo_encrypt_key"))
			require.NoError(t, err)

			tt.mockSetup(api, encryptedUserInfo, mockTracker, mockClient)

			resp, err := p.handleStartWithDeps(tt.args, tt.commandArgs, mockClientFactory(mockClient))
			if tt.expectError {
				require.ErrorContains(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
				require.Contains(t, resp, tt.expectedOutput)
			}

			api.AssertExpectations(t)
			mockTracker.AssertExpectations(t)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestGetHelpText(t *testing.T) {
	p := &Plugin{}
	expected := "###### Mattermost MS Teams Meetings Plugin - Slash Command Help\n" +
		"* `/mstmeetings start` - Start an MS Teams meeting. \n" +
		"* `/mstmeetings connect` - Connect to MS Teams meeting. \n" +
		"* `/mstmeetings disconnect` - Disconnect your Mattermost account from MS Teams. \n" +
		"* `/mstmeetings help` - Display this help text."

	actual := p.getHelpText()
	require.Equal(t, expected, actual)
}

func TestExecuteCommand(t *testing.T) {
	mockAPI := &plugintest.API{}
	p := SetupMockPlugin(mockAPI, nil, nil)

	var dummyPluginContext *plugin.Context

	tests := []struct {
		name        string
		commandArgs *model.CommandArgs
		expectedMsg string
	}{
		{
			name: "Invalid Command",
			commandArgs: &model.CommandArgs{
				Command:   "/dummyCommand start",
				ChannelId: "dummyChannelID",
				UserId:    "dummyUserID",
			},
			expectedMsg: "Command '/dummyCommand' is not /mstmeetings. Please try again.",
		},
		{
			name: "Successful execution of Help Command",
			commandArgs: &model.CommandArgs{
				Command:   "/mstmeetings help",
				ChannelId: "dummyChannelID",
				UserId:    "dummyUserID",
			},
			expectedMsg: "###### Mattermost MS Teams Meetings Plugin - Slash Command Help\n* `/mstmeetings start` - Start an MS Teams meeting. \n* `/mstmeetings connect` - Connect to MS Teams meeting. \n* `/mstmeetings disconnect` - Disconnect your Mattermost account from MS Teams. \n* `/mstmeetings help` - Display this help text.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			post := &model.Post{
				UserId:    p.botUserID,
				ChannelId: tt.commandArgs.ChannelId,
				Message:   tt.expectedMsg,
			}

			mockAPI.On("SendEphemeralPost", tt.commandArgs.UserId, post).Return(&model.Post{}).Once()

			response, _ := p.ExecuteCommand(dummyPluginContext, tt.commandArgs)

			require.Equal(t, &model.CommandResponse{}, response)
			mockAPI.AssertExpectations(t)
		})
	}
}
