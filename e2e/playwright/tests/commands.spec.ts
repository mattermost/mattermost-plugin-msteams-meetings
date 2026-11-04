// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {test} from '@mattermost/playwright-lib';

import {COMMAND} from '../support/constants';
import {openNewChannel, runCommandExpecting} from '../support/ui';

/**
 * @objective Verify the help text is shown for the bare command, `help` and an unknown action
 */
test('shows help and rejects unknown actions', {tag: ['@msteams_plugin', '@msteams_commands']}, async ({pw}) => {
    const {channelsPage} = await openNewChannel(pw);

    // # Run the bare command and `help`
    // * Verify the help text lists every action
    await runCommandExpecting(channelsPage, COMMAND, 'Slash Command Help');
    await runCommandExpecting(channelsPage, `${COMMAND} help`, 'Display this help text.');

    // # Run an unknown action
    // * Verify the plugin names the action and repeats the help
    await runCommandExpecting(channelsPage, `${COMMAND} bogus`, 'Unknown action bogus.');
});

/**
 * @objective Verify commands that take no parameters reject extra ones
 */
test('rejects extra parameters', {tag: ['@msteams_plugin', '@msteams_commands']}, async ({pw}) => {
    const {channelsPage} = await openNewChannel(pw);

    // # Add a parameter to `connect` and `disconnect`
    // * Verify the plugin reports too many parameters
    await runCommandExpecting(channelsPage, `${COMMAND} connect now`, 'Too many parameters.');
    await runCommandExpecting(channelsPage, `${COMMAND} disconnect now`, 'Too many parameters.');
});
