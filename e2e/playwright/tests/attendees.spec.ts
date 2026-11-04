// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, test} from '@mattermost/playwright-lib';

import {loginMember, MEMBER1, MEMBER2} from '../support/admin';
import {COMMAND} from '../support/constants';
import {USER1, USER2} from '../support/credentials';
import {fake, waitForMeetingCount} from '../support/fake';
import {MeetingPost} from '../support/meeting_post';
import {ensureConnected, ensureDisconnected, openChannel, runSlashCommand} from '../support/ui';

/**
 * @objective Verify a direct message meeting invites the connected members and skips members who are not connected
 */
test('invites connected members of a direct message', {tag: ['@msteams_plugin', '@msteams_attendees']}, async ({pw}) => {
    await fake.clearMeetings();
    const {client, user: member1} = await loginMember(MEMBER1);
    const {user: member2} = await loginMember(MEMBER2);
    const dm = await client.createDirectChannel([member1.id, member2.id]);
    // # Connect both members to their Microsoft accounts
    const memberTwoPage = await openChannel(pw, dm.name, MEMBER2);
    await ensureConnected(memberTwoPage, USER2);
    const channelsPage = await openChannel(pw, dm.name, MEMBER1);
    await ensureConnected(channelsPage, USER1);

    // # Start a meeting in the direct message
    await runSlashCommand(channelsPage, `${COMMAND} start Pairing`);
    await expect(new MeetingPost(channelsPage).title).toHaveText('Pairing');

    // * Verify both members are sent to Microsoft as attendees, with their Graph user ids
    const [first] = await waitForMeetingCount(1);
    const invited = first.request.participants?.attendees?.map((attendee) => attendee.upn).sort();
    expect(invited).toEqual([USER1.email, USER2.email].sort());
    for (const attendee of first.request.participants?.attendees ?? []) {
        expect(attendee.identity?.user?.id).toBeTruthy();
    }

    // # Disconnect the second member, then force a new meeting
    await ensureDisconnected(memberTwoPage);
    await runSlashCommand(channelsPage, `${COMMAND} start Solo`);
    const post = new MeetingPost(channelsPage);
    await post.createNewButton.click();

    // * Verify only the connected member is invited
    const meetings = await waitForMeetingCount(2);
    const attendees = meetings[1].request.participants?.attendees?.map((attendee) => attendee.upn);
    expect(attendees).toEqual([USER1.email]);
});
