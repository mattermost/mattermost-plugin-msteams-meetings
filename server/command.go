// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/experimental/command"
	"github.com/pkg/errors"
)

const (
	availableCommands = "Available commands: start, connect, disconnect, help"
	commandHelp       = "###### Mattermost MS Teams Meetings Plugin - Slash Command Help\n" +
		"* |/mstmeetings start| - Start an MS Teams meeting. \n" +
		"* |/mstmeetings connect| - Connect to MS Teams meeting. \n" +
		"* |/mstmeetings disconnect| - Disconnect your Mattermost account from MS Teams. \n" +
		"* |/mstmeetings help| - Display this help text."
	tooManyParametersText = "Too many parameters."
)

func getCommand(client *pluginapi.Client) *model.Command {
	iconData, err := command.GetIconData(&client.System, "assets/profile.svg")
	if err != nil {
		client.Log.Warn("Error getting icon data", "err", err.Error())
	}

	return &model.Command{
		Trigger:              "mstmeetings",
		DisplayName:          "MS Teams Meetings",
		Description:          "Integration with MS Teams Meetings.",
		AutoComplete:         true,
		AutoCompleteDesc:     availableCommands,
		AutoCompleteHint:     "[command]",
		AutocompleteIconData: iconData,
		AutocompleteData:     getAutocompleteData(),
	}
}

func (p *Plugin) postCommandResponse(args *model.CommandArgs, text string) {
	post := &model.Post{
		UserId:    p.botUserID,
		ChannelId: args.ChannelId,
		Message:   text,
	}
	_ = p.API.SendEphemeralPost(args.UserId, post)
}

func getAutocompleteData() *model.AutocompleteData {
	cmd := model.NewAutocompleteData("mstmeetings", "[command]", availableCommands)

	start := model.NewAutocompleteData("start", "", "Start an MS Teams meeting")
	cmd.AddCommand(start)

	connect := model.NewAutocompleteData("connect", "",
		"Connect your Mattermost account to MS Teams")
	cmd.AddCommand(connect)

	disconnect := model.NewAutocompleteData("disconnect", "",
		"Disconnect your Mattermost account from MS Teams")
	cmd.AddCommand(disconnect)

	help := model.NewAutocompleteData("help", "", "Display usage information")
	cmd.AddCommand(help)

	return cmd
}

func (p *Plugin) executeCommand(_ *plugin.Context, args *model.CommandArgs) (string, error) {
	split := strings.Fields(args.Command)
	cmd := split[0]
	action := ""

	if cmd != "/mstmeetings" {
		return fmt.Sprintf("Command '%s' is not /mstmeetings. Please try again.", cmd), nil
	}

	if len(split) > 1 {
		action = split[1]
	} else {
		return p.handleHelp()
	}

	switch action {
	case "start":
		return p.handleStart(split[1:], args)
	case "connect":
		return p.handleConnect(split[1:], args)
	case "disconnect":
		return p.handleDisconnect(split[1:], args)
	case "help":
		return p.handleHelp()
	}

	return fmt.Sprintf("Unknown action `%v`.\n%s", action, p.getHelpText()), nil
}

func (p *Plugin) getHelpText() string {
	return strings.ReplaceAll(commandHelp, "|", "`")
}

func (p *Plugin) handleHelp() (string, error) {
	return p.getHelpText(), nil
}

func (p *Plugin) handleStartWithDeps(args []string, extra *model.CommandArgs, newClient ClientFactory) (string, error) {
	topic := ""
	if len(args) > 1 {
		topic = strings.Join(args[1:], " ")
	}
	userID := extra.UserId
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		return "Cannot get user.", errors.Wrap(appErr, "cannot get user")
	}

	if _, appErr = p.API.GetChannelMember(extra.ChannelId, userID); appErr != nil {
		return "We could not get channel members.", errors.Wrap(appErr, "cannot get channel member")
	}

	recentMeeting, recentMeetingURL, creatorName, provider, appErr := p.checkPreviousMessages(extra.ChannelId)
	if appErr != nil {
		return "Error checking previous messages.", errors.Wrap(appErr, "cannot check previous messages")
	}

	if recentMeeting {
		p.postConfirmCreateOrJoin(recentMeetingURL, extra.ChannelId, topic, userID, creatorName, provider)
		p.trackMeetingDuplication(extra.UserId)
		return "", nil
	}

	_, userInfo, client, authErr := p.authenticateAndFetchUser(userID, extra.ChannelId, newClient)
	if authErr != nil {
		// the user state will be needed later while connecting the user to MS teams meeting via OAuth
		if _, err := p.StoreState(userID, extra.ChannelId, false); err != nil {
			p.API.LogWarn("failed to store user state", "error", err.Error())
		}

		return authErr.Message, authErr.Err
	}

	_, _, err := p.postMeetingWithDeps(user, extra.ChannelId, topic, client, userInfo)
	if err != nil {
		return "Failed to post message. Please try again.", errors.Wrap(err, "cannot post message")
	}

	p.trackMeetingStart(extra.UserId, telemetryStartSourceCommand)
	return "", nil
}

func (p *Plugin) handleStart(args []string, extra *model.CommandArgs) (string, error) {
	return p.handleStartWithDeps(args, extra, p.NewClient)
}

func (p *Plugin) handleConnectWithDeps(args []string, extra *model.CommandArgs, newClient ClientFactory) (string, error) {
	if len(args) > 1 {
		return tooManyParametersText, nil
	}

	msUser, _, _, authErr := p.authenticateAndFetchUser(extra.UserId, extra.ChannelId, newClient)
	if authErr != nil {
		// the user state will be needed later while connecting the user to MS teams meeting via OAuth
		if _, err := p.StoreState(extra.UserId, extra.ChannelId, true); err != nil {
			p.API.LogWarn("failed to store user state", "error", err.Error())
		}

		return authErr.Message, authErr.Err
	}

	if msUser != nil {
		return "User already connected to MS Teams Meetings", nil
	}

	return "", nil
}

func (p *Plugin) handleConnect(args []string, extra *model.CommandArgs) (string, error) {
	return p.handleConnectWithDeps(args, extra, p.NewClient)
}

func (p *Plugin) handleDisconnect(args []string, extra *model.CommandArgs) (string, error) {
	if len(args) > 1 {
		return tooManyParametersText, nil
	}
	err := p.disconnect(extra.UserId)
	if err != nil {
		return fmt.Sprintf("Failed to disconnect user, %s", err.Error()), nil
	}

	p.trackDisconnect(extra.UserId)
	return "You have successfully disconnected from MS Teams Meetings.", nil
}

// ExecuteCommand is called when any registered by this plugin command is executed
func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	msg, err := p.executeCommand(c, args)
	if err != nil {
		p.API.LogWarn("failed to execute command", "error", err.Error())
	}
	if msg != "" {
		p.postCommandResponse(args, msg)
	}
	return &model.CommandResponse{}, nil
}
