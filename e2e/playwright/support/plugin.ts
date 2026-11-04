// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {execFileSync} from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

import './env';

import type {Client4} from '@mattermost/client';
import {testConfig} from '@mattermost/playwright-lib';

import {ensureMember, isDisposableServer, loginAdmin, loginMember, MEMBER1, MEMBER2} from './admin';
import {COMMAND, PLUGIN_ID, TEAM_NAME, TOWN_SQUARE} from './constants';
import {FAKE_CLIENT_ID, FAKE_CLIENT_SECRET, FAKE_TENANT_ID} from './credentials';
import {fake} from './fake';

const DIST_DIR = path.resolve(__dirname, '../../../dist');
type PluginSettingValues = Record<string, unknown>;

export function findPluginBundle(): string {
    const bundles = fs.existsSync(DIST_DIR) ? fs.readdirSync(DIST_DIR).filter((f) => f.startsWith(PLUGIN_ID) && f.endsWith('.tar.gz')) : [];
    if (bundles.length === 0) {
        throw new Error(`No plugin bundle found in ${DIST_DIR}. Run \`make dist\` from the repo root first.`);
    }

    return bundles.
        map((f) => path.join(DIST_DIR, f)).
        sort((a, b) => fs.statSync(b).mtimeMs - fs.statSync(a).mtimeMs)[0];
}

// `make dist` can leave plugin.json owner-only (0600), which the server then fails to read.
function withReadablePermissions(bundlePath: string): string {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'msteams-bundle-'));
    execFileSync('tar', ['xzf', bundlePath, '-C', dir]);
    execFileSync('chmod', ['-R', 'u+rwX,go+rX', dir]);
    const fixed = path.join(dir, path.basename(bundlePath));
    execFileSync('tar', ['czf', fixed, '-C', dir, PLUGIN_ID], {env: {...process.env, COPYFILE_DISABLE: '1'}});
    return fixed;
}

export async function installPlugin(client: Client4, bundlePath: string) {
    const bytes = fs.readFileSync(withReadablePermissions(bundlePath));
    const file = new File([bytes], path.basename(bundlePath), {type: 'application/gzip'});
    await client.uploadPlugin(file, true);
}

export async function restartPlugin(client: Client4) {
    await client.disablePlugin(PLUGIN_ID);
    await client.enablePlugin(PLUGIN_ID);
    await waitForPlugin(client);
}

async function waitForPlugin(client: Client4) {
    const deadline = Date.now() + 20000;
    while (Date.now() < deadline) {
        const plugins = await client.getPlugins();
        if (plugins.active.some((plugin) => plugin.id === PLUGIN_ID)) {
            return;
        }
        // eslint-disable-next-line no-await-in-loop
        await new Promise((resolve) => setTimeout(resolve, 500));
    }
    throw new Error(`Timed out waiting for ${PLUGIN_ID} to become active`);
}

export async function patchPluginSettings(client: Client4, patch: PluginSettingValues) {
    const config = await client.getConfig();
    const plugins = (config.PluginSettings.Plugins ?? {}) as Record<string, PluginSettingValues>;
    plugins[PLUGIN_ID] = {...(plugins[PLUGIN_ID] ?? {}), ...patch};
    config.PluginSettings.Plugins = plugins as typeof config.PluginSettings.Plugins;
    await client.updateConfig(config);
}

// Changing any OAuth setting restarts the plugin and wipes every stored connection.
export async function configurePlugin(client: Client4, clientSecret = FAKE_CLIENT_SECRET) {
    await patchPluginSettings(client, {
        oauth2authority: FAKE_TENANT_ID,
        oauth2clientid: FAKE_CLIENT_ID,
        oauth2clientsecret: clientSecret,
    });
}

// ensureTeam keeps a single team so the channel-header buttons render predictably.
export async function ensureTeam(client: Client4) {
    // Other plugin suites sharing this server archive every team but their own, so restore and rejoin ours.
    const existing = await client.getTeamByName(TEAM_NAME).catch(() => undefined);
    if (!existing) {
        await client.createTeam({
            name: TEAM_NAME,
            display_name: 'MS Teams Meetings E2E',
            type: 'O',
        } as Parameters<Client4['createTeam']>[0]);
    } else {
        if (existing.delete_at) {
            await client.unarchiveTeam(existing.id);
        }
        const me = await client.getMe();
        await client.addToTeam(existing.id, me.id);
    }

    const others = (await client.getMyTeams()).filter((team) => team.name !== TEAM_NAME);
    if (others.length === 0) {
        return;
    }

    if (!isDisposableServer() && process.env.PW_DELETE_OTHER_TEAMS !== 'true') {
        throw new Error(`Refusing to delete teams ${others.map((team) => team.name).join(', ')} on ${testConfig.baseURL}. ` +
            'Leave them first, or set PW_DELETE_OTHER_TEAMS=true to allow deleting them on this non-disposable server.');
    }
    await Promise.all(others.map((team) => client.deleteTeam(team.id)));
}

export async function executeCommand(client: Client4, command: string) {
    const team = await client.getTeamByName(TEAM_NAME);
    const channel = await client.getChannelByName(team.id, TOWN_SQUARE);
    return client.executeCommand(command, {channel_id: channel.id, team_id: team.id});
}

export async function disconnectMembers() {
    for (const member of [MEMBER1, MEMBER2]) {
        try {
            // eslint-disable-next-line no-await-in-loop
            const {client} = await loginMember(member);

            // eslint-disable-next-line no-await-in-loop
            await executeCommand(client, `${COMMAND} disconnect`);
        } catch {
            // The member may not exist yet or may already be disconnected.
        }
    }
}

export async function prepareServer() {
    await fake.reset();
    const {client} = await loginAdmin();
    await ensureTeam(client);
    await ensureMember(client, MEMBER1);
    await ensureMember(client, MEMBER2);

    const bundle = findPluginBundle();
    // eslint-disable-next-line no-console
    console.log(`Installing ${bundle}`);
    await installPlugin(client, bundle);
    await configurePlugin(client);
    await restartPlugin(client);

    // A previous run may have left members connected to Microsoft.
    await disconnectMembers();
}

// With the app bar enabled, Mattermost moves plugin channel header buttons into it.
export async function setAppBarDisabled(client: Client4, disabled: boolean) {
    const config = await client.getConfig();
    config.ExperimentalSettings.DisableAppBar = disabled;
    await client.updateConfig(config);
}
