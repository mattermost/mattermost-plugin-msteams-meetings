// Hand-written response contracts for the Microsoft Graph (beta) and Entra ID calls the
// plugin makes. Graph's published OpenAPI metadata marks nearly everything optional,
// so these list the fields the plugin actually reads or sends. The fake checks each of
// its own responses and the plugin's requests, and the nightly job checks the live
// service against the same lists.

export type Contract = {name: string; required: string[]};

export const CONTRACTS: Record<string, Contract> = {
    token: {name: 'token response', required: ['token_type', 'access_token', 'expires_in', 'scope']},
    appToken: {name: 'app-only token response', required: ['token_type', 'access_token', 'expires_in']},
    user: {name: 'user', required: ['id', 'displayName', 'userPrincipalName', 'mail']},
    onlineMeeting: {
        name: 'online meeting',
        required: ['id', 'joinUrl', 'subject', 'startDateTime', 'endDateTime', 'participants.organizer.identity.user.id'],
    },
    createOnlineMeetingRequest: {
        name: 'create online meeting request',
        required: ['subject', 'startDateTime', 'endDateTime', 'participants.organizer.identity.user.id', 'participants.organizer.upn'],
    },
    graphError: {name: 'Graph error', required: ['error.code', 'error.message']},
};

function lookup(value: unknown, path: string): unknown {
    let current: any = value;
    for (const part of path.split('.')) {
        if (current === null || typeof current !== 'object' || !(part in current)) {
            return undefined;
        }
        current = current[part];
    }
    return current;
}

// Returns the required fields that are missing or null.
export function missingFields(contractKey: string, value: unknown): string[] {
    const contract = CONTRACTS[contractKey];
    return contract.required.filter((path) => {
        const found = lookup(value, path);
        return found === undefined || found === null;
    });
}
