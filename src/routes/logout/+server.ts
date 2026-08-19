import { redirect } from '@sveltejs/kit';
import { AUTH_COOKIE } from '$lib/server/auth';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ cookies }) => {
	cookies.delete(AUTH_COOKIE, { path: '/' });
	redirect(303, '/login');
};
