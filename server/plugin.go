// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/experimental/telemetry"
	"github.com/pkg/errors"
)

const (
	botUserName    = "mstmeetings"
	botDisplayName = "MS Teams Meetings"
	botDescription = "Created by the MS Teams Meetings plugin."
)

// Plugin defines the plugin struct
type Plugin struct {
	plugin.MattermostPlugin

	// botUserID of the created bot account.
	botUserID string

	client ClientInterface

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	telemetryClient telemetry.Client
	tracker         telemetry.Tracker
}

// OnActivate checks if the configurations is valid and ensures the bot account exists
func (p *Plugin) OnActivate() error {
	pluginAPIClient := pluginapi.NewClient(p.API, p.Driver)

	config := p.getConfiguration()
	if err := config.IsValid(); err != nil {
		return err
	}

	if _, err := p.getSiteURL(); err != nil {
		return err
	}

	botUserID, err := pluginAPIClient.Bot.EnsureBot(&model.Bot{
		Username:    botUserName,
		DisplayName: botDisplayName,
		Description: botDescription,
	})
	if err != nil {
		return errors.Wrap(err, "failed to ensure bot account")
	}
	p.botUserID = botUserID

	bundlePath, err := p.API.GetBundlePath()
	if err != nil {
		return errors.Wrap(err, "couldn't get bundle path")
	}

	if err = p.API.RegisterCommand(getCommand(pluginAPIClient)); err != nil {
		return errors.WithMessage(err, "OnActivate: failed to register command")
	}

	profileImage, err := os.ReadFile(filepath.Join(bundlePath, "assets", "profile.png"))
	if err != nil {
		return errors.Wrap(err, "couldn't read profile image")
	}

	if appErr := p.API.SetProfileImage(botUserID, profileImage); appErr != nil {
		return errors.Wrap(appErr, "couldn't set profile image")
	}

	p.telemetryClient, err = telemetry.NewRudderClient()
	if err != nil {
		p.API.LogWarn("telemetry client not started", "error", err.Error())
	}

	return nil
}

func (p *Plugin) OnDeactivate() error {
	if p.telemetryClient != nil {
		err := p.telemetryClient.Close()
		if err != nil {
			p.API.LogWarn("OnDeactivate: failed to close telemetryClient", "error", err.Error())
		}
	}

	return nil
}
