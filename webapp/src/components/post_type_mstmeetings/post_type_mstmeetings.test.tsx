// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {IntlProvider} from 'react-intl';

import {act, fireEvent, render, screen} from '@testing-library/react';

import {Post} from '@mattermost/types/posts';

import PostTypeMSTMeetings from './post_type_mstmeetings';

jest.mock('mattermost-redux/utils/theme_utils', () => ({
    makeStyleFromTheme: (fn: (theme: Record<string, string>) => Record<string, unknown>) => (theme: Record<string, string>) => fn(theme || {}),
}));

const defaultTheme = {buttonColor: '#fff'};

const basePost: Post = {
    id: 'post-id',
    create_at: 0,
    update_at: 0,
    edit_at: 0,
    delete_at: 0,
    is_pinned: false,
    user_id: 'user-id',
    channel_id: 'channel-id',
    root_id: '',
    parent_id: '',
    original_id: '',
    message: '',
    type: 'custom_mstmeetings',
    props: {},
    hashtags: '',
    pending_post_id: '',
};

function renderComponent(props: Partial<React.ComponentProps<typeof PostTypeMSTMeetings>>) {
    const defaultProps: React.ComponentProps<typeof PostTypeMSTMeetings> = {
        post: basePost,
        theme: defaultTheme as React.ComponentProps<typeof PostTypeMSTMeetings>['theme'],
        creatorName: 'Alice',
        currentChannelId: 'channel-123',
        fromBot: false,
        actions: {
            startMeeting: jest.fn().mockResolvedValue(),
        },
    };
    const merged = {...defaultProps, ...props};
    return render(
        <IntlProvider locale='en'>
            <PostTypeMSTMeetings {...merged}/>
        </IntlProvider>,
    );
}

describe('PostTypeMSTMeetings', () => {
    describe('meeting_status text and UI', () => {
        it('shows "I have started a meeting" and JOIN MEETING link', () => {
            const post: Post = {
                ...basePost,
                props: {
                    meeting_status: 'STARTED',
                    meeting_link: 'https://teams.microsoft.com/meet',
                    meeting_topic: 'Daily sync',
                },
            };
            renderComponent({post, fromBot: false});

            expect(screen.getByTestId('mstmeetings-pretext')).toHaveTextContent('I have started a meeting');
            expect(screen.getByTestId('mstmeetings-title')).toHaveTextContent('Daily sync');
            expect(screen.getByTestId('mstmeetings-join-meeting')).toBeInTheDocument();
            expect(screen.getByTestId('mstmeetings-join-meeting')).toHaveAttribute('href', 'https://teams.microsoft.com/meet');
        });

        it('shows creator name in pretext when fromBot is true', () => {
            const post: Post = {
                ...basePost,
                props: {
                    meeting_status: 'STARTED',
                    meeting_link: 'https://teams.microsoft.com/meet',
                },
            };
            renderComponent({post, fromBot: true, creatorName: 'Bob'});

            expect(screen.getByTestId('mstmeetings-pretext')).toHaveTextContent('Bob has started a meeting');
            expect(screen.getByTestId('mstmeetings-join-meeting')).toBeInTheDocument();
        });

        it('shows expected pretext, subtitle, CREATE NEW MEETING and JOIN EXISTING MEETING', () => {
            const post: Post = {
                ...basePost,
                props: {
                    meeting_status: 'RECENTLY_CREATED',
                    meeting_link: 'https://teams.microsoft.com/existing',
                    meeting_topic: 'Standup',
                },
            };
            renderComponent({post, creatorName: 'Charlie'});

            expect(screen.getByTestId('mstmeetings-pretext')).toHaveTextContent('Charlie already created a MS Teams Meeting recently');
            expect(screen.getByTestId('mstmeetings-subtitle')).toHaveTextContent('Would you like to join, or create your own meeting?');
            expect(screen.getByTestId('mstmeetings-title')).toHaveTextContent('Standup');
            expect(screen.getByTestId('mstmeetings-create-new-meeting')).toBeInTheDocument();
            expect(screen.getByTestId('mstmeetings-create-new-meeting')).toHaveTextContent('CREATE NEW MEETING');
            expect(screen.getByTestId('mstmeetings-join-existing-meeting')).toBeInTheDocument();
            expect(screen.getByTestId('mstmeetings-join-existing-meeting')).toHaveAttribute('href', 'https://teams.microsoft.com/existing');
        });

        it('shows default title only, no join/create controls', () => {
            const post: Post = {...basePost, props: {}};
            renderComponent({post});

            expect(screen.getByTestId('mstmeetings-pretext')).toHaveTextContent('');
            expect(screen.getByTestId('mstmeetings-title')).toHaveTextContent('MS Teams Meeting');
            expect(screen.queryByTestId('mstmeetings-join-meeting')).not.toBeInTheDocument();
            expect(screen.queryByTestId('mstmeetings-create-new-meeting')).not.toBeInTheDocument();
            expect(screen.queryByTestId('mstmeetings-subtitle')).not.toBeInTheDocument();
        });

        it('uses meeting_topic as title when it is set', () => {
            const post: Post = {
                ...basePost,
                props: {meeting_status: 'STARTED', meeting_link: 'https://link', meeting_topic: 'Sprint planning'},
            };
            renderComponent({post});

            expect(screen.getByTestId('mstmeetings-title')).toHaveTextContent('Sprint planning');
        });
    });

    describe('handleForceStart', () => {
        it('calls startMeeting with currentChannelId, true (force), and meeting_topic when CREATE NEW MEETING is clicked', async () => {
            const startMeeting = jest.fn().mockResolvedValue();
            const post: Post = {
                ...basePost,
                props: {
                    meeting_status: 'RECENTLY_CREATED',
                    meeting_link: 'https://existing',
                    meeting_topic: 'My topic',
                },
            };
            renderComponent({
                post,
                currentChannelId: 'channel-456',
                actions: {startMeeting},
            });

            const createButton = screen.getByTestId('mstmeetings-create-new-meeting');
            await act(async () => {
                fireEvent.click(createButton);
            });

            expect(startMeeting).toHaveBeenCalledTimes(1);
            expect(startMeeting).toHaveBeenCalledWith('channel-456', true, 'My topic');
        });

        it('does not call startMeeting twice when clicked rapidly', async () => {
            let resolveMeeting: () => void;
            const startMeeting = jest.fn().mockImplementation(() => new Promise<void>((resolve) => {
                resolveMeeting = resolve;
            }));
            const post: Post = {
                ...basePost,
                props: {
                    meeting_status: 'RECENTLY_CREATED',
                    meeting_link: 'https://existing',
                    meeting_topic: 'Topic',
                },
            };
            renderComponent({post, actions: {startMeeting}});

            const createButton = screen.getByTestId('mstmeetings-create-new-meeting');
            fireEvent.click(createButton);
            fireEvent.click(createButton);

            expect(startMeeting).toHaveBeenCalledTimes(1);
            await act(async () => {
                (resolveMeeting! as () => void)();
            });
        });
    });
});
