// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test} from '@mattermost/playwright-lib';

import {COMMAND, JOIN_URL_PREFIX} from '../support/constants';
import {USER1} from '../support/credentials';
import {fake, waitForMeetingCount} from '../support/fake';
import {MeetingPost} from '../support/meeting_post';
import {loginAdmin} from '../support/admin';
import {setAppBarDisabled} from '../support/plugin';
import {appBarStartButton, completeConnect, ensureConnected, ensureDisconnected, headerStartButton, openNewChannel, runSlashCommand} from '../support/ui';

test.beforeEach(async () => {
    await fake.clearMeetings();
});

/**
 * @objective Verify `/mstmeetings start` posts a meeting with the topic, created for the member's Microsoft account
 */
test('starts a meeting with a topic from the slash command', {tag: ['@msteams_plugin', '@msteams_start']}, async ({pw}) => {
    const {channelsPage} = await openNewChannel(pw);
    await ensureConnected(channelsPage);

    // # Start a meeting with a topic
    await runSlashCommand(channelsPage, `${COMMAND} start Weekly sync`);

    // * Verify the post is titled with the topic and offers a join button
    const post = new MeetingPost(channelsPage);
    await expect(post.title).toHaveText('Weekly sync');
    await expect(post.pretext).toContainText('I have started a meeting');

    // * Verify Microsoft created the meeting for the connected member and the button joins it
    const [meeting] = await waitForMeetingCount(1);
    expect(meeting.organizer).toBe(USER1.email);
    expect(meeting.response.joinUrl).toContain(JOIN_URL_PREFIX);
    await expect(post.joinButton).toHaveAttribute('href', meeting.response.joinUrl);

    // # Click the join button
    // * Verify it opens the Teams join link
    const popupPromise = channelsPage.page.waitForEvent('popup');
    await post.joinButton.click();
    await expect(await popupPromise).toHaveURL(meeting.response.joinUrl);
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
     * @objective Verify the channel header button starts a meeting and opens the join link
     */
    test('starts a meeting from the channel header button', {tag: ['@msteams_plugin', '@msteams_start']}, async ({pw}) => {
        const {channelsPage} = await openNewChannel(pw);
        await ensureConnected(channelsPage);

        // # Click the channel header button
        const popupPromise = channelsPage.page.waitForEvent('popup');
        await headerStartButton(channelsPage).click();

        // * Verify a meeting post appears with the default title
        const post = new MeetingPost(channelsPage);
        await expect(post.title).toHaveText('MS Teams Meeting');

        // * Verify the join link opened in a new tab
        const [meeting] = await waitForMeetingCount(1);
        await expect(await popupPromise).toHaveURL(meeting.response.joinUrl);
        await expect(post.joinButton).toHaveAttribute('href', meeting.response.joinUrl);
    });
});

/**
 * @objective Verify the app bar icon starts a meeting
 */
test('starts a meeting from the app bar icon', {tag: ['@msteams_plugin', '@msteams_start']}, async ({pw}) => {
    const {channelsPage} = await openNewChannel(pw);
    await ensureConnected(channelsPage);

    // # Click the app bar icon
    await appBarStartButton(channelsPage).click();

    // * Verify a meeting post appears
    await expect(new MeetingPost(channelsPage).title).toHaveText('MS Teams Meeting');
    await waitForMeetingCount(1);
});

/**
 * @objective Verify a member who is not connected is asked to connect and the meeting is posted once they do
 */
test('posts the meeting after connecting from a start request', {tag: ['@msteams_plugin', '@msteams_start']}, async ({pw}) => {
    const {channelsPage} = await openNewChannel(pw);
    await ensureDisconnected(channelsPage);

    // # Start a meeting without being connected
    await runSlashCommand(channelsPage, `${COMMAND} start Kickoff`);

    // * Verify the plugin offers the connect link and creates no meeting
    await expect(channelsPage.centerView.container.getByRole('link', {name: 'Click here to link your Microsoft account.'}).last()).toBeVisible();
    expect(await fake.meetings()).toHaveLength(0);

    // # Sign in through the link
    await completeConnect(channelsPage, USER1);

    // * Verify the meeting is posted without starting it again
    await expect(new MeetingPost(channelsPage).pretext).toContainText('I have started a meeting');
    await waitForMeetingCount(1);
});
