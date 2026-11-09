import fs from 'node:fs';
import path from 'node:path';

// Loads an untracked .env for local runs. Variables already in the environment win.
const envFile = path.resolve(__dirname, '../.env');
if (fs.existsSync(envFile)) {
    process.loadEnvFile(envFile);
}

// Defaults used by @mattermost/playwright-lib, which reads process.env while its
// module is evaluated. Import this before that package. localhost:8065 is the
// only redirect URI registered on the Entra test app.
process.env.PW_BASE_URL = process.env.PW_BASE_URL || 'http://localhost:8065';
process.env.PW_HEADLESS = process.env.PW_HEADLESS || 'true';
