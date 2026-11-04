// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import './env';

export const FAKE_URL = process.env.FAKE_URL || 'http://127.0.0.1:8080';

export type FakeMeeting = {
    organizer: string;
    request: {
        subject?: string;
        participants?: {
            organizer?: {identity?: {user?: {id?: string}}; upn?: string};
            attendees?: Array<{identity?: {user?: {id?: string}}; upn?: string}>;
        };
    };
    response: {id: string; joinUrl: string; subject: string};
};

export type FakeRequest = {method?: string; path?: string; status?: number; grant?: string; user?: string};

async function control<T>(method: string, path: string, body?: unknown): Promise<T> {
    const response = await fetch(`${FAKE_URL}/__control/${path}`, {
        method,
        headers: {'content-type': 'application/json'},
        body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (!response.ok && response.status !== 204) {
        throw new Error(`Fake ${method} ${path} failed: ${response.status} ${await response.text()}`);
    }
    return (response.status === 204 ? undefined : await response.json()) as T;
}

export const fake = {
    reset: () => control('POST', 'reset'),
    configure: (config: {accessTokenTtlSeconds?: number; adminConsent?: boolean; appOnlyMeetings?: boolean}) => control('POST', 'config', config),

    meetings: async () => (await control<{value: FakeMeeting[]}>('GET', 'meetings')).value,
    clearMeetings: () => control('DELETE', 'meetings'),
    clearFailNextCreate: () => control('DELETE', 'fail-next-create'),
    failNextCreate: (failure: {status: number; code: string; message: string}) => control('POST', 'fail-next-create', failure),

    requests: async () => (await control<{value: FakeRequest[]}>('GET', 'requests')).value,
    clearRequests: () => control('DELETE', 'requests'),
    violations: async () => (await control<{value: string[]}>('GET', 'violations')).value,

    revokeRefreshTokens: () => control('POST', 'tokens/revoke'),
};

export async function waitForMeetingCount(count: number, timeoutMs = 15_000): Promise<FakeMeeting[]> {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
        // eslint-disable-next-line no-await-in-loop
        const meetings = await fake.meetings();
        if (meetings.length >= count) {
            return meetings;
        }
        // eslint-disable-next-line no-await-in-loop
        await new Promise((resolve) => setTimeout(resolve, 300));
    }
    throw new Error(`The fake did not receive ${count} meeting(s) in time`);
}
