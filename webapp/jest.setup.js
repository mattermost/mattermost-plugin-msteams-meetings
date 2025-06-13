// Copyright (c) 2020-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Mock Node.js built-in modules that may cause issues in Jest
global.ReadableStream = global.ReadableStream || class ReadableStream {};
global.WritableStream = global.WritableStream || class WritableStream {};
global.TransformStream = global.TransformStream || class TransformStream {};

// Mock stream module
/* eslint-disable global-require */
jest.mock('stream', () => require('stream-browserify'));
/* eslint-enable global-require */