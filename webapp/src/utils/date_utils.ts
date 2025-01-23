// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

export function formatDate(date: Date, useMilitaryTime = false) {
    const monthNames = [
        'Jan', 'Feb', 'Mar',
        'Apr', 'May', 'Jun', 'Jul',
        'Aug', 'Sep', 'Oct',
        'Nov', 'Dec',
    ];

    const day = date.getDate();
    const monthIndex = date.getMonth();
    let hours = date.getHours();
    const minutes = date.getMinutes();

    let ampm = '';
    if (!useMilitaryTime) {
        ampm = ' AM';
        if (hours >= 12) {
            ampm = ' PM';
        }

        hours %= 12;
        if (!hours) {
            hours = 12;
        }
    }

    let stringMinutes = String(minutes);
    if (minutes < 10) {
        stringMinutes = '0' + minutes;
    }

    return monthNames[monthIndex] + ' ' + day + ' at ' + hours + ':' + stringMinutes + ampm;
}
