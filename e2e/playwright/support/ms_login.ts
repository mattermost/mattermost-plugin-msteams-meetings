// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Locator, Page} from '@playwright/test';

import type {TestUser} from './credentials';

const FLOW_TIMEOUT_MS = 45_000;

// The Microsoft sign-in page: email, password, then the optional "Stay signed in?" and consent screens.
export class MicrosoftLoginPage {
    readonly page: Page;
    readonly emailInput: Locator;
    readonly passwordInput: Locator;
    readonly nextButton: Locator;
    readonly signInButton: Locator;
    readonly error: Locator;
    readonly consentHeading: Locator;
    readonly acceptConsentButton: Locator;
    readonly cancelConsentButton: Locator;
    readonly staySignedInHeading: Locator;
    readonly staySignedInNoButton: Locator;
    readonly completedMessage: Locator;

    constructor(page: Page) {
        this.page = page;
        this.emailInput = page.getByPlaceholder('Email, phone, or Skype');
        this.passwordInput = page.getByPlaceholder('Password');
        this.nextButton = page.getByRole('button', {name: 'Next'});
        this.signInButton = page.getByRole('button', {name: 'Sign in'});
        this.error = page.getByRole('alert');
        this.consentHeading = page.getByRole('heading', {name: 'Permissions requested'});
        this.acceptConsentButton = page.getByRole('button', {name: 'Accept'});
        this.cancelConsentButton = page.getByRole('button', {name: 'Cancel'});
        this.staySignedInHeading = page.getByRole('heading', {name: 'Stay signed in?'});
        this.staySignedInNoButton = page.getByRole('button', {name: 'No', exact: true});
        this.completedMessage = page.getByText('Completed connecting to Microsoft');
    }

    async submitCredentials(user: TestUser) {
        await this.emailInput.waitFor({timeout: 15_000}).catch(async () => {
            throw new Error(`Microsoft sign-in page did not load. Last page: ${await this.describe()}`);
        });
        await this.emailInput.fill(user.email);
        await this.nextButton.click();

        await this.passwordInput.fill(user.password);
        await this.signInButton.click();
    }

    async signIn(user: TestUser) {
        await this.submitCredentials(user);
        await this.finishSignIn();
    }

    // Answers the optional screens until the plugin reports the connection is complete.
    // The tab is opened by the test rather than by script, so the plugin's window.close() may not close it.
    async finishSignIn() {
        const deadline = Date.now() + FLOW_TIMEOUT_MS;
        while (!this.page.isClosed() && Date.now() < deadline) {
            // eslint-disable-next-line no-await-in-loop
            if (await this.completedMessage.isVisible().catch(() => false)) {
                return;
            }

            // eslint-disable-next-line no-await-in-loop
            await this.handleOptionalScreen();
            // eslint-disable-next-line no-await-in-loop
            await this.page.waitForTimeout(500).catch(() => undefined);
        }

        if (!this.page.isClosed()) {
            throw new Error(`Microsoft sign-in did not finish. Last page: ${await this.describe()}`);
        }
    }

    private async handleOptionalScreen() {
        try {
            if (await this.staySignedInHeading.isVisible()) {
                // Declining keeps sessions out of later specs.
                await this.staySignedInNoButton.click();
                return;
            }

            if (await this.consentHeading.isVisible()) {
                await this.acceptConsentButton.click();
                return;
            }

            if (await this.error.first().isVisible()) {
                throw new Error(`Microsoft sign-in failed: ${await this.error.first().innerText()}`);
            }
        } catch (err) {
            // The popup closing mid-check is the success path.
            if (!this.page.isClosed()) {
                throw err;
            }
        }
    }

    private async describe() {
        const text = await this.page.locator('body').innerText().catch(() => '');
        return `${this.page.url()} ${text.replace(/\s+/g, ' ').slice(0, 300)}`;
    }
}
