// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {IntlProvider} from 'react-intl';

import {render, screen} from '@testing-library/react';

import Icon from './icon';

const wrapWithIntl = (component: React.ReactNode) => {
    return (
        <IntlProvider locale='en'>
            {component}
        </IntlProvider>
    );
};

describe('Icon', () => {
    it('renders with accessible label', () => {
        render(wrapWithIntl(<Icon/>));
        expect(screen.getByLabelText('camera icon')).toBeInTheDocument();
    });

    it('renders with icon class', () => {
        render(wrapWithIntl(<Icon/>));
        expect(document.querySelector('.icon')).toBeInTheDocument();
    });
});
