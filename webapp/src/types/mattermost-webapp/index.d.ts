// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {Channel, ChannelMembership} from 'mattermost-redux/types/channels';

export interface PluginRegistry {
    registerChannelHeaderButtonAction(icon: React.ReactNode, callback: (channel: Channel) => void, text: string)
    registerPostTypeComponent(typeName: string, component: React.ElementType)
    registerAppBarComponent(iconUrl: string, action: (channel: Channel, channelMember: ChannelMembership) => void, tooltipText: React.ReactNode)
}
