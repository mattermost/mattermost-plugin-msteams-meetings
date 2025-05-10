// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
	msgraph "github.com/yaegashi/msgraph.go/beta"
)

func (p *Plugin) postMeeting(creator *model.User, channelID string, topic string) (*model.Post, *msgraph.OnlineMeeting, error) {
	userInfo, err := p.GetUserInfo(creator.Id)
	if err != nil {
		return nil, nil, err
	}

	if !p.API.HasPermissionToChannel(creator.Id, channelID, model.PermissionCreatePost) {
		return nil, nil, errors.New("cannot create post in this channel")
	}

	attendees := []*UserInfo{}

	channel, appErr := p.API.GetChannel(channelID)
	if appErr != nil {
		return nil, nil, appErr
	}

	if channel.IsGroupOrDirect() {
		var members model.ChannelMembers
		members, appErr = p.API.GetChannelMembers(channelID, 0, 100)
		if appErr != nil {
			return nil, nil, appErr
		}
		if members == nil {
			return nil, nil, errors.New("returned members is nil")
		}
		for _, member := range members {
			var attendeeInfo *UserInfo
			attendeeInfo, err = p.GetUserInfo(member.UserId)
			if err != nil {
				continue
			}
			attendees = append(attendees, attendeeInfo)
		}
	}

	meeting, err := p.client.CreateMeeting(userInfo, attendees, topic)
	if err != nil {
		return nil, nil, err
	}

	post := &model.Post{
		UserId:    creator.Id,
		ChannelId: channelID,
		Message:   fmt.Sprintf("Meeting started at [this link](%s).", *meeting.JoinURL),
		Type:      "custom_mstmeetings",
		Props: map[string]interface{}{
			"meeting_link":             *meeting.JoinURL,
			"meeting_status":           postTypeStarted,
			"meeting_personal":         true,
			"meeting_topic":            topic,
			"meeting_creator_username": creator.Username,
			"meeting_provider":         msteamsProviderName,
		},
	}

	post, appErr = p.API.CreatePost(post)
	if appErr != nil {
		return nil, nil, appErr
	}

	return post, meeting, nil
}

func (p *Plugin) postConfirmCreateOrJoin(meetingURL string, channelID string, topic string, userID string, creatorName string, provider string) *model.Post {
	message := "There is another recent meeting created on this channel."
	if provider != msteamsProviderName {
		message = fmt.Sprintf("There is another recent meeting created on this channel with %s.", provider)
	}
	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: channelID,
		Message:   message,
		Type:      "custom_mstmeetings",
		Props: map[string]interface{}{
			"type":                     "custom_mstmeetings",
			"meeting_link":             meetingURL,
			"meeting_status":           postTypeConfirm,
			"meeting_personal":         true,
			"meeting_topic":            topic,
			"meeting_creator_username": creatorName,
			"meeting_provider":         provider,
		},
	}

	return p.API.SendEphemeralPost(userID, post)
}

func (p *Plugin) postConnect(channelID string, userID string) (*model.Post, error) {
	oauthMsg, err := p.getOauthMessage(channelID)
	if err != nil {
		p.API.LogError("postConnect, cannot get oauth message", "error", err.Error())
		return nil, err
	}

	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: channelID,
		Message:   oauthMsg,
	}

	return p.API.SendEphemeralPost(userID, post), nil
}
