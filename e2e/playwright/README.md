# MS Teams Meetings plugin E2E tests

Playwright suite that drives the plugin in a real browser against a disposable Mattermost server. Microsoft Entra ID and Microsoft Graph cannot run in a container, so the suite talks to a **fake** that implements the parts of both the plugin uses. A nightly **contract check** keeps the fake honest against a real tenant.

Built on [`@mattermost/playwright-lib`](https://www.npmjs.com/package/@mattermost/playwright-lib), so no `mattermost` monorepo checkout is needed. No secrets are required to run the suite, so it works for forks and Cursor Cloud Agents.

## How it works

```
 browser ──login.microsoftonline.com (rewritten by Playwright)──┐
 browser ──teams.microsoft.com (stub page from Playwright)      ▼
 Mattermost + plugin ──https://login.microsoftonline.com ──▶  fake-microsoft
                     ──https://graph.microsoft.com/beta ──▶  (Node, no dependencies)
```

- `fake-microsoft/server.ts` serves OAuth v2 (authorize with login, consent and "stay signed in" pages; the authorization-code, refresh-token and client-credentials grants; AADSTS errors) and the Graph **beta** calls the plugin makes: `GET /me`, `GET /users/{id|upn}` and `POST`/`GET`/`DELETE` `/users/{id}/onlineMeetings`. The plugin uses `msgraph.go/beta`, so the fake serves `/beta`, not `/v1.0`.
- The plugin hardcodes the Microsoft hosts, so Docker Compose gives the fake the network aliases `login.microsoftonline.com` and `graph.microsoft.com`. A throwaway CA, created by `fake-microsoft/gen-certs.sh`, is trusted by Mattermost through `SSL_CERT_FILE`. The browser is redirected to the fake by Playwright routes (`routeMicrosoftToFake` in `support/ui.ts`). Mattermost's `SiteURL` stays `http://localhost:8065`.
- The plugin has no popup. It posts a markdown link to `/oauth2/connect`. `followConnectLink` reads the link, resolves the plugin's redirect itself, and opens the Microsoft authorize URL in a new tab, because Playwright routes never see redirect targets.
- The fake exposes a control API under `/__control/` (reset, config, token expiry and revocation, failing the next meeting creation, requests log, meetings list). `support/fake.ts` wraps it.
- `fake-microsoft/contract.ts` lists the fields each response must contain. The fake validates its own responses and the plugin's requests against it. Violations are listed at `/__control/violations`, and global teardown fails the run if there are any.
- Each test works in its own channel, because the plugin offers to join a meeting created in the same channel in the last 30 seconds.

Test Microsoft users are `alice@contoso.test` (connected as `msteams-e2e-user`), `bob@contoso.test` (connected as `msteams-e2e-user2`) and `carol@contoso.test` (no mail address).

## Running

From the repo root:

```sh
make e2e-env-up     # Postgres, the fake and Mattermost on http://localhost:8065
make e2e            # build the plugin bundle, install deps, run the suite
make e2e-env-down   # stop and wipe the server
```

Or from this directory, after `make dist` and `make e2e-env-up`:

```sh
npm ci && npx playwright install chromium
npm test              # headless
npm run test:headed
npm run test:ui
```

`POST /__control/reset` clears the fake, and global setup calls it.

### Coverage

| Spec | Covers |
| --- | --- |
| `connect` | Connect through the link, repeat connect, disconnect |
| `start_meeting` | Slash command with topic, channel header button, app bar icon, connect-then-post, join button |
| `recent_meeting` | Join existing or create new when a meeting was just created |
| `attendees` | Direct message invites connected members and skips disconnected ones |
| `behavior` | Wrong password, denied consent, account without mail, access-token refresh, revoked refresh token, settings change disconnecting members |
| `errors` | Graph failure from the command and the header button, forged OAuth state, unauthenticated API call |
| `commands` | `help`, bare command, unknown action, extra parameters |

## Contract check against the real service

`npm run contract:compare` runs the same Graph scenarios (app-only token, get users, create online meeting, error shapes, delete) against the fake and a live tenant. It fails when a response is missing a contract field or when the fake and the live service answer with different status codes. The nightly workflow `e2e-contract.yml` runs it.

```sh
npm run fake              # start the fake on http://127.0.0.1:8080
npm run contract:fake     # scenarios against the fake only
npm run contract:live     # scenarios against the live tenant only
npm run contract:compare  # both, and report drift
```

When Microsoft changes a response, the nightly job goes red. Update the fake and `fake-microsoft/contract.ts` together, then rerun the suite.

The contract check uses an app-only token and never signs in as a user, so no passwords or MFA handling are needed. It does not run the browser suite against Microsoft, so real login and consent are not covered.

## Live tenant setup (one-time, by a tenant admin)

Use a **dedicated** tenant, such as a Microsoft 365 Developer Program sandbox, and never the corporate one. None of this is needed for the PR suite.

1. **Users.** Create two test users. Each needs a license that includes **Microsoft Teams** and a mailbox (for example Microsoft 365 Business Basic, E3 or E5). Until Teams provisioning finishes, `onlineMeetings` returns 403 or 404.
2. **App registration.**
   - Single tenant.
   - Add these **Web** redirect URIs. They are used only when someone points a local Mattermost at the real tenant for manual QA, never by the contract check:
     - `http://localhost:8065/plugins/com.mattermost.msteamsmeetings/oauth2/complete`
   - Create one client secret (24 months at most) and note when it expires.
3. **Delegated permissions** (Microsoft Graph, admin consent granted). They cover real plugin use and manual QA:
   - `offline_access`, `User.Read`, `OnlineMeetings.ReadWrite`.
4. **Application permissions** (Microsoft Graph, admin consent granted). They cover the contract checks:
   - `OnlineMeetings.ReadWrite.All`, `User.Read.All`.
5. **Teams application access policy.** App-only `onlineMeetings` calls return 403 without it. This needs the Teams Administrator role:

   ```powershell
   Install-Module MicrosoftTeams; Connect-MicrosoftTeams
   New-CsApplicationAccessPolicy -Identity MM-Plugins-E2E -AppIds "<client-id>" -Description "Mattermost plugin E2E"
   Grant-CsApplicationAccessPolicy -PolicyName MM-Plugins-E2E -Identity "<user1-object-id>"
   Grant-CsApplicationAccessPolicy -PolicyName MM-Plugins-E2E -Identity "<user2-object-id>"
   ```

   The policy can take up to 30 minutes to apply.
6. **GitHub.** Create the environment `e2e-entra` and restrict it to the `master` branch, so forks and PR branches cannot read it. The nightly job stays off until you set the repository variable `MSTEAMS_E2E_CONTRACT_ENABLED` to `true`, so turn it on once everything below is in place. Add these secrets, or put them in an untracked `e2e/playwright/.env` for local runs:

| Variable | Value |
| --- | --- |
| `MSTEAMS_E2E_TENANT_ID` | Directory (tenant) ID |
| `MSTEAMS_E2E_CLIENT_ID` | App client ID |
| `MSTEAMS_E2E_CLIENT_SECRET` | App client secret |
| `MSTEAMS_E2E_USER1_EMAIL` / `MSTEAMS_E2E_USER2_EMAIL` | The two test users' UPNs |

To rotate the secret, create a new one on the app and update `MSTEAMS_E2E_CLIENT_SECRET`.

## Notes

- With the app bar enabled, Mattermost moves the plugin's channel header button into it. The `channel header button` specs turn the app bar off for their duration and turn it back on afterwards.
- Do not run the suite against a Mattermost server you care about. Setup installs the plugin, rewrites its settings and deletes the admin's other teams. Outside CI it refuses to delete any team except playwright-lib's own `ad-1` unless `PW_DELETE_OTHER_TEAMS=true` is set.
- The plugin treats an access token as expired 10 seconds before it expires, so the token specs use a 15 second token and wait about 7 seconds.
- The plugin does not save refreshed tokens, so the fake keeps old refresh tokens valid after rotating them, as Entra does.
- Beta Graph marks `joinUrl` as deprecated and the plugin dereferences it. The contract check's `create online meeting` step is the one most likely to catch a change.
- `make dist` can leave `plugin.json` owner-only inside the bundle. `installPlugin` repacks the bundle with readable permissions before uploading.
