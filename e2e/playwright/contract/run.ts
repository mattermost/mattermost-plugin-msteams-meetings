// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Runs the same Graph (beta) scenarios against the fake and/or the live tenant and checks that
// each response satisfies the contract and that both sides agree on status codes.
//   npm run contract:fake      fake only (needs the compose stack, or node fake-microsoft/server.ts)
//   npm run contract:live      live tenant only (needs MSTEAMS_E2E_* secrets)
//   npm run contract:compare   both, and fail when they disagree
import '../support/env';

import {missingFields} from '../fake-microsoft/contract.ts';

type Target = {name: string; graph: string; tokenUrl: string; clientId: string; clientSecret: string; user1: string; user2: string};
type Step = {name: string; status: number; violations: string[]};

const FAKE_URL = process.env.FAKE_URL || 'http://127.0.0.1:8080';

function fakeTarget(): Target {
    return {
        name: 'fake',
        graph: `${FAKE_URL}/beta`,
        tokenUrl: `${FAKE_URL}/fake-tenant/oauth2/v2.0/token`,
        clientId: 'fake-client-id',
        clientSecret: 'fake-client-secret',
        user1: 'alice@contoso.test',
        user2: 'bob@contoso.test',
    };
}

function liveTarget(): Target {
    const required = (name: string) => {
        const value = process.env[name];
        if (!value) {
            throw new Error(`Set ${name} to run the live contract checks (see e2e/playwright/README.md).`);
        }
        return value;
    };
    return {
        name: 'live',
        graph: 'https://graph.microsoft.com/beta',
        tokenUrl: `https://login.microsoftonline.com/${encodeURIComponent(required('MSTEAMS_E2E_TENANT_ID'))}/oauth2/v2.0/token`,
        clientId: required('MSTEAMS_E2E_CLIENT_ID'),
        clientSecret: required('MSTEAMS_E2E_CLIENT_SECRET'),
        user1: required('MSTEAMS_E2E_USER1_EMAIL'),
        user2: required('MSTEAMS_E2E_USER2_EMAIL'),
    };
}

async function appToken(target: Target): Promise<{status: number; body: any}> {
    const response = await fetch(target.tokenUrl, {
        method: 'POST',
        body: new URLSearchParams({client_id: target.clientId, client_secret: target.clientSecret, scope: 'https://graph.microsoft.com/.default', grant_type: 'client_credentials'}),
    });
    return {status: response.status, body: await response.json()};
}

async function call(target: Target, token: string | undefined, method: string, path: string, body?: unknown) {
    const headers: Record<string, string> = {'Content-Type': 'application/json'};
    if (token) {
        headers.Authorization = `Bearer ${token}`;
    }
    const response = await fetch(`${target.graph}${path}`, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)});
    const text = await response.text();
    return {status: response.status, body: text ? JSON.parse(text) : undefined};
}

function check(name: string, status: number, contract: string | undefined, body: unknown): Step {
    const violations = contract ? missingFields(contract, body).map((f) => `missing ${f}`) : [];
    return {name, status, violations};
}

async function runScenarios(target: Target): Promise<Step[]> {
    const steps: Step[] = [];

    const tokenResult = await appToken(target);
    steps.push(check('client credentials token', tokenResult.status, 'appToken', tokenResult.body));
    const token = tokenResult.body.access_token as string;

    const user1 = await call(target, token, 'GET', `/users/${encodeURIComponent(target.user1)}`);
    steps.push(check('get organizer', user1.status, 'user', user1.body));
    const user2 = await call(target, token, 'GET', `/users/${encodeURIComponent(target.user2)}`);
    steps.push(check('get attendee', user2.status, 'user', user2.body));

    let meetingId = '';
    try {
        const start = new Date();
        const end = new Date(start.getTime() + (60 * 60 * 1000));
        const created = await call(target, token, 'POST', `/users/${user1.body.id}/onlineMeetings`, {
            startDateTime: start.toISOString(),
            endDateTime: end.toISOString(),
            subject: `[contract-${Date.now().toString(36)}] contract check`,
            participants: {
                organizer: {identity: {user: {id: user1.body.id}}, upn: user1.body.userPrincipalName},
                attendees: [{identity: {user: {id: user2.body.id}}, upn: user2.body.userPrincipalName}],
            },
        });
        meetingId = created.body?.id || '';
        steps.push(check('create online meeting', created.status, 'onlineMeeting', created.body));

        if (target.name === 'fake') {
            await fetch(`${FAKE_URL}/__control/violations`, {method: 'DELETE'});
        }
        const noToken = await call(target, undefined, 'GET', `/users/${user1.body.id}`);
        steps.push(check('error: missing token', noToken.status, 'graphError', noToken.body));

        const unknown = await call(target, token, 'GET', '/users/00000000-0000-0000-0000-000000000000');
        steps.push(check('error: unknown user', unknown.status, 'graphError', unknown.body));

        const invalid = await call(target, token, 'POST', `/users/${user1.body.id}/onlineMeetings`, {subject: 'missing times'});
        steps.push(check('error: invalid meeting body', invalid.status, 'graphError', invalid.body));
        if (target.name === 'fake') {
            await fetch(`${FAKE_URL}/__control/violations`, {method: 'DELETE'});
        }
    } finally {
        if (meetingId) {
            const deleted = await call(target, token, 'DELETE', `/users/${user1.body.id}/onlineMeetings/${encodeURIComponent(meetingId)}`);
            steps.push({
                name: 'delete online meeting',
                status: deleted.status,
                violations: deleted.status === 204 ? [] : [`expected 204, got ${deleted.status}`],
            });
        }
    }

    return steps;
}

function report(target: Target, steps: Step[]): boolean {
    let ok = true;
    // eslint-disable-next-line no-console
    console.log(`\n${target.name}`);
    for (const step of steps) {
        const bad = step.violations.length > 0;
        ok = ok && !bad;
        // eslint-disable-next-line no-console
        console.log(`  ${bad ? 'FAIL' : 'ok  '} ${step.name} (${step.status})${bad ? `: ${step.violations.join(', ')}` : ''}`);
    }
    return ok;
}

async function main() {
    const mode = process.argv[2] || 'compare';
    const targets = mode === 'fake' ? [fakeTarget()] : mode === 'live' ? [liveTarget()] : [fakeTarget(), liveTarget()];

    const results = new Map<string, Step[]>();
    let ok = true;
    for (const target of targets) {
        // eslint-disable-next-line no-await-in-loop
        const steps = await runScenarios(target);
        results.set(target.name, steps);
        ok = report(target, steps) && ok;
    }

    if (targets.length === 2) {
        const [fakeSteps, liveSteps] = [results.get('fake') as Step[], results.get('live') as Step[]];
        for (const [i, fakeStep] of fakeSteps.entries()) {
            const liveStep = liveSteps[i];
            if (!liveStep || liveStep.name !== fakeStep.name || liveStep.status !== fakeStep.status) {
                ok = false;
                // eslint-disable-next-line no-console
                console.log(`DRIFT ${fakeStep.name}: fake answered ${fakeStep.status}, live answered ${liveStep ? liveStep.status : 'nothing'}`);
            }
        }
        if (fakeSteps.length !== liveSteps.length) {
            ok = false;
            // eslint-disable-next-line no-console
            console.log(`DRIFT the fake ran ${fakeSteps.length} steps and live ran ${liveSteps.length}`);
        }
    }

    process.exit(ok ? 0 : 1);
}

main().catch((err) => {
    // eslint-disable-next-line no-console
    console.error(err);
    process.exit(1);
});
