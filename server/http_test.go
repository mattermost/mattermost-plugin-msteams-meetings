// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	msgraph "github.com/yaegashi/msgraph.go/beta"
	"golang.org/x/oauth2"
)

func TestConnectUser(t *testing.T) {
	api := &plugintest.API{}
	p := &Plugin{
		MattermostPlugin: plugin.MattermostPlugin{
			API: api,
		},
	}

	tests := []struct {
		name                string
		userID              string
		channelID           string
		expectedStatus      int
		expectedBody        string
		redirectExpected    bool
		expectedRedirectURL string
		setup               func()
	}{
		{
			name:           "Unauthorized User",
			channelID:      "testChannelID",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Not authorized\n",
			setup: func() {
				api.On("LogError", "connectUser, unauthorized user").Return(nil)
			},
		},
		{
			name:           "Missing Channel ID",
			userID:         "testUserID",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "channelID missing\n",
			setup: func() {
				api.On("LogError", "connectUser, missing channelID in query params").Return(nil)
			},
		},
		{
			name:           "Error getting OAuth Config",
			userID:         "testUserID",
			channelID:      "testChannelID",
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "error fetching siteURL\n",
			setup: func() {
				p.setConfiguration(&configuration{
					OAuth2ClientID:     "testOAuth2ClientID",
					OAuth2ClientSecret: "testOAuth2ClientSecret",
					OAuth2Authority:    "testOAuth2Authority",
				})

				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: nil,
					},
				})

				api.On("LogError", "connectUser, failed to get oauth config", "Error", "error fetching siteURL").Return(nil)
			},
		},
		{
			name:           "Error Getting User State",
			userID:         "testUserID",
			channelID:      "testChannelID",
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "error occurred getting stored user state\n",
			setup: func() {
				p.setConfiguration(&configuration{
					OAuth2ClientID:     "testOAuth2ClientID",
					OAuth2ClientSecret: "testOAuth2ClientSecret",
					OAuth2Authority:    "testOAuth2Authority",
				})
				testSiteURL := "testSiteURL"
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: &testSiteURL,
					},
				})

				api.On("KVGet", getOAuthUserStateKey("testUserID")).Return(nil, &model.AppError{Message: "error occurred getting stored user state"})
				api.On("LogError", "connectUser, failed to get user state", "UserID", "testUserID", "Error", "error occurred getting stored user state").Return(nil)
			},
		},
		{
			name:                "Successful OAuth Redirect",
			userID:              "testUserID",
			channelID:           "testChannelID",
			expectedStatus:      http.StatusFound,
			expectedBody:        "<a href=\"https://login.microsoftonline.com/testOAuth2Authority/oauth2/v2.0/authorize?access_type=offline&amp;client_id=testOAuth2ClientID&amp;redirect_uri=testSiteURL%2Fplugins%2Fcom.mattermost.msteamsmeetings%2Foauth2%2Fcomplete&amp;response_type=code&amp;scope=offline_access+OnlineMeetings.ReadWrite&amp;state=testOAuthState\">Found</a>.\n\n",
			redirectExpected:    true,
			expectedRedirectURL: "https://login.microsoftonline.com/testOAuth2Authority/oauth2/v2.0/authorize?access_type=offline&client_id=testOAuth2ClientID&redirect_uri=testSiteURL%2Fplugins%2Fcom.mattermost.msteamsmeetings%2Foauth2%2Fcomplete&response_type=code&scope=offline_access+OnlineMeetings.ReadWrite&state=testOAuthState",
			setup: func() {
				p.setConfiguration(&configuration{
					OAuth2ClientID:     "testOAuth2ClientID",
					OAuth2ClientSecret: "testOAuth2ClientSecret",
					OAuth2Authority:    "testOAuth2Authority",
				})

				testSiteURL := "testSiteURL"
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: &testSiteURL,
					},
				})

				mockState := "testOAuthState"
				api.On("KVGet", getOAuthUserStateKey("testUserID")).Return([]byte(mockState), nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api.ExpectedCalls = nil

			tt.setup()

			req := httptest.NewRequest(http.MethodGet, "/oauth2/connect?channelID="+tt.channelID, nil)
			if tt.userID != "" {
				req.Header.Set("Mattermost-User-ID", tt.userID)
			}
			w := httptest.NewRecorder()

			p.connectUser(w, req)

			resp := w.Result()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			require.Equal(t, tt.expectedStatus, resp.StatusCode)
			require.Equal(t, tt.expectedBody, string(body))

			if tt.redirectExpected {
				require.Equal(t, tt.expectedRedirectURL, resp.Header.Get("Location"))
			}

			api.AssertExpectations(t)
		})
	}
}

