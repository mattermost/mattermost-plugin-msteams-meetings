// Mock Node.js built-in modules that may cause issues in Jest
global.ReadableStream = global.ReadableStream || class ReadableStream {};
global.WritableStream = global.WritableStream || class WritableStream {};
global.TransformStream = global.TransformStream || class TransformStream {};

// Mock stream module
jest.mock('stream', () => require('stream-browserify'));