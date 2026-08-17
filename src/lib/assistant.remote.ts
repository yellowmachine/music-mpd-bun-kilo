import { command } from '$app/server';
import { runAssistant } from '$lib/server/assistant';

export const sendMessage = command('unchecked', async (text: string) => {
	const { text: reply } = await runAssistant(text);
	return { reply };
});
