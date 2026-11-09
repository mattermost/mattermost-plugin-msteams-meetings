// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import './env';

export type TestUser = {email: string; password: string};

// These match the defaults baked into fake-microsoft/server.ts.
export const FAKE_TENANT_ID = 'fake-tenant';
export const FAKE_CLIENT_ID = 'fake-client-id';
export const FAKE_CLIENT_SECRET = 'fake-client-secret';
export const USER1: TestUser = {email: 'alice@contoso.test', password: 'Alice-Fake-Pass1'};
export const USER2: TestUser = {email: 'bob@contoso.test', password: 'Bob-Fake-Pass1'};
export const NO_MAIL_USER: TestUser = {email: 'carol@contoso.test', password: 'Carol-Fake-Pass1'};
