import { env } from '$env/dynamic/private';

const REPO = 'yellowmachine/music-mpd-bun-kilo';
const BRANCH = 'main';
const CHECK_INTERVAL_MS = 6 * 60 * 60 * 1000; // 6h — GitHub's public API rate limit is 60 req/hr unauthenticated

export interface UpdateStatus {
	current: string | null;
	latest: string | null;
	updateAvailable: boolean;
	checkedAt: number | null;
	error: string | null;
}

let status: UpdateStatus = {
	current: env.GIT_SHA && env.GIT_SHA !== 'unknown' ? env.GIT_SHA : null,
	latest: null,
	updateAvailable: false,
	checkedAt: null,
	error: null
};

async function checkForUpdate() {
	if (!status.current) return; // image wasn't built with GIT_SHA baked in — nothing to compare against

	try {
		const res = await fetch(`https://api.github.com/repos/${REPO}/commits/${BRANCH}`, {
			headers: { Accept: 'application/vnd.github+json' }
		});
		if (!res.ok) throw new Error(`GitHub API responded ${res.status}`);
		const json = (await res.json()) as { sha?: string };
		if (!json.sha) throw new Error('GitHub API response missing sha');

		status = {
			...status,
			latest: json.sha,
			updateAvailable: json.sha !== status.current,
			checkedAt: Date.now(),
			error: null
		};
	} catch (e) {
		status = {
			...status,
			checkedAt: Date.now(),
			error: e instanceof Error ? e.message : 'unknown error'
		};
	}
}

const MIN_RECHECK_INTERVAL_MS = 60 * 1000; // avoid hammering GitHub if /admin is loaded repeatedly in quick succession

export async function getUpdateStatus(): Promise<UpdateStatus> {
	const stale = !status.checkedAt || Date.now() - status.checkedAt > MIN_RECHECK_INTERVAL_MS;
	if (stale) await checkForUpdate();
	return status;
}

export function startPeriodicCheck() {
	checkForUpdate();
	setInterval(checkForUpdate, CHECK_INTERVAL_MS);
}
