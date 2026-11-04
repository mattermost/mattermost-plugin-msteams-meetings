// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test} from '@mattermost/playwright-lib';

import {COMMAND} from '../support/constants';
import {MEMBER1} from '../support/admin';
import {fake, waitForMeetingCount} from '../support/fake';
import {MeetingPost} from '../support/meeting_post';
import {ensureConnected, openNewChannel, runSlashCommand} from '../support/ui';

/**
 * @objective Verify starting a second meeting soon after the first offers to join the existing one or create a new one
 */
test('offers to join a recently created meeting', {tag: ['@msteams_plugin', '@msteams_recent']}, async ({pw}) => {
    await fake.clearMeetings();
    const {channelsPage} = await openNewChannel(pw);
    await ensureConnected(channelsPage);

    // # Start a meeting, then start another right away
    await runSlashCommand(channelsPage, `${COMMAND} start First`);
    await expect(new MeetingPost(channelsPage).title).toHaveText('First');
    const [first] = await waitForMeetingCount(1);
    await runSlashCommand(channelsPage, `${COMMAND} start Second`);

    // * Verify the plugin names the creator and offers both choices without creating a meeting
    const post = new MeetingPost(channelsPage);
    await expect(post.pretext).toContainText(`${MEMBER1.username} already created a MS Teams Meeting recently`);
    await expect(post.subtitle).toHaveText('Would you like to join, or create your own meeting?');
    await expect(post.joinExistingButton).toHaveAttribute('href', first.response.joinUrl);
    expect(await fake.meetings()).toHaveLength(1);

    // # Choose to create a new meeting
    await post.createNewButton.click();

    // * Verify a second meeting is created and posted
    const meetings = await waitForMeetingCount(2);
    await expect(post.joinButton).toHaveAttribute('href', meetings[1].response.joinUrl);
    expect(meetings[1].response.joinUrl).not.toBe(first.response.joinUrl);
});
