// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import './env';

import {Client4} from '@mattermost/client';
import type {UserProfile} from '@mattermost/types/users';
import {testConfig} from '@mattermost/playwright-lib';

import {TEAM_NAME} from './constants';

const DISPOSABLE_SERVER_URL = 'http://localhost:8065';

export function isDisposableServer() {
    return new URL(testConfig.baseURL).origin === DISPOSABLE_SERVER_URL;
}

// Fixed users, rather than random ones per test, keep Microsoft connections stable across specs.
export const MEMBER1 = {
    username: 'msteams-e2e-user',
    email: 'msteams-e2e-user@example.com',
};
export const MEMBER2 = {
    username: 'msteams-e2e-user2',
    email: 'msteams-e2e-user2@example.com',
};
export type Member = typeof MEMBER1;

// The default password is public, so other servers must supply their own.
function memberPassword() {
    const password = process.env.PW_REGULAR_USER_PASSWORD;
    if (password) {
        return password;
    }
    if (!isDisposableServer()) {
        throw new Error(`Set PW_REGULAR_USER_PASSWORD to run against ${testConfig.baseURL}.`);
    }
    return 'Ms-E2e-Member#2026';
}

async function login(username: string, password: string) {
    const client = new Client4();
    client.setUrl(testConfig.baseURL);
    const profile = await client.login(username, password);
    return {client, user: {...profile, password}};
}

// loginAdmin works outside test fixtures (e.g. in global setup), unlike pw.getAdminClient().
export function loginAdmin() {
    return login(testConfig.adminUsername, testConfig.adminPassword);
}

export function loginMember(member: Member = MEMBER1) {
    return login(member.username, memberPassword());
}

export async function ensureMember(adminClient: Client4, member: Member): Promise<UserProfile> {
    const password = memberPassword();
    let user: UserProfile;
    try {
        user = await adminClient.getUserByUsername(member.username);
    } catch {
        user = await adminClient.createUser({...member, password} as UserProfile, '', '');
    }
    if (user.roles.split(' ').includes('system_admin')) {
        throw new Error(`${member.username} must not be a system admin`);
    }

    const team = await adminClient.getTeamByName(TEAM_NAME);
    await adminClient.addToTeam(team.id, user.id);
    return user;
}
