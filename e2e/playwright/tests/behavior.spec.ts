// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test} from '@mattermost/playwright-lib';

import {loginAdmin} from '../support/admin';
import {COMMAND} from '../support/constants';
import {NO_MAIL_USER, USER1} from '../support/credentials';
import {fake} from '../support/fake';
import {configurePlugin} from '../support/plugin';
import {connectAccount, connectLink, ensureConnected, ensureDisconnected, followConnectLink, openNewChannel, runCommandExpecting, runSlashCommand} from '../support/ui';

const CONNECT_REPLY = 'Click here to link your Microsoft account.';

// The plugin treats an access token as expired shortly before it really does, so a short lifetime forces a refresh.
const SHORT_TOKEN_SECONDS = 15;
const REFRESH_WAIT_MS = 7_000;

test.afterEach(async () => {
    await fake.configure({accessTokenTtlSeconds: 3600, adminConsent: true});
});

/**
 * @objective Verify a wrong password keeps the member on the sign-in page and leaves them disconnected
 */
test('rejects a wrong Microsoft password', {tag: ['@msteams_plugin', '@msteams_behavior']}, async ({pw}) => {
    const {channelsPage} = await openNewChannel(pw);
    await ensureDisconnected(channelsPage);
    await runSlashCommand(channelsPage, `${COMMAND} connect`);

    // # Sign in with a wrong password
    const login = await followConnectLink(channelsPage);
    await login.submitCredentials({email: USER1.email, password: 'not-the-password'});

    // * Verify Microsoft reports the error and the member stays on the password step
    await expect(login.error.first()).toContainText('Your account or password is incorrect.');
    await expect(login.passwordInput).toBeVisible();
    await login.page.close();

    // * Verify the member is still disconnected
    await runCommandExpecting(channelsPage, `${COMMAND} connect`, CONNECT_REPLY);
});

/**
 * @objective Verify declining the consent screen leaves the member disconnected
 *
 * @precondition
 * The tenant has not consented for the app, so the consent screen is shown.
 */
test('stays disconnected when consent is denied', {tag: ['@msteams_plugin', '@msteams_behavior']}, async ({pw}) => {
    await fake.configure({adminConsent: false});
    const {channelsPage} = await openNewChannel(pw);
    await ensureDisconnected(channelsPage);
    await runSlashCommand(channelsPage, `${COMMAND} connect`);

    // # Sign in, then cancel on the consent screen
    const login = await followConnectLink(channelsPage);
    await login.submitCredentials(USER1);
    await expect(login.consentHeading).toBeVisible();
    await login.cancelConsentButton.click();

    // * Verify the plugin rejects the callback because it has no authorization code
    await expect(login.page.getByText('missing authorization code')).toBeVisible();
    await login.page.close();

    // * Verify the member is still disconnected
    await runCommandExpecting(channelsPage, `${COMMAND} connect`, CONNECT_REPLY);
});

/**
 * @objective Verify an account without a mail address cannot be connected
 */
test('refuses a Microsoft account without mail', {tag: ['@msteams_plugin', '@msteams_behavior']}, async ({pw}) => {
    const {channelsPage} = await openNewChannel(pw);
    await ensureDisconnected(channelsPage);
    await runSlashCommand(channelsPage, `${COMMAND} connect`);

    // # Sign in as a user without mail
    const login = await followConnectLink(channelsPage);
    await login.submitCredentials(NO_MAIL_USER);
    await login.staySignedInNoButton.click();

    // * Verify the plugin explains the account is incomplete
    await expect(login.page.getByText('User has no mail')).toBeVisible();
    await login.page.close();

    // * Verify the member is still disconnected
    await runCommandExpecting(channelsPage, `${COMMAND} connect`, CONNECT_REPLY);
});

/**
 * @objective Verify an expired access token is refreshed with the refresh token
 */
test('refreshes an expired access token', {tag: ['@msteams_plugin', '@msteams_behavior']}, async ({pw}) => {
    await fake.configure({accessTokenTtlSeconds: SHORT_TOKEN_SECONDS});
    const {channelsPage} = await openNewChannel(pw);
    await ensureDisconnected(channelsPage);
    await connectAccount(channelsPage, USER1);

    // # Wait until the plugin considers the token expired
    await channelsPage.page.waitForTimeout(REFRESH_WAIT_MS);
    await fake.clearRequests();

    // * Verify the member is still connected
    await runCommandExpecting(channelsPage, `${COMMAND} connect`, 'User already connected to MS Teams Meetings');

    // * Verify the plugin used the refresh token to get a new access token
    expect((await fake.requests()).map((request) => request.grant)).toContain('refresh_token');
});

/**
 * @objective Verify a revoked refresh token asks the member to connect again
 */
test('asks to reconnect after the refresh token is revoked', {tag: ['@msteams_plugin', '@msteams_behavior']}, async ({pw}) => {
    await fake.configure({accessTokenTtlSeconds: SHORT_TOKEN_SECONDS});
    const {channelsPage} = await openNewChannel(pw);
    await ensureDisconnected(channelsPage);
    await connectAccount(channelsPage, USER1);

    // # Revoke the refresh token and wait for the access token to expire
    await fake.revokeRefreshTokens();
    await channelsPage.page.waitForTimeout(REFRESH_WAIT_MS);

    // * Verify the plugin offers the connect link
    await runCommandExpecting(channelsPage, `${COMMAND} connect`, CONNECT_REPLY);
    await expect(connectLink(channelsPage)).toBeVisible();
});

/**
 * @objective Verify changing the Microsoft app settings disconnects every member
 */
test('disconnects members when the app settings change', {tag: ['@msteams_plugin', '@msteams_behavior']}, async ({pw}) => {
    const {channelsPage} = await openNewChannel(pw);
    await ensureConnected(channelsPage, USER1);

    // # Change the client secret and restore it
    const {client} = await loginAdmin();
    try {
        await configurePlugin(client, 'a-different-secret');
    } finally {
        await configurePlugin(client);
    }
    await channelsPage.page.waitForTimeout(3_000);

    // * Verify the member has to connect again
    await runCommandExpecting(channelsPage, `${COMMAND} connect`, CONNECT_REPLY);
});
