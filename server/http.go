// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License for license information.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"golang.org/x/oauth2"
)

const (
	postTypeStarted = "STARTED"
	postTypeConfirm = "RECENTLY_CREATED"

	msteamsProviderName = "Microsoft Teams Meetings"
)

func (p *Plugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	config := p.getConfiguration()
	if err := config.IsValid(); err != nil {
		p.API.LogError("Invalid plugin config", "Error", err.Error())
		http.Error(w, "This plugin is not configured.", http.StatusNotImplemented)
		return
	}

	switch path := r.URL.Path; path {
	case "/api/v1/meetings":
		p.handleStartMeeting(w, r)
	case "/oauth2/connect":
		p.connectUser(w, r)
	case "/oauth2/complete":
		p.completeUserOAuth(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (p *Plugin) connectUser(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		p.API.LogError("connectUser, unauthorized user")
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	channelID := r.URL.Query().Get("channelID")
	if channelID == "" {
		p.API.LogError("connectUser, missing channelID in query params")
		http.Error(w, "channelID missing", http.StatusBadRequest)
		return
	}

	conf, err := p.getOAuthConfig()
	if err != nil {
		p.API.LogError("connectUser, failed to get oauth config", "Error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	state, err := p.GetState(getOAuthUserStateKey(userID))
	if err != nil {
		p.API.LogError("connectUser, failed to store user state", "UserID", userID, "Error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	url := conf.AuthCodeURL(state, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusFound)
}

func (p *Plugin) completeUserOAuth(w http.ResponseWriter, r *http.Request) {
	authedUserID := r.Header.Get("Mattermost-User-ID")
	if authedUserID == "" {
		p.API.LogError("completeUserOAuth, unauthorized user")
		http.Error(w, "Not authorized, missing Mattermost user id", http.StatusUnauthorized)
		return
	}

	ctx := context.Background()
	conf, err := p.getOAuthConfig()
	if err != nil {
		p.API.LogError("completeUserOAuth, failed to get oauth config", "Error", err.Error())
		http.Error(w, "error in oauth config", http.StatusInternalServerError)
		return
	}

	code := r.URL.Query().Get("code")
	if len(code) == 0 {
		p.API.LogError("completeUserOAuth, missing authorization code")
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	state := r.URL.Query().Get("state")

	key, userID, channelID, justConnect, err := p.ParseState(state)
	if err != nil {
		p.API.LogDebug("complete oauth, cannot parse state", "error", err.Error())
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	storedState, err := p.GetState(key)
	if err != nil {
		p.API.LogError("completeUserOAuth, missing stored state")
		http.Error(w, "missing stored state", http.StatusBadRequest)
		return
	}

	if storedState != state {
		p.API.LogError("completeUserOAuth, invalid state")
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	_ = p.DeleteState(key)

	if userID != authedUserID {
		p.API.LogError("completeUserOAuth, unauthorized user", "UserID", authedUserID)
		http.Error(w, "Not authorized, incorrect user", http.StatusUnauthorized)
		return
	}

	tok, err := conf.Exchange(ctx, code)
	if err != nil {
		p.API.LogDebug("complete oauth, error getting token", "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p.client = p.NewClient(conf, tok)

	remoteUser, err := p.getUserWithToken()
	if err != nil {
		p.API.LogDebug("complete oauth, error getting user", "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if remoteUser.Mail == nil {
		p.API.LogDebug("user has no mail")
		http.Error(w, "User has no mail. Please check the user is properly configured in Microsoft", http.StatusInternalServerError)
		return
	}

	if remoteUser.ID == nil {
		p.API.LogDebug("user has no ID")
		http.Error(w, "User has no ID. Please check the user is properly configured in Microsoft", http.StatusInternalServerError)
		return
	}

	if remoteUser.UserPrincipalName == nil {
		p.API.LogDebug("user has no UPN")
		http.Error(w, "User has no user principal name. Please check the user is properly configured in Microsoft", http.StatusInternalServerError)
		return
	}

	userInfo := &UserInfo{
		UserID:     userID,
		OAuthToken: tok,
		Email:      *remoteUser.Mail,
		RemoteID:   *remoteUser.ID,
		UPN:        *remoteUser.UserPrincipalName,
	}

	err = p.StoreUserInfo(userInfo)
	if err != nil {
		p.API.LogDebug("complete oauth, error storing the user info", "error", err.Error())
		http.Error(w, "Unable to connect user to Microsoft", http.StatusInternalServerError)
		return
	}

	p.trackConnect(userID)

	html := `
<!DOCTYPE html>
<html>
	<head>
		<script>
			window.close();
		</script>
	</head>
	<body>
		<p>Completed connecting to Microsoft. Please close this window.</p>
	</body>
</html>
`
	if justConnect {
		post := &model.Post{
			UserId:    p.botUserID,
			ChannelId: channelID,
			Message:   "You have successfully connected to MS Teams Meetings.",
		}

		p.API.SendEphemeralPost(userID, post)
	} else {
		user, appErr := p.API.GetUser(userID)
		if appErr != nil {
			p.API.LogError("complete oauth, error getting MM user", "error", appErr.Error())
			http.Error(w, appErr.Error(), http.StatusInternalServerError)
			return
		}

		_, _, err = p.postMeeting(user, channelID, "")
		if err != nil {
			p.API.LogDebug("complete oauth, error posting meeting", "error", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(html))
}

type startMeetingRequest struct {
	ChannelID string `json:"channel_id"`
	Personal  bool   `json:"personal"`
	Topic     string `json:"topic"`
	MeetingID int    `json:"meeting_id"`
}

func (p *Plugin) handleStartMeeting(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-Id")
	if userID == "" {
		p.API.LogError("handleStartMeeting, unauthorized user")
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	var req startMeetingRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		p.API.LogError("handleStartMeeting, failed to decode start meeting payload", "Error", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		p.API.LogError("handleStartMeeting, failed to get user", "UserID", userID, "Error", appErr.Message)
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	_, appErr = p.API.GetChannelMember(req.ChannelID, userID)
	if appErr != nil {
		p.API.LogError("handleStartMeeting, failed to get channel member", "UserID", userID, "Error", appErr.Message)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.URL.Query().Get("force") == "" {
		recentMeeting, recentMeetingURL, creatorName, provider, cpmErr := p.checkPreviousMessages(req.ChannelID)
		if cpmErr != nil {
			p.API.LogError("handleStartMeeting, error occurred while checking previous messages in channel", "ChannelID", req.ChannelID, "Error", cpmErr.Message)
			http.Error(w, cpmErr.Error(), cpmErr.StatusCode)
			return
		}

		if recentMeeting {
			_, err = w.Write([]byte(`{"meeting_url": ""}`))
			if err != nil {
				p.API.LogWarn("failed to write response", "error", err.Error())
			}
			p.postConfirmCreateOrJoin(recentMeetingURL, req.ChannelID, req.Topic, userID, creatorName, provider)
			p.trackMeetingDuplication(userID)
			return
		}
	}

	_, authErr := p.authenticateAndFetchUser(userID, req.ChannelID)
	if authErr != nil {
		if _, err = w.Write([]byte(`{"meeting_url": ""}`)); err != nil {
			p.API.LogWarn("failed to write response", "error", err.Error())
		}

		if _, err = p.postConnect(req.ChannelID, userID); err != nil {
			p.API.LogWarn("failed to create connect post", "error", err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// the user state will be needed later while connecting the user to MS teams meeting via OAuth
		if _, err = p.StoreState(userID, req.ChannelID, false); err != nil {
			p.API.LogWarn("failed to store user state", "error", err.Error())
		}

		return
	}

	_, meeting, err := p.postMeeting(user, req.ChannelID, req.Topic)
	if err != nil {
		p.API.LogError("handleStartMeeting, failed to post meeting", "UserID", user.Id, "Error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	p.trackMeetingStart(userID, telemetryStartSourceWebapp)
	if r.URL.Query().Get("force") != "" {
		p.trackMeetingForced(userID)
	}

	_, err = w.Write([]byte(fmt.Sprintf(`{"meeting_url": "%s"}`, *meeting.JoinURL)))
	if err != nil {
		p.API.LogWarn("failed to write response", "error", err.Error())
	}
}
