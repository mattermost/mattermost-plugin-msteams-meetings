// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"context"

	"github.com/mattermost/mattermost/server/public/plugin"
	msgraph "github.com/yaegashi/msgraph.go/beta"
	"golang.org/x/oauth2"
)

type ClientInterface interface {
	CreateMeeting(creator *UserInfo, attendeesIDs []*UserInfo, subject string) (*msgraph.OnlineMeeting, error)
	GetMe() (*msgraph.User, error)
}

// ClientFactory is a function type for creating clients, used for dependency injection in tests
type ClientFactory func(conf *oauth2.Config, token *oauth2.Token) ClientInterface

// Client represents a MSGraph API client
type Client struct {
	builder *msgraph.GraphServiceRequestBuilder
	api     plugin.API
}

// NewClient returns a new MSGraph API client.
func (p *Plugin) NewClient(conf *oauth2.Config, token *oauth2.Token) ClientInterface {
	ctx := context.Background()
	httpClient := conf.Client(ctx, token)
	builder := msgraph.NewClient(httpClient)
	builder.SetURL(graphBaseURL(p.getConfiguration().OAuth2TenantType))
	return &Client{
		builder: builder,
		api:     p.API,
	}
}
