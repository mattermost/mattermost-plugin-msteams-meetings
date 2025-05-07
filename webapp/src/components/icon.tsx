// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

export default function Icon() {
    const {formatMessage} = useIntl();
    const ariaLabel = formatMessage({
        id: 'msteamsmeetings.camera.ariaLabel',
        defaultMessage: 'camera icon',
    });

    return (
        <span
            aria-label={ariaLabel}
            className='icon'
            style={{verticalAlign: 'middle'}}
        >
            <svg
                width='19'
                height='19'
                viewBox='0 0 20 20'
                fill='inherit'
                xmlns='http://www.w3.org/2000/svg'
            >
                <path
                    fillRule='evenodd'
                    clipRule='evenodd'
                    d='M1.6 0C0.716344 0 0 0.716345 0 1.6V18.4C0 19.2837 0.716345 20 1.6 20H18.4C19.2837 20 20 19.2837 20 18.4V1.6C20 0.716344 19.2837 0 18.4 0H1.6ZM3.6 2C2.71634 2 2 2.71634 2 3.6V16.4C2 17.2837 2.71634 18 3.6 18H16.4C17.2837 18 18 17.2837 18 16.4V3.6C18 2.71634 17.2837 2 16.4 2H3.6Z'
                    fill='inherit'
                />
                <path
                    fillRule='evenodd'
                    clipRule='evenodd'
                    d='M9 5H6V7H9V15H11V7H14V5H11H9Z'
                    fill='inherit'
                />
            </svg>
        </span>
    );
}
