// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	msgraph "github.com/yaegashi/msgraph.go/beta"
)

type receivedRequest struct {
	method string
	path   string
}

func newTestGraphClient(t *testing.T, response string) (*Client, *receivedRequest) {
	t.Helper()
	received := &receivedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.method, received.path = r.Method, r.URL.Path
		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Planning", body["subject"])
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)

	builder := msgraph.NewClient(server.Client())
	builder.SetURL(server.URL + "/v1.0")
	return &Client{builder: builder}, received
}

func TestCreateMeetingJoinURL(t *testing.T) {
	creator := &UserInfo{RemoteID: "creator-id", UPN: "creator@example.com"}

	t.Run("v1.0 response with joinWebUrl", func(t *testing.T) {
		client, received := newTestGraphClient(t, `{"id":"meeting-id","joinWebUrl":"https://gov.teams.microsoft.us/l/meetup-join/abc"}`)
		meeting, err := client.CreateMeeting(creator, nil, "Planning")
		require.NoError(t, err)
		require.NotNil(t, meeting.JoinURL)
		assert.Equal(t, "https://gov.teams.microsoft.us/l/meetup-join/abc", *meeting.JoinURL)
		assert.Equal(t, http.MethodPost, received.method)
		assert.Equal(t, "/v1.0/users/creator-id/onlineMeetings", received.path)
	})

	t.Run("beta response with joinUrl", func(t *testing.T) {
		client, _ := newTestGraphClient(t, `{"id":"meeting-id","joinUrl":"https://teams.microsoft.com/l/meetup-join/abc"}`)
		meeting, err := client.CreateMeeting(creator, nil, "Planning")
		require.NoError(t, err)
		require.NotNil(t, meeting.JoinURL)
		assert.Equal(t, "https://teams.microsoft.com/l/meetup-join/abc", *meeting.JoinURL)
	})

	t.Run("response without a join link is an error, not a panic", func(t *testing.T) {
		client, _ := newTestGraphClient(t, `{"id":"meeting-id"}`)
		meeting, err := client.CreateMeeting(creator, nil, "Planning")
		require.Error(t, err)
		assert.Nil(t, meeting)
	})
}
