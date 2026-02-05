// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {makeStyleFromTheme} from 'mattermost-redux/utils/theme_utils';
import {ActionResult} from 'mattermost-redux/types/actions';
import {Post} from '@mattermost/types/posts';
import {Theme} from 'mattermost-redux/selectors/entities/preferences';

import Icon from 'components/icon';

type Props = {
    post: Post;
    compactDisplay?: boolean;
    isRHS?: boolean;
    useMilitaryTime?: boolean;
    theme: Theme;
    creatorName: string;
    currentChannelId: string;
    fromBot: boolean;
    actions: {
        startMeeting: (channelID: string, force: boolean, topic: string) => ActionResult;
    };
}

export default function PostTypeMSTMeetings(props: Props) {
    const style = getStyle(props.theme);
    const post = props.post;
    const postProps = post.props || {};

    const [creatingMeeting, setCreatingMeeting] = React.useState(false);

    const handleForceStart = async () => {
        if (!creatingMeeting) {
            setCreatingMeeting(true);
            await props.actions.startMeeting(props.currentChannelId, true, postProps?.meeting_topic as string);
            setCreatingMeeting(false);
        }
    };

    let preText = '';
    let content: JSX.Element | undefined;
    let subtitle = '';
    if (postProps.meeting_status === 'STARTED') {
        preText = 'I have started a meeting';
        if (props.fromBot) {
            preText = `${props.creatorName} has started a meeting`;
        }
        content = (
            <a
                className='btn btn-lg btn-primary'
                style={style.button}
                rel='noopener noreferrer'
                target='_blank'
                href={postProps?.meeting_link as string}
                data-testid='mstmeetings-join-meeting'
            >
                <i style={style.buttonIcon}>
                    <Icon/>
                </i>
                {'JOIN MEETING'}
            </a>
        );
    } else if (postProps.meeting_status === 'RECENTLY_CREATED') {
        preText = `${props.creatorName} already created a MS Teams Meeting recently`;

        subtitle = 'Would you like to join, or create your own meeting?';
        content = (
            <div>
                <div>
                    <a
                        className='btn btn-lg btn-primary'
                        style={style.button}
                        rel='noopener noreferrer'
                        onClick={handleForceStart}
                        data-testid='mstmeetings-create-new-meeting'
                    >
                        {'CREATE NEW MEETING'}
                    </a>
                </div>
                <div>
                    <a
                        className='btn btn-lg btn-primary'
                        style={style.button}
                        rel='noopener noreferrer'
                        target='_blank'
                        href={postProps?.meeting_link as string}
                        data-testid='mstmeetings-join-existing-meeting'
                    >
                        <i style={style.buttonIcon}>
                            <Icon/>
                        </i>
                        {'JOIN EXISTING MEETING'}
                    </a>
                </div>
            </div>
        );
    }

    let title = 'MS Teams Meeting';
    if (postProps?.meeting_topic) {
        title = postProps.meeting_topic as string;
    }

    return (
        <div className='attachment attachment--pretext'>
            <div
                className='attachment__thumb-pretext'
                data-testid='mstmeetings-pretext'
            >
                {preText}
            </div>
            <div className='attachment__content'>
                <div className='clearfix attachment__container'>
                    <h5
                        className='mt-1'
                        style={style.title}
                        data-testid='mstmeetings-title'
                    >
                        {title}
                    </h5>
                    {subtitle ? (
                        <div data-testid='mstmeetings-subtitle'>
                            {subtitle}
                        </div>
                    ) : null}
                    <div>
                        <div style={style.body}>
                            {content}
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}

PostTypeMSTMeetings.defaultProps = {
    compactDisplay: false,
    isRHS: false,
};

const getStyle = makeStyleFromTheme((theme) => {
    return {
        body: {
            overflowX: 'auto',
            overflowY: 'hidden',
            paddingRight: '5px',
            width: '100%',
        },
        title: {
            fontWeight: '600',
        },
        button: {
            fontFamily: 'Open Sans',
            fontSize: '12px',
            fontWeight: 'bold',
            letterSpacing: '1px',
            lineHeight: '19px',
            marginTop: '12px',
            borderRadius: '4px',
            color: theme.buttonColor,
        },
        buttonIcon: {
            paddingRight: '8px',
            fill: theme.buttonColor,
        },
        summary: {
            fontFamily: 'Open Sans',
            fontSize: '14px',
            fontWeight: '600',
            lineHeight: '26px',
            margin: '0',
            padding: '14px 0 0 0',
        },
        summaryItem: {
            fontFamily: 'Open Sans',
            fontSize: '14px',
            lineHeight: '26px',
        },
    };
});