func TestHandleStartMeeting(t *testing.T) {
	api := &plugintest.API{}
	tracker := &MockTracker{}
	mockClient := &MockClient{}
	p := &Plugin{
		MattermostPlugin: plugin.MattermostPlugin{
			API: api,
		},
		tracker:   tracker,
		botUserID: "botUserID",
	}

	// Create mock client factory
	mockClientFactory := func(_ *oauth2.Config, _ *oauth2.Token) ClientInterface {
		return mockClient
	}

	testCases := []struct {
		name           string
		userID         string
		channelID      string
		expectedStatus int
		expectedBody   string
		setup          func()
	}{
		{
			name:           "Unauthorized User",
			userID:         "",
			channelID:      "testChannelID",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Not authorized\n",
			setup: func() {
				api.On("LogError", "handleStartMeeting, unauthorized user").Return(nil)
			},
		},
		{
			name:           "Invalid Request Body",
			userID:         "testUserID",
			channelID:      "testChannelID",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid character 'i' looking for beginning of value\n",
			setup: func() {
				api.On("LogError", "handleStartMeeting, failed to decode start meeting payload", "Error", "invalid character 'i' looking for beginning of value").Return(nil)
			},
		},
		{
			name:           "Error Getting User",
			userID:         "testUserID",
			channelID:      "testChannelID",
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "mock error\n",
			setup: func() {
				api.On("GetUser", "testUserID").Return(nil, &model.AppError{Message: "mock error", StatusCode: http.StatusInternalServerError})
				api.On("LogError", "handleStartMeeting, failed to get user", "UserID", "testUserID", "Error", "mock error").Return(nil)
			},
		},
		{
			name:           "Error Getting Channel Member",
			userID:         "testUserID",
			channelID:      "testChannelID",
			expectedStatus: http.StatusForbidden,
			expectedBody:   "Forbidden\n",
			setup: func() {
				user := &model.User{Id: "testUserID"}
				api.On("GetUser", "testUserID").Return(user, nil)
				api.On("GetChannelMember", "testChannelID", "testUserID").Return(nil, &model.AppError{Message: "mock error", StatusCode: http.StatusForbidden})
				api.On("LogError", "handleStartMeeting, failed to get channel member", "UserID", "testUserID", "Error", "mock error").Return(nil)
			},
		},
		{
			name:           "Error Checking Previous Messages",
			userID:         "testUserID",
			channelID:      "testChannelID",
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "mock error while checking previous messages\n",
			setup: func() {
				user := &model.User{Id: "testUserID"}
				api.On("GetUser", "testUserID").Return(user, nil)
				api.On("GetChannelMember", "testChannelID", "testUserID").Return(nil, nil)
				api.On("GetPostsSince", "testChannelID", (time.Now().Unix()-30)*1000).Return(nil, &model.AppError{Message: "mock error while checking previous messages", StatusCode: http.StatusInternalServerError})
				api.On("LogError", "handleStartMeeting, error occurred while checking previous messages in channel", "ChannelID", "testChannelID", "Error", "mock error while checking previous messages").Return(nil)
			},
		},
		{
			name:           "Authorization Error Writing Response",
			userID:         "testUserID",
			channelID:      "testChannelID",
			expectedStatus: http.StatusOK,
			expectedBody:   "{\"meeting_url\": \"\"}error fetching siteURL\n",
			setup: func() {
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: nil,
					},
				})
				api.On("GetUser", "testUserID").Return(&model.User{Id: "testUserID"}, nil)
				api.On("GetChannelMember", "testChannelID", "testUserID").Return(nil, nil)
				api.On("GetPostsSince", "testChannelID", (time.Now().Unix()-30)*1000).Return(&model.PostList{}, nil)
				api.On("LogError", "postConnect, cannot get oauth message", "error", "error fetching siteURL").Return()
				api.On("LogError", "authenticateAndFetchUser, cannot get oauth message", "error", "error fetching siteURL").Return()
				api.On("LogWarn", "failed to create connect post", "error", mock.Anything).Return(nil)
			},
		},
		{
			name:           "Error creating connect post",
			userID:         "testUserID",
			channelID:      "testChannelID",
			expectedStatus: http.StatusOK,
			expectedBody:   "{\"meeting_url\": \"\"}error fetching siteURL\n",
			setup: func() {
				siteURL := "testSiteURL"
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: &siteURL,
					},
				}).Times(1)
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: nil,
					},
				}).Times(2)
				p.setConfiguration(&configuration{
					EncryptionKey: "demo_encrypt_key",
				})
				testUserInfo := &UserInfo{}
				encryptedTestUserInfo, err := testUserInfo.EncryptedJSON([]byte("demo_encrypt_key"))
				require.NoError(t, err)
				api.On("KVGet", "token_testUserID").Return(encryptedTestUserInfo, nil)
				api.On("GetUser", "testUserID").Return(&model.User{Id: "testUserID"}, nil)
				api.On("GetChannelMember", "testChannelID", "testUserID").Return(nil, nil)
				api.On("GetPostsSince", "testChannelID", (time.Now().Unix()-30)*1000).Return(&model.PostList{}, nil)
				api.On("LogError", "authenticateAndFetchUser, cannot get oauth config", "error", "error fetching siteURL").Return()
				api.On("LogError", "postConnect, cannot get oauth message", "error", "error fetching siteURL").Return()
				api.On("LogWarn", "failed to create connect post", "error", "error fetching siteURL")
			},
		},
		{
			name:           "Error posting meeting",
			userID:         "testUserID",
			channelID:      "testChannelID",
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "cannot create post in this channel\n",
			setup: func() {
				siteURL := "testSiteURL"
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: &siteURL,
					},
				})
				p.setConfiguration(&configuration{
					EncryptionKey: "demo_encrypt_key",
				})
				testUserInfo := &UserInfo{}
				encryptedTestUserInfo, err := testUserInfo.EncryptedJSON([]byte("demo_encrypt_key"))
				require.NoError(t, err)
				api.On("KVGet", "token_testUserID").Return(encryptedTestUserInfo, nil)
				api.On("GetUser", "testUserID").Return(&model.User{Id: "testUserID"}, nil)
				api.On("GetChannelMember", "testChannelID", "testUserID").Return(nil, nil)
				api.On("GetPostsSince", "testChannelID", (time.Now().Unix()-30)*1000).Return(&model.PostList{}, nil)
				api.On("LogError", "handleStartMeeting, failed to post meeting", "UserID", "testUserID", "Error", "cannot create post in this channel")
				api.On("HasPermissionToChannel", "testUserID", "testChannelID", model.PermissionCreatePost).Return(false)
				mockClient.On("GetMe").Return(&msgraph.User{}, nil)
			},
		},
		{
			name:           "Start meeting successfully",
			userID:         "testUserID",
			channelID:      "testChannelID",
			expectedStatus: http.StatusOK,
			expectedBody:   `{"meeting_url": "testJoinURL"}`,
			setup: func() {
				siteURL := "testSiteURL"
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: &siteURL,
					},
				})
				p.setConfiguration(&configuration{
					EncryptionKey: "demo_encrypt_key",
				})
				testUserInfo := &UserInfo{}
				encryptedTestUserInfo, err := testUserInfo.EncryptedJSON([]byte("demo_encrypt_key"))
				require.NoError(t, err)

				testJoinURL := "testJoinURL"
				api.On("KVGet", "token_testUserID").Return(encryptedTestUserInfo, nil)
				api.On("GetUser", "testUserID").Return(&model.User{Id: "testUserID"}, nil)
				api.On("GetChannel", "testChannelID").Return(&model.Channel{Id: "testChannelID", Type: model.ChannelTypeOpen}, nil)
				api.On("GetChannelMember", "testChannelID", "testUserID").Return(nil, nil)
				api.On("GetPostsSince", "testChannelID", (time.Now().Unix()-30)*1000).Return(&model.PostList{}, nil)
				api.On("CreatePost", mock.Anything).Return(&model.Post{}, nil)
				api.On("HasPermissionToChannel", "testUserID", "testChannelID", model.PermissionCreatePost).Return(true)
				mockClient.On("GetMe").Return(&msgraph.User{}, nil)
				mockClient.On("CreateMeeting", mock.Anything, mock.Anything, mock.Anything).Return(&msgraph.OnlineMeeting{JoinURL: &testJoinURL}, nil)
				tracker.On("TrackUserEvent", "meeting_started", "testUserID", mock.Anything).Return(nil)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			api.ExpectedCalls = nil
			mockClient.ExpectedCalls = nil

			tc.setup()

			var reqBody []byte
			if tc.name == "Invalid Request Body" {
				reqBody = []byte("invalid-json-body")
			} else {
				reqBody, _ = json.Marshal(&startMeetingRequest{
					ChannelID: tc.channelID,
					Personal:  false,
					Topic:     "Test Meeting",
					MeetingID: 123,
				})
			}

			req := httptest.NewRequest(http.MethodPost, "/start_meeting", bytes.NewBuffer(reqBody))
			if tc.userID != "" {
				req.Header.Set("Mattermost-User-ID", tc.userID)
			}
			w := httptest.NewRecorder()

			p.handleStartMeetingWithDeps(w, req, mockClientFactory)

			resp := w.Result()
			body := w.Body.String()

			require.Equal(t, tc.expectedStatus, resp.StatusCode)
			require.Equal(t, tc.expectedBody, body)

			api.AssertExpectations(t)
			tracker.AssertExpectations(t)
		})
	}
}

