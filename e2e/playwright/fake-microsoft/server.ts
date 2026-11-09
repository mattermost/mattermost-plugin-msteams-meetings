// A fake of Microsoft Entra ID sign-in and the Microsoft Graph (beta) calls the plugin makes.
// It listens on HTTPS 443 for Mattermost (reached through the Compose network aliases
// login.microsoftonline.com and graph.microsoft.com) and on HTTP 8080 for the browser
// and the test control API. Uses only Node built-ins, with type annotations that Node strips.
import crypto from 'node:crypto';
import fs from 'node:fs';
import http from 'node:http';
import https from 'node:https';

import {missingFields} from './contract.ts';

const CLIENT_ID = process.env.FAKE_CLIENT_ID || 'fake-client-id';
const CLIENT_SECRET = process.env.FAKE_CLIENT_SECRET || 'fake-client-secret';
const REDIRECT_URIS = (process.env.FAKE_REDIRECT_URIS || 'http://localhost:8065/plugins/com.mattermost.msteamsmeetings/oauth2/complete').split(',');
const JOIN_URL_PREFIX = 'https://teams.microsoft.com/l/meetup-join/';

type Json = Record<string, any>;
type Result = {status: number; body?: Json | string; headers?: Record<string, string>};

// carol has no mail address, which the plugin rejects while connecting.
const USERS = [
    {upn: 'alice@contoso.test', name: 'Alice Fake', password: 'Alice-Fake-Pass1', hasMail: true},
    {upn: 'bob@contoso.test', name: 'Bob Fake', password: 'Bob-Fake-Pass1', hasMail: true},
    {upn: 'carol@contoso.test', name: 'Carol Fake', password: 'Carol-Fake-Pass1', hasMail: false},
].map((u) => ({...u, id: crypto.createHash('sha1').update(u.upn).digest('hex').replace(/^(.{8})(.{4})(.{4})(.{4})(.{12}).*$/, '$1-$2-$3-$4-$5')}));
type User = typeof USERS[number];

type AccessToken = {user: User | null; scope: string; expires: number};
type Code = {user: User; redirectUri: string; scope: string; expires: number};
type Pending = {query: URLSearchParams; user?: User};
type Meeting = {organizer: string; request: Json; response: Json};
type FailNext = {status: number; code: string; message: string};

const defaultConfig = () => ({accessTokenTtlSeconds: 3600, adminConsent: true, appOnlyMeetings: true});

const state = {
    accessTokens: new Map<string, AccessToken>(),
    refreshTokens: new Map<string, {user: User; scope: string}>(),
    codes: new Map<string, Code>(),
    pending: new Map<string, Pending>(),
    meetings: new Map<string, Meeting>(),
    failNext: null as FailNext | null,
    requests: [] as Json[],
    violations: [] as string[],
    config: defaultConfig(),
};

function reset() {
    state.accessTokens.clear();
    state.refreshTokens.clear();
    state.codes.clear();
    state.pending.clear();
    state.meetings.clear();
    state.failNext = null;
    state.requests = [];
    state.violations = [];
    state.config = defaultConfig();
}

