// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiffersOAuth2TenantType(t *testing.T) {
	base := configuration{
		OAuth2Authority:    "tenant-id",
		OAuth2ClientID:     "client-id",
		OAuth2ClientSecret: "secret",
		EncryptionKey:      "key",
	}
	withTenantType := func(tenantType string) *configuration {
		c := base
		c.OAuth2TenantType = tenantType
		return &c
	}

	// Empty and commercial select the same cloud, so switching between them must not reset
	// every user's connection.
	assert.False(t, withTenantType("").differsOAuth2(withTenantType(TenantTypeCommercial)))
	assert.False(t, withTenantType(TenantTypeCommercial).differsOAuth2(withTenantType("")))
	assert.False(t, withTenantType(TenantTypeGCCHigh).differsOAuth2(withTenantType(TenantTypeGCCHigh)))

	// Moving between clouds invalidates stored tokens.
	assert.True(t, withTenantType("").differsOAuth2(withTenantType(TenantTypeGCCHigh)))
	assert.True(t, withTenantType(TenantTypeCommercial).differsOAuth2(withTenantType(TenantTypeGCCHigh)))
	assert.True(t, withTenantType(TenantTypeGCCHigh).differsOAuth2(withTenantType(TenantTypeDoD)))
}