func TestCompleteUserOAuth(t *testing.T) {
	api := &plugintest.API{}
	p := &Plugin{
		MattermostPlugin: plugin.MattermostPlugin{
			API: api,
		},
	}

	tests := []struct {
		name              string
		userID            string
		expectedStatus    int
		expectedBody      string
		state             string
		authorizationCode string
		setup             func()
	}{
		{
			name:           "Unauthorized User",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Not authorized, missing Mattermost user id\n",
			setup: func() {
				api.On("LogError", "completeUserOAuth, unauthorized user").Return(nil)
			},
		},
		{
			name:           "Error getting OAuth config",
			userID:         "testUserID",
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "error in oauth config\n",
			setup: func() {
				p.setConfiguration(&configuration{
					OAuth2ClientID:     "testOAuth2ClientID",
					OAuth2ClientSecret: "testOAuth2ClientSecret",
					OAuth2Authority:    "testOAuth2Authority",
				})
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: nil,
					},
				})
				api.On("LogError", "completeUserOAuth, failed to get oauth config", "Error", "error fetching siteURL").Return(nil)
			},
		},
		{
			name:           "Missing authorization code",
			userID:         "testUserID",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "missing authorization code\n",
			setup: func() {
				p.setConfiguration(&configuration{
					OAuth2ClientID:     "testOAuth2ClientID",
					OAuth2ClientSecret: "testOAuth2ClientSecret",
					OAuth2Authority:    "testOAuth2Authority",
				})
				siteURL := "testSiteURL"
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: &siteURL,
					},
				})
				api.On("LogError", "completeUserOAuth, missing authorization code").Return(nil)
			},
		},
		{
			name:              "Invalid state length",
			userID:            "testUserID",
			expectedStatus:    http.StatusBadRequest,
			expectedBody:      "invalid state\n",
			state:             "component1_component2",
			authorizationCode: "validCode123",
			setup: func() {
				p.setConfiguration(&configuration{
					OAuth2ClientID:     "testOAuth2ClientID",
					OAuth2ClientSecret: "testOAuth2ClientSecret",
					OAuth2Authority:    "testOAuth2Authority",
				})
				siteURL := "testSiteURL"
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: &siteURL,
					},
				})
				api.On("LogDebug", "complete oauth, state mismatch", "stateComponents", "[component1 component2]", "state", "component1_component2").Return(nil)
				api.On("LogDebug", "complete oauth, cannot parse state", "error", "status mismatch").Return(nil)
			},
		},
		{
			name:              "Missing stored state in KV store",
			userID:            "testUserID",
			expectedStatus:    http.StatusBadRequest,
			expectedBody:      "missing stored state\n",
			state:             "valid_state_userID_channelID",
			authorizationCode: "testAuthCode",
			setup: func() {
				p.setConfiguration(&configuration{
					OAuth2ClientID:     "testOAuth2ClientID",
					OAuth2ClientSecret: "testOAuth2ClientSecret",
					OAuth2Authority:    "testOAuth2Authority",
				})
				siteURL := "testSiteURL"
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: &siteURL,
					},
				})
				api.On("KVGet", "valid_state").Return(nil, &model.AppError{Message: "error getting state from store"})
				api.On("LogError", "completeUserOAuth, missing stored state").Return(nil)
			},
		},
		{
			name:              "Stored state does not match provided state",
			userID:            "testUserID",
			expectedStatus:    http.StatusBadRequest,
			expectedBody:      "invalid state\n",
			state:             "valid_state_userID_channelID",
			authorizationCode: "testAuthCode",
			setup: func() {
				p.setConfiguration(&configuration{
					OAuth2ClientID:     "testOAuth2ClientID",
					OAuth2ClientSecret: "testOAuth2ClientSecret",
					OAuth2Authority:    "testOAuth2Authority",
				})
				siteURL := "testSiteURL"
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: &siteURL,
					},
				})
				api.On("KVGet", "valid_state").Return([]byte("different_stored_state"), nil)
				api.On("LogError", "completeUserOAuth, invalid state").Return(nil)
			},
		},
		{
			name:              "Incorrect User ID in State",
			userID:            "testUserID",
			expectedStatus:    http.StatusUnauthorized,
			expectedBody:      "Not authorized, incorrect user\n",
			state:             "valid_state_for_oauth",
			authorizationCode: "testAuthCode",
			setup: func() {
				p.setConfiguration(&configuration{
					OAuth2ClientID:     "testOAuth2ClientID",
					OAuth2ClientSecret: "testOAuth2ClientSecret",
					OAuth2Authority:    "testOAuth2Authority",
				})
				siteURL := "testSiteURL"
				api.On("GetConfig").Return(&model.Config{
					ServiceSettings: model.ServiceSettings{
						SiteURL: &siteURL,
					},
				})
				api.On("KVGet", "valid_state").Return([]byte("valid_state_for_oauth"), nil)
				api.On("KVDelete", "valid_state").Return(nil)
				api.On("LogError", "completeUserOAuth, unauthorized user", "UserID", "testUserID").Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api.ExpectedCalls = nil

			tt.setup()

			req := httptest.NewRequest(http.MethodGet, "/oauth2/complete", nil)
			if tt.userID != "" {
				req.Header.Set("Mattermost-User-ID", tt.userID)
			}

			if tt.state != "" {
				q := req.URL.Query()
				q.Add("state", tt.state)
				req.URL.RawQuery = q.Encode()
			}

			if tt.authorizationCode != "" {
				q := req.URL.Query()
				q.Add("code", tt.authorizationCode)
				req.URL.RawQuery = q.Encode()
			}

			w := httptest.NewRecorder()

			p.completeUserOAuth(w, req)

			resp := w.Result()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			require.Equal(t, tt.expectedStatus, resp.StatusCode)
			require.Contains(t, tt.expectedBody, string(body))

			api.AssertExpectations(t)
		})
	}
}
