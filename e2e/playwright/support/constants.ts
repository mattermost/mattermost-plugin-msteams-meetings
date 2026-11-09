// Copyright (c) 2019-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

export const PLUGIN_ID = 'com.mattermost.msteamsmeetings';

export const TEAM_NAME = 'msteams-e2e';
export const TOWN_SQUARE = 'town-square';
export const COMMAND = '/mstmeetings';

export const BOT_USERNAME = 'mstmeetings';

// The plugin id contains dots, so an #id selector would parse them as classes.
export const APP_BAR_ICON = `[id="app-bar-icon-${PLUGIN_ID}"]`;

export const JOIN_URL_PREFIX = 'https://teams.microsoft.com/l/meetup-join/';

// The tooltip text the plugin registers for its channel header and app bar buttons.
export const START_HELP_TEXT = 'Start MS Teams Meeting';