const uid = () => crypto.randomUUID();
const token = (prefix: string) => `${prefix}.${crypto.randomBytes(24).toString('base64url')}`;
const html = (s: string) => s.replace(/[&<>"]/g, (c) => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;'}[c] as string));

function findUser(idOrUpn: string): User | undefined {
    const key = decodeURIComponent(idOrUpn).toLowerCase();
    return USERS.find((u) => u.id === key || u.upn === key);
}

function validate(contractKey: string, value: unknown, where: string) {
    const missing = missingFields(contractKey, value);
    if (missing.length > 0) {
        state.violations.push(`${where}: missing ${missing.join(', ')}`);
    }
}

// ---- Entra ID sign-in pages ----

function page(body: string): Result {
    return {
        status: 200,
        headers: {'content-type': 'text/html; charset=utf-8'},
        body: `<!doctype html><html><head><title>Sign in to your account</title></head><body>${body}</body></html>`,
    };
}

function errorPage(status: number, message: string): Result {
    return {status, headers: {'content-type': 'text/html; charset=utf-8'}, body: `<!doctype html><html><body><h1>Sign in error</h1><p id="service_exception_message" role="alert">${html(message)}</p></body></html>`};
}

function hidden(sid: string) {
    return `<input type="hidden" name="sid" value="${sid}">`;
}

function emailPage(tenant: string, sid: string, error = ''): Result {
    return page(`<form method="post" action="/${tenant}/login/email">${hidden(sid)}
        <h1>Sign in</h1>${error ? `<div id="usernameError" role="alert">${html(error)}</div>` : ''}
        <input type="email" name="loginfmt" placeholder="Email, phone, or Skype">
        <input type="submit" id="idSIButton9" value="Next"></form>`);
}

function passwordPage(tenant: string, sid: string, email: string, error = ''): Result {
    return page(`<form method="post" action="/${tenant}/login/password">${hidden(sid)}
        <h1>Enter password</h1><div id="displayName">${html(email)}</div>
        ${error ? `<div id="passwordError" role="alert">${html(error)}</div>` : ''}
        <input type="password" name="passwd" placeholder="Password">
        <input type="submit" id="idSIButton9" value="Sign in"></form>`);
}

function consentPage(tenant: string, sid: string): Result {
    return page(`<form method="post" action="/${tenant}/login/consent">${hidden(sid)}
        <h1>Permissions requested</h1><p>Let this app create online meetings on your behalf.</p>
        <button type="submit" id="idSIButton9" name="action" value="accept">Accept</button>
        <button type="submit" id="cancelConsent" name="action" value="deny">Cancel</button></form>`);
}

function kmsiPage(tenant: string, sid: string): Result {
    return page(`<form method="post" action="/${tenant}/login/kmsi">${hidden(sid)}
        <h1>Stay signed in?</h1>
        <button type="submit" id="idSIButton9" name="action" value="yes">Yes</button>
        <button type="submit" id="idBtn_Back" name="action" value="no">No</button></form>`);
}

function redirect(location: string): Result {
    return {status: 302, headers: {location}, body: ''};
}

function redirectWith(redirectUri: string, params: Record<string, string>): Result {
    const url = new URL(redirectUri);
    for (const [key, value] of Object.entries(params)) {
        url.searchParams.set(key, value);
    }
    return redirect(url.toString());
}

function authorize(tenant: string, query: URLSearchParams): Result {
    const clientId = query.get('client_id');
    const redirectUri = query.get('redirect_uri') || '';
    if (clientId !== CLIENT_ID) {
        return errorPage(400, `AADSTS700016: Application with identifier '${clientId}' was not found in the directory.`);
    }
    if (!REDIRECT_URIS.includes(redirectUri)) {
        return errorPage(400, `AADSTS50011: The redirect URI '${redirectUri}' specified in the request does not match the redirect URIs configured for the application '${clientId}'.`);
    }
    if (query.get('response_type') !== 'code') {
        return errorPage(400, 'AADSTS700054: response_type \'code\' is not enabled for the application.');
    }

    const sid = uid();
    state.pending.set(sid, {query});
    return emailPage(tenant, sid);
}

function finishSignIn(pending: Pending): Result {
    const redirectUri = pending.query.get('redirect_uri') as string;
    const code = token('code');
    state.codes.set(code, {
        user: pending.user as User,
        redirectUri,
        scope: pending.query.get('scope') || '',
        expires: Date.now() + (10 * 60 * 1000),
    });
    return redirectWith(redirectUri, {code, state: pending.query.get('state') || '', session_state: uid()});
}

function loginStep(tenant: string, step: string, form: URLSearchParams): Result {
    const sid = form.get('sid') || '';
    const pending = state.pending.get(sid);
    if (!pending) {
        return errorPage(400, 'AADSTS90016: The sign-in session was not found. Start again.');
    }

    switch (step) {
    case 'email': {
        const user = findUser(form.get('loginfmt') || '');
        if (!user) {
            return emailPage(tenant, sid, "We couldn't find an account with that username.");
        }
        pending.user = user;
        return passwordPage(tenant, sid, user.upn);
    }
    case 'password': {
        const user = pending.user;
        if (!user || form.get('passwd') !== user.password) {
            return passwordPage(tenant, sid, user?.upn || '', 'Your account or password is incorrect.');
        }
        if (pending.query.get('prompt') === 'consent' || !state.config.adminConsent) {
            return consentPage(tenant, sid);
        }
        return kmsiPage(tenant, sid);
    }
    case 'consent':
        if (form.get('action') === 'deny') {
            return redirectWith(pending.query.get('redirect_uri') as string, {
                error: 'access_denied',
                error_description: 'AADSTS65004: The user declined to consent to access the app.',
                state: pending.query.get('state') || '',
            });
        }
        return kmsiPage(tenant, sid);
    case 'kmsi':
        state.pending.delete(sid);
        return finishSignIn(pending);
    default:
        return errorPage(404, 'Not found');
    }
}

// ---- Token endpoint ----

function tokenError(status: number, error: string, code: number, description: string): Result {
    return {
        status,
        body: {
            error,
            error_description: `AADSTS${code}: ${description}\r\nTrace ID: ${uid()}\r\nCorrelation ID: ${uid()}\r\nTimestamp: ${new Date().toISOString()}`,
            error_codes: [code],
            timestamp: new Date().toISOString(),
            trace_id: uid(),
            correlation_id: uid(),
        },
    };
}

function issueTokens(user: User | null, scope: string, withRefresh: boolean): Result {
    const accessToken = token('at');
    const ttl = state.config.accessTokenTtlSeconds;
    state.accessTokens.set(accessToken, {user, scope, expires: Date.now() + (ttl * 1000)});

    // Entra omits scope from client-credentials responses.
    const body: Json = {token_type: 'Bearer', expires_in: ttl, ext_expires_in: ttl, access_token: accessToken};
    if (user) {
        body.scope = scope;
    }
    if (withRefresh && user) {
        // Like Entra, rotating a refresh token leaves the previous one valid.
        body.refresh_token = token('rt');
        state.refreshTokens.set(body.refresh_token, {user, scope});
    }
    validate(user ? 'token' : 'appToken', body, 'token endpoint');
    return {status: 200, body};
}

function tokenEndpoint(form: URLSearchParams, authHeader: string): Result {
    let clientId = form.get('client_id');
    let clientSecret = form.get('client_secret');
    if (authHeader.startsWith('Basic ')) {
        const [id, secret] = Buffer.from(authHeader.slice(6), 'base64').toString().split(':');
        clientId = decodeURIComponent(id);
        clientSecret = decodeURIComponent(secret || '');
    }
    if (clientId !== CLIENT_ID) {
        return tokenError(401, 'invalid_client', 700016, `Application with identifier '${clientId}' was not found in the directory.`);
    }
    if (clientSecret !== CLIENT_SECRET) {
        return tokenError(401, 'invalid_client', 7000215, 'Invalid client secret provided. Ensure the secret being sent in the request is the client secret value, not the client secret ID.');
    }

    const grant = form.get('grant_type');
    switch (grant) {
    case 'authorization_code': {
        const code = state.codes.get(form.get('code') || '');
        state.codes.delete(form.get('code') || '');
        if (!code || code.expires < Date.now()) {
            return tokenError(400, 'invalid_grant', 70000, 'The provided value for the \'code\' parameter is not valid. The code has expired.');
        }
        if (code.redirectUri !== form.get('redirect_uri')) {
            return tokenError(400, 'invalid_grant', 50011, 'The redirect URI does not match the one used to obtain the code.');
        }
        return issueTokens(code.user, code.scope, true);
    }
    case 'refresh_token': {
        const refresh = state.refreshTokens.get(form.get('refresh_token') || '');
        if (!refresh) {
            return tokenError(400, 'invalid_grant', 700082, 'The refresh token has expired due to inactivity. The token was issued on 2026-01-01T00:00:00.0000000Z and was inactive for 90.00:00:00.');
        }
        return issueTokens(refresh.user, refresh.scope, true);
    }
    case 'client_credentials':
        if (form.get('scope') !== 'https://graph.microsoft.com/.default') {
            return tokenError(400, 'invalid_scope', 70011, 'The provided request must include a \'scope\' input parameter. The provided value for the input parameter \'scope\' is not valid.');
        }
        return issueTokens(null, 'https://graph.microsoft.com/.default', false);
    default:
        return tokenError(400, 'unsupported_grant_type', 70003, `The app requested an unsupported grant type '${grant}'.`);
    }
}

// ---- Graph (beta) ----

function graphError(status: number, code: string, message: string): Result {
    const body = {error: {code, message, innerError: {date: new Date().toISOString(), 'request-id': uid(), 'client-request-id': uid()}}};
    validate('graphError', body, 'graph error');
    return {status, body};
}

type Auth = {user: User | null; scope: string};

const isResult = (value: unknown): value is Result => typeof value === 'object' && value !== null && 'status' in value;

function authenticate(header: string): Auth | Result {
    const bearer = header.startsWith('Bearer ') ? header.slice(7) : '';
    if (!bearer) {
        return graphError(401, 'InvalidAuthenticationToken', 'Access token is empty.');
    }
    const found = state.accessTokens.get(bearer);
    if (!found || found.expires < Date.now()) {
        return graphError(401, 'InvalidAuthenticationToken', 'Access token has expired or is not yet valid.');
    }
    return {user: found.user, scope: found.scope};
}

function userResult(user: User): Result {
    const body: Json = {
        '@odata.context': 'https://graph.microsoft.com/beta/$metadata#users/$entity',
        id: user.id,
        displayName: user.name,
        givenName: user.name.split(' ')[0],
        surname: user.name.split(' ')[1],
        userPrincipalName: user.upn,
        mail: user.hasMail ? user.upn : null,
    };
    if (user.hasMail) {
        validate('user', body, 'user');
    }
    return {status: 200, body};
}

function resolveUser(auth: Auth, ref: string): User | Result {
    const user = findUser(ref);
    if (!user) {
        return graphError(404, 'Request_ResourceNotFound', `Resource '${decodeURIComponent(ref)}' does not exist or one of its queried reference-property objects are not present.`);
    }
    if (auth.user && auth.user.id !== user.id) {
        return graphError(403, 'Forbidden', 'Insufficient privileges to complete the operation.');
    }
    return user;
}

function createMeeting(auth: Auth, user: User, body: Json): Result {
    if (auth.user && !auth.scope.split(' ').some((s) => s.toLowerCase().endsWith('onlinemeetings.readwrite'))) {
        return graphError(403, 'Forbidden', 'The access token is missing the OnlineMeetings.ReadWrite scope.');
    }
    if (!auth.user && !state.config.appOnlyMeetings) {
        return graphError(403, 'Forbidden', 'No application access policy found for this app.');
    }

    validate('createOnlineMeetingRequest', body, 'create online meeting request');
    // Graph defaults missing times to now and an hour later, but rejects unparsable ones.
    const start = Date.parse(body.startDateTime ?? new Date().toISOString());
    const end = body.endDateTime === undefined ? start + (60 * 60 * 1000) : Date.parse(body.endDateTime);
    if (Number.isNaN(start) || Number.isNaN(end)) {
        return graphError(400, 'BadRequest', 'startDateTime and endDateTime must be valid ISO 8601 date-times.');
    }
    const startDateTime = body.startDateTime ?? new Date(start).toISOString();
    const endDateTime = body.endDateTime ?? new Date(end).toISOString();

    if (state.failNext) {
        const {status, code, message} = state.failNext;
        state.failNext = null;
        return graphError(status, code, message);
    }

    const id = Buffer.from(`1*${uid()}*0**fake`).toString('base64');
    const joinUrl = `${JOIN_URL_PREFIX}fake-${crypto.randomBytes(8).toString('hex')}`;
    const response: Json = {
        '@odata.context': 'https://graph.microsoft.com/beta/$metadata#users(\'' + user.id + '\')/onlineMeetings/$entity',
        id,
        creationDateTime: new Date().toISOString(),
        startDateTime,
        endDateTime,
        joinUrl,
        joinWebUrl: joinUrl,
        subject: body.subject,
        participants: {
            ...body.participants,
            organizer: {...body.participants?.organizer, role: 'presenter'},
        },
        chatInfo: {threadId: `19:meeting_${uid()}@thread.v2`},
    };
    validate('onlineMeeting', response, 'online meeting response');
    state.meetings.set(id, {organizer: user.upn, request: body, response});
    return {status: 201, body: response};
}

function graphRoute(method: string, pathname: string, body: Json, auth: Auth): Result {
    const segments = pathname.replace(/^\/beta/, '').split('/').filter(Boolean);
    const [root, ref, sub, subId] = segments;

    if (root === 'me' && segments.length === 1 && method === 'GET') {
        if (!auth.user) {
            return graphError(400, 'BadRequest', '/me request is only valid with delegated authentication flow.');
        }
        return userResult(auth.user);
    }

    if (root === 'users' && ref) {
        const user = resolveUser(auth, ref);
        if (isResult(user)) {
            return user;
        }
        if (!sub && method === 'GET') {
            return userResult(user);
        }
        if (sub === 'onlineMeetings' && !subId && method === 'POST') {
            return createMeeting(auth, user, body);
        }
        if (sub === 'onlineMeetings' && subId) {
            const meeting = state.meetings.get(decodeURIComponent(subId));
            if (!meeting || meeting.organizer !== user.upn) {
                return graphError(404, 'NotFound', 'Meeting not found.');
            }
            if (method === 'GET') {
                return {status: 200, body: meeting.response};
            }
            if (method === 'DELETE') {
                state.meetings.delete(decodeURIComponent(subId));
                return {status: 204};
            }
        }
    }
    return graphError(404, 'BadRequest', `Resource not found for the segment '${root || ''}'.`);
}

// ---- HTTP plumbing ----

async function readBody(req: http.IncomingMessage): Promise<string> {
    const chunks: Buffer[] = [];
    for await (const chunk of req) {
        chunks.push(chunk as Buffer);
    }
    return Buffer.concat(chunks).toString();
}

function send(res: http.ServerResponse, result: Result) {
    const isString = typeof result.body === 'string';
    const payload = result.body === undefined ? '' : isString ? result.body as string : JSON.stringify(result.body);
    res.writeHead(result.status, {
        'content-type': isString ? 'text/plain' : 'application/json',
        ...result.headers,
        'content-length': Buffer.byteLength(payload),
    });
    res.end(payload);
}

function control(method: string, pathname: string, body: Json): Result {
    const [, section, ref] = pathname.split('/');

    if (section === 'health') {
        return {status: 200, body: {ok: true}};
    }
    if (section === 'reset' && method === 'POST') {
        reset();
        return {status: 200, body: {ok: true}};
    }
    if (section === 'config' && method === 'POST') {
        Object.assign(state.config, body);
        return {status: 200, body: state.config};
    }
    if (section === 'requests') {
        if (method === 'DELETE') {
            state.requests = [];
            return {status: 200, body: {ok: true}};
        }
        return {status: 200, body: {value: state.requests}};
    }
    if (section === 'violations') {
        if (method === 'DELETE') {
            state.violations = [];
            return {status: 200, body: {ok: true}};
        }
        return {status: 200, body: {value: state.violations}};
    }
    if (section === 'tokens' && method === 'POST') {
        if (ref === 'revoke') {
            state.refreshTokens.clear();
        }
        return {status: 200, body: {ok: true}};
    }
    if (section === 'fail-next-create' && method === 'DELETE') {
        state.failNext = null;
        return {status: 200, body: {ok: true}};
    }
    if (section === 'fail-next-create' && method === 'POST') {
        state.failNext = {status: body.status || 500, code: body.code || 'InternalServerError', message: body.message || 'Simulated failure.'};
        return {status: 200, body: {ok: true}};
    }
    if (section === 'meetings') {
        if (method === 'DELETE') {
            state.meetings.clear();
            return {status: 200, body: {ok: true}};
        }
        return {status: 200, body: {value: [...state.meetings.values()]}};
    }
    return {status: 404, body: {error: 'unknown control route'}};
}

async function handle(req: http.IncomingMessage, res: http.ServerResponse) {
    const url = new URL(req.url || '/', 'http://fake');
    const method = req.method || 'GET';
    const raw = await readBody(req);
    const contentType = req.headers['content-type'] || '';
    const isJson = contentType.includes('json');
    const form = new URLSearchParams(isJson ? '' : raw);
    let json: Json = {};
    if (isJson && raw) {
        try {
            json = JSON.parse(raw);
        } catch {
            json = {};
        }
    }

    let result: Result;
    const logEntry: Json = {method, path: url.pathname, time: new Date().toISOString()};

    if (url.pathname.startsWith('/__control/')) {
        return send(res, control(method, url.pathname.replace('/__control', ''), json));
    }

    const match = url.pathname.match(/^\/([^/]+)\/(?:oauth2\/v2\.0\/(authorize|token)|login\/(email|password|consent|kmsi))$/);
    if (match) {
        const [, tenant, oauthStep, loginStepName] = match;
        if (oauthStep === 'authorize' && method === 'GET') {
            result = authorize(tenant, url.searchParams);
        } else if (oauthStep === 'token' && method === 'POST') {
            logEntry.grant = form.get('grant_type');
            result = tokenEndpoint(form, String(req.headers.authorization || ''));
        } else if (loginStepName && method === 'POST') {
            result = loginStep(tenant, loginStepName, form);
        } else {
            result = errorPage(405, 'Method not allowed');
        }
    } else if (url.pathname.startsWith('/beta/')) {
        const auth = authenticate(String(req.headers.authorization || ''));
        if (isResult(auth)) {
            result = auth;
        } else {
            logEntry.user = auth.user?.upn || 'app-only';
            result = graphRoute(method, url.pathname, json, auth);
        }
    } else {
        result = {status: 404, body: 'Not found'};
    }

    logEntry.status = result.status;
    state.requests.push(logEntry);
    send(res, result);
}

const onRequest = (req: http.IncomingMessage, res: http.ServerResponse) => {
    handle(req, res).catch((err) => {
        console.error(err);
        send(res, {status: 500, body: {error: String(err)}});
    });
};

http.createServer(onRequest).listen(8080, () => console.log('fake microsoft: http on 8080'));

const certDir = process.env.FAKE_CERT_DIR || '/certs';
if (fs.existsSync(`${certDir}/fake.crt`)) {
    https.createServer({cert: fs.readFileSync(`${certDir}/fake.crt`), key: fs.readFileSync(`${certDir}/fake.key`)}, onRequest).listen(443, () => console.log('fake microsoft: https on 443'));
}
