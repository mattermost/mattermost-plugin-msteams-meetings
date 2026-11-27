// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"context"
	"net/http"
	"time"

	"github.com/pkg/errors"
	msgraph "github.com/yaegashi/msgraph.go/beta"
)

func (c *Client) CreateMeeting(creator *UserInfo, attendeesIDs []*UserInfo, subject string) (*msgraph.OnlineMeeting, error) {
	ctx := context.Background()
	start := time.Now()
	end := start.Add(1 * time.Hour)
	attendees := []msgraph.MeetingParticipantInfo{}
	if subject == "" {
		subject = "MS Teams Meeting"
	}
	for _, attendee := range attendeesIDs {
		attendees = append(attendees, msgraph.MeetingParticipantInfo{
			Identity: &msgraph.IdentitySet{
				User: &msgraph.Identity{
					ID: &attendee.RemoteID,
				},
			},
			Upn: &attendee.UPN,
		})
	}

	in := msgraph.OnlineMeeting{
		StartDateTime: &start,
		EndDateTime:   &end,
		Subject:       &subject,
		Participants: &msgraph.MeetingParticipants{
			Organizer: &msgraph.MeetingParticipantInfo{
				Identity: &msgraph.IdentitySet{
					User: &msgraph.Identity{
						ID: &creator.RemoteID,
					},
				},
				Upn: &creator.UPN,
			},
			Attendees: attendees,
		},
	}
	// The beta API returns the join link as joinUrl; v1.0 (used for the US Government clouds)
	// returns it as joinWebUrl, which the beta model doesn't have.
	var out struct {
		msgraph.OnlineMeeting
		JoinWebURL *string `json:"joinWebUrl,omitempty"`
	}

	err := c.builder.Users().ID(creator.RemoteID).OnlineMeetings().Request().JSONRequest(ctx, http.MethodPost, "", &in, &out)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create meeting")
	}

	meeting := out.OnlineMeeting
	if out.JoinWebURL != nil && *out.JoinWebURL != "" {
		meeting.JoinURL = out.JoinWebURL
	}
	if meeting.JoinURL == nil || *meeting.JoinURL == "" {
		return nil, errors.New("cannot create meeting: the response has no join URL")
	}
	return &meeting, nil
}
