// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/require"
)

func TestGetOauthMessage(t *testing.T) {
	for _, testCase := range []struct {
		description string
		siteURL     string
		setupFunc   func(p *Plugin)
	}{
		{
			description: "successful",
			siteURL:     "https://example-url.com",
			setupFunc: func(p *Plugin) {
				msg, err := p.getOauthMessage("mockChannelID")
				require.NoError(t, err)
				require.EqualValues(t, "[Click here to link your Microsoft account.](https://example-url.com/plugins/com.mattermost.msteamsmeetings/oauth2/connect?channelID=mockChannelID)", msg)
			},
		},
		{
			description: "missing site URL",
			siteURL:     "",
			setupFunc: func(p *Plugin) {
				msg, err := p.getOauthMessage("mockChannelID")
				require.EqualError(t, err, "error fetching siteURL")
				require.EqualValues(t, "", msg)
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := &Plugin{}
			api := &plugintest.API{}
			api.On("GetConfig").Return(&model.Config{
				ServiceSettings: model.ServiceSettings{
					SiteURL: model.NewPointer(testCase.siteURL),
				},
			})

			p.SetAPI(api)

			testCase.setupFunc(p)
		})
	}
}
