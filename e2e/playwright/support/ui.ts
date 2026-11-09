// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {expect, type ChannelsPage, type PlaywrightExtended} from '@mattermost/playwright-lib';
import type {BrowserContext} from '@playwright/test';

import {loginMember, MEMBER1, type Member} from './admin';
import {APP_BAR_ICON, COMMAND, START_HELP_TEXT, TEAM_NAME} from './constants';
import {USER1, type TestUser} from './credentials';
import {FAKE_URL} from './fake';
import {MicrosoftLoginPage} from './ms_login';

const CONNECT_LINK_TEXT = 'Click here to link your Microsoft account.';
const ALREADY_CONNECTED = 'User already connected to MS Teams Meetings';

// The plugin sends the browser to the real login host, so serve it from the fake instead.
// Teams join links point at the real Teams host, so answer those with a stub page.
export async function routeMicrosoftToFake(context: BrowserContext) {
    await context.route(/^https:\/\/login\.microsoftonline\.com\//, async (route) => {
        const url = route.request().url().replace('https://login.microsoftonline.com', FAKE_URL);
        const response = await route.fetch({url, maxRedirects: 0});
        await route.fulfill({response});
    });

    await context.route(/^https:\/\/teams\.microsoft\.com\//, (route) => route.fulfill({
        contentType: 'text/html',
        body: '<!doctype html><title>Fake Teams</title><h1>Fake Teams meeting</h1>',
    }));
}

// Each test works in its own channel, because the plugin offers to join a meeting created in the same channel in the last 30 seconds.
export async function createChannel(...others: Member[]) {
    const {client, user} = await loginMember(MEMBER1);
    const team = await client.getTeamByName(TEAM_NAME);
    const name = `e2e-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`;
    const channel = await client.createChannel({team_id: team.id, name, display_name: name, type: 'O'} as Parameters<typeof client.createChannel>[0]);

    for (const other of others) {
        // eslint-disable-next-line no-await-in-loop
        const profile = await client.getUserByUsername(other.username);

        // eslint-disable-next-line no-await-in-loop
        await client.addToChannel(profile.id, channel.id);
    }
    return {name, id: channel.id, userId: user.id};
}

export async function openChannel(pw: PlaywrightExtended, channelName: string, member: Member = MEMBER1) {
    const {user} = await loginMember(member);
    const {channelsPage} = await pw.testBrowser.login(user);
    await routeMicrosoftToFake(channelsPage.page.context());
    await channelsPage.goto(TEAM_NAME, channelName);
    await channelsPage.toBeVisible();
    return channelsPage;
}

export async function openNewChannel(pw: PlaywrightExtended, member: Member = MEMBER1) {
    const channel = await createChannel(...(member === MEMBER1 ? [] : [member]));
    return {channelsPage: await openChannel(pw, channel.name, member), channel};
}

// Clicking send rather than pressing Enter, which would pick a slash-command autocomplete suggestion.
export async function submitSlashCommand(channelsPage: ChannelsPage, command: string) {
    const postCreate = channelsPage.centerView.postCreate;
    await postCreate.input.fill(command);
    await postCreate.sendMessageButton.click();
}

export async function runSlashCommand(channelsPage: ChannelsPage, command: string) {
    await submitSlashCommand(channelsPage, command);
    await expect(channelsPage.centerView.postCreate.input).toHaveValue('');
}

// Earlier commands can leave an identical reply on screen, so wait for a new last post.
export async function runCommandExpecting(channelsPage: ChannelsPage, command: string, reply: string | RegExp, replyTimeout?: number) {
    const centerView = channelsPage.centerView;
    const before = await centerView.getLastPostID();
    await runSlashCommand(channelsPage, command);
    await expect.poll(() => centerView.getLastPostID()).not.toBe(before);

    const post = await centerView.getLastPost();
    await expect(post.body).toContainText(reply, {timeout: replyTimeout});
    return post;
}

export function connectLink(channelsPage: ChannelsPage) {
    return channelsPage.centerView.container.getByRole('link', {name: CONNECT_LINK_TEXT}).last();
}

// The connect link opens in a new tab, and Playwright routes never see redirect targets, so resolve the
// plugin's redirect here and navigate a new page straight to the Microsoft authorize URL.
export async function followConnectLink(channelsPage: ChannelsPage) {
    const page = channelsPage.page;
    const href = await connectLink(channelsPage).getAttribute('href');
    if (!href) {
        throw new Error('The connect link has no href');
    }

    const connect = await page.request.get(href, {maxRedirects: 0});
    const authorizeUrl = connect.headers().location;
    if (connect.status() !== 302 || !authorizeUrl) {
        throw new Error(`Expected a redirect from ${href}, got ${connect.status()}`);
    }

    const loginPage = await page.context().newPage();
    await loginPage.goto(authorizeUrl);
    return new MicrosoftLoginPage(loginPage);
}

// Signs in through the link in the latest connect reply, then closes the sign-in tab.
export async function completeConnect(channelsPage: ChannelsPage, user: TestUser = USER1) {
    const login = await followConnectLink(channelsPage);
    await login.signIn(user);
    await login.page.close();
}

export async function connectAccount(channelsPage: ChannelsPage, user: TestUser = USER1) {
    await runSlashCommand(channelsPage, `${COMMAND} connect`);
    await expect(connectLink(channelsPage)).toBeVisible();
    await completeConnect(channelsPage, user);
    await expect(channelsPage.centerView.container.getByText('You have successfully connected to MS Teams Meetings.').last()).toBeVisible();
}

// Connects unless the member already is, so specs do not depend on the order they run in.
export async function ensureConnected(channelsPage: ChannelsPage, user: TestUser = USER1) {
    await runSlashCommand(channelsPage, `${COMMAND} connect`);
    const already = channelsPage.centerView.container.getByText(ALREADY_CONNECTED).last();
    const link = connectLink(channelsPage);
    await expect(already.or(link)).toBeVisible();

    if (await link.isVisible()) {
        await completeConnect(channelsPage, user);
        await expect(channelsPage.centerView.container.getByText('You have successfully connected to MS Teams Meetings.').last()).toBeVisible();
    }
}

export async function ensureDisconnected(channelsPage: ChannelsPage) {
    await runCommandExpecting(channelsPage, `${COMMAND} disconnect`, /successfully disconnected|Failed to disconnect/);
}

export function headerStartButton(channelsPage: ChannelsPage) {
    return channelsPage.page.locator('#channel-header').getByRole('button', {name: START_HELP_TEXT});
}

export function appBarStartButton(channelsPage: ChannelsPage) {
    return channelsPage.page.locator(APP_BAR_ICON);
}
