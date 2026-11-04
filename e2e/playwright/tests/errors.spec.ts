// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test} from '@mattermost/playwright-lib';

import {loginAdmin, loginMember} from '../support/admin';
import {COMMAND, PLUGIN_ID} from '../support/constants';
import {fake} from '../support/fake';
import {setAppBarDisabled} from '../support/plugin';
import {ensureConnected, headerStartButton, openNewChannel, runCommandExpecting} from '../support/ui';

const GRAPH_FAILURE = {status: 403, code: 'Forbidden', message: 'No application access policy found for this app.'};

test.beforeEach(async () => {
    await fake.clearMeetings();
});

test.afterEach(async () => {
    await fake.clearFailNextCreate();
});

/**
 * @objective Verify a Microsoft failure while creating a meeting is shown from the slash command
 */
test('reports a failed meeting from the slash command', {tag: ['@msteams_plugin', '@msteams_errors']}, async ({pw}) => {
    const {channelsPage} = await openNewChannel(pw);
    await ensureConnected(channelsPage);

    // # Make Microsoft refuse the next meeting, then start one
    await fake.failNextCreate(GRAPH_FAILURE);

    // * Verify the plugin tells the member to try again
    await runCommandExpecting(channelsPage, `${COMMAND} start`, 'Failed to post message. Please try again.');
    expect(await fake.meetings()).toHaveLength(0);
});

test.describe('channel header button', () => {
    test.beforeAll(async () => {
        const {client} = await loginAdmin();
        await setAppBarDisabled(client, true);
    });

    test.afterAll(async () => {
        const {client} = await loginAdmin();
        await setAppBarDisabled(client, false);
    });

    /**
     * @objective Verify a Microsoft failure while creating a meeting is shown from the channel header button
     */
    test('reports a failed meeting from the channel header button', {tag: ['@msteams_plugin', '@msteams_errors']}, async ({pw}) => {
        const {channelsPage} = await openNewChannel(pw);
        await ensureConnected(channelsPage);

        // # Make Microsoft refuse the next meeting, then click the channel header button
        await fake.failNextCreate(GRAPH_FAILURE);
        await headerStartButton(channelsPage).click();

        // * Verify the Microsoft error message is shown in the channel
        await expect(channelsPage.centerView.container.getByText(GRAPH_FAILURE.message).last()).toBeVisible();
    });
});

/**
 * @objective Verify the plugin API rejects forged or unauthenticated requests
 */
test('rejects forged and unauthenticated plugin requests', {tag: ['@msteams_plugin', '@msteams_errors']}, async ({baseURL}) => {
    const {client} = await loginMember();
    const pluginUrl = `${baseURL}/plugins/${PLUGIN_ID}`;
    const headers = {Authorization: `Bearer ${client.getToken()}`};

    // # Complete the OAuth flow with a forged state
    const forged = await fetch(`${pluginUrl}/oauth2/complete?code=forged&state=forged`, {headers});

    // * Verify the plugin rejects the state
    expect(forged.status).toBe(400);
    expect(await forged.text()).toContain('invalid state');

    // # Start a meeting without a session
    const anonymous = await fetch(`${pluginUrl}/api/v1/meetings`, {method: 'POST', body: '{}'});

    // * Verify the plugin refuses the request
    expect(anonymous.status).toBe(401);
});
