// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"fmt"

	"golang.org/x/oauth2"
)

// Values accepted by the OAuth2TenantType setting, identifying which Microsoft cloud
// (national cloud deployment) the Entra ID tenant lives in.
// See https://learn.microsoft.com/graph/deployments
const (
	// TenantTypeCommercial is the global Microsoft cloud. GCC (moderate) tenants also use it.
	TenantTypeCommercial = "commercial"
	// TenantTypeGCCHigh is Microsoft 365 GCC High (US Government L4).
	TenantTypeGCCHigh = "gcch"
	// TenantTypeDoD is Microsoft 365 DoD (US Government L5).
	TenantTypeDoD = "dod"
)

type cloudEndpoints struct {
	loginHost string
	graphHost string
}

var cloudEndpointsByTenantType = map[string]cloudEndpoints{
	TenantTypeCommercial: {loginHost: "https://login.microsoftonline.com", graphHost: "https://graph.microsoft.com"},
	TenantTypeGCCHigh:    {loginHost: "https://login.microsoftonline.us", graphHost: "https://graph.microsoft.us"},
	TenantTypeDoD:        {loginHost: "https://login.microsoftonline.us", graphHost: "https://dod-graph.microsoft.us"},
}

// validateTenantType returns an error if tenantType is not a known Microsoft cloud.
// An empty value is valid and means commercial, so existing configurations keep working.
func validateTenantType(tenantType string) error {
	if tenantType == "" {
		return nil
	}
	if _, ok := cloudEndpointsByTenantType[tenantType]; !ok {
		return fmt.Errorf("OAuth2TenantType %q is not a supported Microsoft cloud", tenantType)
	}
	return nil
}

func getCloudEndpoints(tenantType string) cloudEndpoints {
	if endpoints, ok := cloudEndpointsByTenantType[tenantType]; ok {
		return endpoints
	}
	return cloudEndpointsByTenantType[TenantTypeCommercial]
}

// loginEndpoint returns the Microsoft identity platform (v2.0) OAuth2 endpoint for the given
// tenant in the given cloud. For the commercial cloud it is identical to
// golang.org/x/oauth2/microsoft.AzureADEndpoint.
func loginEndpoint(tenant, tenantType string) oauth2.Endpoint {
	if tenant == "" {
		tenant = "common"
	}
	base := getCloudEndpoints(tenantType).loginHost + "/" + tenant + "/oauth2/v2.0"
	return oauth2.Endpoint{
		AuthURL:       base + "/authorize",
		TokenURL:      base + "/token",
		DeviceAuthURL: base + "/devicecode",
	}
}

// graphBaseURL returns the Microsoft Graph beta API base URL for the given cloud.
func graphBaseURL(tenantType string) string {
	return getCloudEndpoints(tenantType).graphHost + "/beta"
}
