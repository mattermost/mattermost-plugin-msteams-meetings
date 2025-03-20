// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {Store, Action} from 'redux';

import {Channel} from 'mattermost-redux/types/channels';
import {GlobalState} from 'mattermost-redux/types/store';
import {getConfig} from 'mattermost-redux/selectors/entities/general';

import {id as pluginId} from './manifest';
import Icon from './components/icon';
import PostTypeMSTMeetings from './components/post_type_mstmeetings';
import {startMeeting} from './actions';
import Client from './client';
// eslint-disable-next-line import/no-unresolved
import {PluginRegistry} from './types/mattermost-webapp';

class Plugin {
    public async initialize(registry: PluginRegistry, store: Store<GlobalState, Action<Record<string, unknown>>>) {
        const helpText = 'Start MS Teams Meeting';

        let creatingMeeting = false;
        const action = async (channel: Channel) => {
            if (!creatingMeeting) {
                creatingMeeting = true;
                await startMeeting(channel.id, false, '')(store.dispatch, store.getState);
                creatingMeeting = false;
            }
        };

        // Channel header icon
        registry.registerChannelHeaderButtonAction(<Icon/>, action, helpText);

        // App Bar icon
        if (registry.registerAppBarComponent) {
            const siteUrl = getConfig(store.getState())?.SiteURL || '';
            const iconURL = `${siteUrl}/plugins/${pluginId}/public/app-bar-icon.png`;
            registry.registerAppBarComponent(iconURL, action, helpText);
        }

        registry.registerPostTypeComponent('custom_mstmeetings', PostTypeMSTMeetings);
        Client.setServerRoute(getServerRoute(store.getState()));
    }
}

declare global {
    interface Window {
        registerPlugin(id: string, plugin: Plugin): void
    }
}

window.registerPlugin(pluginId, new Plugin());

const getServerRoute = (state: GlobalState) => {
    const config = getConfig(state);

    let basePath = '';
    if (config && config.SiteURL) {
        basePath = new URL(config.SiteURL).pathname;

        if (basePath && basePath[basePath.length - 1] === '/') {
            basePath = basePath.substr(0, basePath.length - 1);
        }
    }

    return basePath;
};
