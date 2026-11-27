// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2/microsoft"
)

func TestLoginEndpoint(t *testing.T) {
	for _, tc := range []struct {
		name       string
		tenant     string
		tenantType string
		authURL    string
		tokenURL   string
	}{
		{
			name:       "empty tenant type defaults to commercial",
			tenant:     "tenant-id",
			tenantType: "",
			authURL:    "https://login.microsoftonline.com/tenant-id/oauth2/v2.0/authorize",
			tokenURL:   "https://login.microsoftonline.com/tenant-id/oauth2/v2.0/token",
		},
		{
			name:       "commercial",
			tenant:     "tenant-id",
			tenantType: TenantTypeCommercial,
			authURL:    "https://login.microsoftonline.com/tenant-id/oauth2/v2.0/authorize",
			tokenURL:   "https://login.microsoftonline.com/tenant-id/oauth2/v2.0/token",
		},
		{
			name:       "GCC High",
			tenant:     "tenant-id",
			tenantType: TenantTypeGCCHigh,
			authURL:    "https://login.microsoftonline.us/tenant-id/oauth2/v2.0/authorize",
			tokenURL:   "https://login.microsoftonline.us/tenant-id/oauth2/v2.0/token",
		},
		{
			name:       "DoD",
			tenant:     "tenant-id",
			tenantType: TenantTypeDoD,
			authURL:    "https://login.microsoftonline.us/tenant-id/oauth2/v2.0/authorize",
			tokenURL:   "https://login.microsoftonline.us/tenant-id/oauth2/v2.0/token",
		},
		{
			name:       "empty tenant falls back to common",
			tenant:     "",
			tenantType: TenantTypeGCCHigh,
			authURL:    "https://login.microsoftonline.us/common/oauth2/v2.0/authorize",
			tokenURL:   "https://login.microsoftonline.us/common/oauth2/v2.0/token",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := loginEndpoint(tc.tenant, tc.tenantType)
			assert.Equal(t, tc.authURL, endpoint.AuthURL)
			assert.Equal(t, tc.tokenURL, endpoint.TokenURL)
		})
	}
}

func TestLoginEndpointCommercialMatchesAzureADEndpoint(t *testing.T) {
	// Guards backwards compatibility: the commercial cloud must behave exactly like the
	// previously used golang.org/x/oauth2/microsoft endpoint.
	for _, tenant := range []string{"", "tenant-id"} {
		assert.Equal(t, microsoft.AzureADEndpoint(tenant), loginEndpoint(tenant, ""))
		assert.Equal(t, microsoft.AzureADEndpoint(tenant), loginEndpoint(tenant, TenantTypeCommercial))
	}
}

func TestGraphBaseURL(t *testing.T) {
	assert.Equal(t, "https://graph.microsoft.com/beta", graphBaseURL(""))
	assert.Equal(t, "https://graph.microsoft.com/beta", graphBaseURL(TenantTypeCommercial))
	assert.Equal(t, "https://graph.microsoft.us/v1.0", graphBaseURL(TenantTypeGCCHigh))
	assert.Equal(t, "https://dod-graph.microsoft.us/v1.0", graphBaseURL(TenantTypeDoD))
}

func TestValidateTenantType(t *testing.T) {
	for _, valid := range []string{"", TenantTypeCommercial, TenantTypeGCCHigh, TenantTypeDoD} {
		assert.NoError(t, validateTenantType(valid), valid)
	}
	assert.Error(t, validateTenantType("china"))
	assert.Error(t, validateTenantType("GCCH"))
}
