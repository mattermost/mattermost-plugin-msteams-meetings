// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ChannelsPage} from '@mattermost/playwright-lib';
import type {Locator} from '@playwright/test';

// The newest custom_mstmeetings post in the center channel, built on the plugin's data-testid attributes.
export class MeetingPost {
    readonly pretext: Locator;
    readonly title: Locator;
    readonly subtitle: Locator;
    readonly joinButton: Locator;
    readonly createNewButton: Locator;
    readonly joinExistingButton: Locator;

    constructor(channelsPage: ChannelsPage) {
        const container = channelsPage.centerView.container;
        this.pretext = container.getByTestId('mstmeetings-pretext').last();
        this.title = container.getByTestId('mstmeetings-title').last();
        this.subtitle = container.getByTestId('mstmeetings-subtitle').last();
        this.joinButton = container.getByTestId('mstmeetings-join-meeting').last();
        this.createNewButton = container.getByTestId('mstmeetings-create-new-meeting').last();
        this.joinExistingButton = container.getByTestId('mstmeetings-join-existing-meeting').last();
    }
}
