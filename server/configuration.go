// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"reflect"

	"github.com/mattermost/mattermost/server/public/pluginapi/experimental/bot/logger"
	"github.com/mattermost/mattermost/server/public/pluginapi/experimental/telemetry"
	"github.com/pkg/errors"
)

// configuration captures the plugin's external configuration as exposed in the Mattermost server
// configuration, as well as values computed from the configuration. Any public fields will be
// deserialized from the Mattermost server configuration in OnConfigurationChange.
//
// As plugins are inherently concurrent (hooks being called asynchronously), and the plugin
// configuration can change at any time, access to the configuration must be synchronized. The
// strategy used in this plugin is to guard a pointer to the configuration, and clone the entire
// struct whenever it changes. You may replace this with whatever strategy you choose.
//
// If you add non-reference types to your configuration struct, be sure to rewrite Clone as a deep
// copy appropriate for your types.
type configuration struct {
	OAuth2Authority    string `json:"oauth2authority"`
	OAuth2ClientID     string `json:"oauth2clientid"`
	OAuth2ClientSecret string `json:"oauth2clientsecret"`
	EncryptionKey      string `json:"encryptionkey"`
}

func (c *configuration) ToMap() (map[string]interface{}, error) {
	var out map[string]interface{}
	data, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func (c *configuration) differsOAuth2(other *configuration) bool {
	return c.OAuth2Authority != other.OAuth2Authority ||
		c.OAuth2ClientID != other.OAuth2ClientID ||
		c.OAuth2ClientSecret != other.OAuth2ClientSecret ||
		c.EncryptionKey != other.EncryptionKey
}

// Clone shallow copies the configuration. Your implementation may require a deep copy if
// your configuration has reference types.
func (c *configuration) Clone() *configuration {
	var clone = *c
	return &clone
}

// IsValid checks if all needed fields are set.
func (c *configuration) IsValid() error {
	switch {
	case len(c.OAuth2ClientSecret) == 0:
		return errors.New("OAuthClientSecret is not configured")

	case len(c.OAuth2ClientID) == 0:
		return errors.New("OAuthClientID is not configured")

	case len(c.OAuth2Authority) == 0:
		return errors.New("OAuth2Authority is not configured")
	}

	return nil
}

// getConfiguration retrieves the active configuration under lock, making it safe to use
// concurrently. The active configuration may change underneath the client of this method, but
// the struct returned by this API call is considered immutable.
func (p *Plugin) getConfiguration() *configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	return p.configuration
}

// setConfiguration replaces the active configuration under lock.
//
// Do not call setConfiguration while holding the configurationLock, as sync.Mutex is not
// reentrant. In particular, avoid using the plugin API entirely, as this may in turn trigger a
// hook back into the plugin. If that hook attempts to acquire this lock, a deadlock may occur.
//
// This method panics if setConfiguration is called with the existing configuration. This almost
// certainly means that the configuration was modified without being cloned and may result in
// an unsafe access.
func (p *Plugin) setConfiguration(c *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if c != nil && p.configuration == c {
		// Ignore assignment if the configuration struct is empty. Go will optimize the
		// allocation for same to point at the same memory address, breaking the check
		// above.
		if reflect.ValueOf(*c).NumField() == 0 {
			return
		}

		panic("setConfiguration called with the existing configuration")
	}

	p.configuration = c
}

// OnConfigurationChange is invoked when configuration changes may have been made.
func (p *Plugin) OnConfigurationChange() error {
	prev := p.getConfiguration()

	// Load the public configuration fields from the Mattermost server configuration.
	loaded := configuration{}
	if err := p.API.LoadPluginConfiguration(&loaded); err != nil {
		return errors.Wrap(err, "failed to load plugin configuration")
	}

	changedEncryptionKey := false
	resetUserKeys := (prev != nil && loaded.differsOAuth2(prev))
	if loaded.EncryptionKey == "" {
		secret, err := generateSecret()
		if err != nil {
			return err
		}
		loaded.EncryptionKey = secret
		resetUserKeys = true
		changedEncryptionKey = true
		p.API.LogInfo("auto-generated encryption key in the configuration")
	}

	p.setConfiguration(&loaded)

	if changedEncryptionKey {
		go p.storeConfiguration(&loaded)
	}
	if resetUserKeys {
		// not sure how to avoid executing this from each server since
		// cluster.Mutex relies on KV, which is wiped out. Should be safe to
		// delete multiple times though.
		go p.resetAllOAuthTokens()
	}

	p.tracker = telemetry.NewTracker(p.telemetryClient, p.API.GetDiagnosticId(), p.API.GetServerVersion(), manifest.Id, manifest.Version, "msteamsmeetings", telemetry.NewTrackerConfig(p.API.GetConfig()), logger.New(p.API))

	return nil
}

func (p *Plugin) storeConfiguration(c *configuration) {
	var err error
	defer func() {
		if err != nil {
			p.API.LogError("Failed to store updated plugin configuration with an encryption key: %s", err.Error())
		}
	}()
	configMap, err := c.ToMap()
	if err != nil {
		return
	}
	appErr := p.API.SavePluginConfig(configMap)
	if appErr != nil {
		err = appErr
		return
	}
}
