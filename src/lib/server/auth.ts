import { createHash, timingSafeEqual } from 'node:crypto';

export const AUTH_COOKIE = 'auth_token';

// Derived from the admin password rather than the password itself, so the
// raw password never sits in a cookie even though the cookie is httpOnly.
export function sessionToken(password: string): string {
	return createHash('sha256').update(password).digest('hex');
}

export function passwordMatches(input: string, expected: string): boolean {
	const a = Buffer.from(input);
	const b = Buffer.from(expected);
	if (a.length !== b.length) return false;
	return timingSafeEqual(a, b);
}
