// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test} from '@mattermost/playwright-lib';

import {COMMAND} from '../support/constants';
import {USER1} from '../support/credentials';
import {fake} from '../support/fake';
import {connectAccount, ensureDisconnected, openNewChannel, runCommandExpecting} from '../support/ui';

/**
 * @objective Verify a member can connect and disconnect a Microsoft account through the connect link
 */
test('connects and disconnects a member through the Microsoft sign-in', {tag: ['@msteams_plugin', '@msteams_connect']}, async ({pw}) => {
    const {channelsPage} = await openNewChannel(pw);
    await ensureDisconnected(channelsPage);
    await fake.clearRequests();

    // # Connect through the link in the command reply
    // * Verify the plugin confirms the connection
    await connectAccount(channelsPage, USER1);

    // * Verify the plugin exchanged an authorization code for tokens
    const grants = (await fake.requests()).map((request) => request.grant);
    expect(grants).toContain('authorization_code');

    // # Connect again
    // * Verify the plugin reports the existing connection
    await runCommandExpecting(channelsPage, `${COMMAND} connect`, 'User already connected to MS Teams Meetings');

    // # Disconnect
    // * Verify the plugin confirms the disconnection
    await runCommandExpecting(channelsPage, `${COMMAND} disconnect`, 'You have successfully disconnected from MS Teams Meetings.');

    // # Connect again
    // * Verify the plugin offers the connect link, so the member is disconnected
    await runCommandExpecting(channelsPage, `${COMMAND} connect`, 'Click here to link your Microsoft account.');
});
