import {
	getCurrentSessionId,
	setSessions,
	setCurrentSessionId,
	setStreamingContent,
	appendStreamingContent,
	setIsStreaming,
	updateSession,
	addSession,
	removeSession,
} from '../stores/chat.svelte';
import { EventsOn } from '../../../wailsjs/runtime/runtime';
import type { config } from '../../../wailsjs/go/models';

export async function loadConfig(): Promise<config.Config> {
	const { GetConfig } = await import('../../../wailsjs/go/main/ChatService');
	return await GetConfig();
}

export async function loadSessions() {
	const { ListSessions } = await import('../../../wailsjs/go/main/ChatService');
	const list = await ListSessions();
	setSessions(list || []);
}

export async function createNewSession() {
	const { CreateSession } = await import('../../../wailsjs/go/main/ChatService');
	const id = await CreateSession('New Chat');
	const { GetSession } = await import('../../../wailsjs/go/main/ChatService');
	const session = await GetSession(id);
	if (session) {
		addSession(session);
	}
	setCurrentSessionId(id);
	return id;
}

export async function switchSession(id: string) {
	setCurrentSessionId(id);
	const { GetSession } = await import('../../../wailsjs/go/main/ChatService');
	const session = await GetSession(id);
	if (session) {
		updateSession(id, session);
	}
}

export async function deleteSession(id: string) {
	const { DeleteSession } = await import('../../../wailsjs/go/main/ChatService');
	await DeleteSession(id);
	removeSession(id);
	const currentId = getCurrentSessionId();
	if (currentId === id) {
		setCurrentSessionId(null);
	}
}

export async function sendMessage(message: string) {
	const sessionId = getCurrentSessionId();
	if (!sessionId || !message.trim()) return;

	setStreamingContent('');
	setIsStreaming(true);

	const { SendMessage } = await import('../../../wailsjs/go/main/ChatService');
	try {
		await SendMessage(sessionId, message);
	} finally {
		setIsStreaming(false);
	}

	const { GetSession } = await import('../../../wailsjs/go/main/ChatService');
	const session = await GetSession(sessionId);
	if (session) {
		updateSession(sessionId, session);
	}
}

let llmListenerRegistered = false;

export function registerLLMListener() {
	if (llmListenerRegistered) return;
	llmListenerRegistered = true;

	EventsOn('llm-event', (data: Record<string, string>) => {
		if (data.type === 'chunk') {
			appendStreamingContent(data.content);
		} else if (data.type === 'done') {
			setStreamingContent('');
		} else if (data.type === 'error') {
			setStreamingContent('[Error] ' + data.content);
			setIsStreaming(false);
		}
	});
}
