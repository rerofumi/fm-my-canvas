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
	setArtifactFiles,
	setPreviewUrl,
	setSelectedFilePath,
	getStreamingContent,
	clearArtifactData,
	addToolCallEntry,
	updateToolCallResult,
	getToolCallLog,
} from '../stores/chat.svelte';
import { EventsOn } from '../../../wailsjs/runtime/runtime';
import type { config } from '../../../wailsjs/go/models';
import { parseStreamingArtifacts, parseArtifacts } from '../parsers/artifact';

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

		const { RestoreArtifacts } = await import('../../../wailsjs/go/main/ChatService');
		const result = await RestoreArtifacts(id);

		if (result.preview_url) {
			setPreviewUrl(result.preview_url);
		} else {
			setPreviewUrl('');
		}

		if (result.files) {
			const paths = result.files.split(',').filter(Boolean);
			if (paths.length > 0) {
				setSelectedFilePath(paths[0]);

				let lastArtifactContent = '';
				for (let i = session.messages.length - 1; i >= 0; i--) {
					const msg = session.messages[i];
					if (msg.role === 'assistant') {
						const files = parseArtifacts(msg.content);
						if (files.length > 0) {
							lastArtifactContent = msg.content;
							break;
						}
					}
				}

				if (lastArtifactContent) {
					setArtifactFiles(parseArtifacts(lastArtifactContent));
				} else {
					setArtifactFiles(paths.map(p => ({ language: '', path: p, content: '' })));
				}
			}
		} else {
			setArtifactFiles([]);
			setPreviewUrl('');
			setSelectedFilePath(null);
		}
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

	clearArtifactData();
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
let artifactThrottleTimer: ReturnType<typeof setTimeout> | null = null;

function scheduleArtifactUpdate() {
	if (artifactThrottleTimer) return;
	artifactThrottleTimer = setTimeout(() => {
		artifactThrottleTimer = null;
		try {
			const content = getStreamingContent();
			if (content) {
				const { files } = parseStreamingArtifacts(content);
				setArtifactFiles(files);
			}
		} catch {
			// ignore parse errors during streaming
		}
	}, 400);
}

export function registerLLMListener() {
	if (llmListenerRegistered) return;
	llmListenerRegistered = true;

	EventsOn('llm-event', (data: Record<string, string>) => {
		try {
			if (data.type === 'chunk') {
				appendStreamingContent(data.content);
				scheduleArtifactUpdate();
			} else if (data.type === 'done') {
				scheduleArtifactUpdate();
				setStreamingContent('');
			} else if (data.type === 'error') {
				setStreamingContent('[Error] ' + data.content);
				setIsStreaming(false);
			}
		} catch {
			// ignore event processing errors
		}
	});

	EventsOn('artifact-update', (data: Record<string, string>) => {
		try {
			if (data.preview_url) {
				setPreviewUrl(data.preview_url);
			}
			if (data.files) {
				const paths = data.files.split(',').filter(Boolean);
				if (paths.length > 0) {
					setSelectedFilePath(paths[0]);
				}
			}
		} catch {
			// ignore
		}
	});

	EventsOn('tool-event', (data: Record<string, any>) => {
		try {
			if (data.type === 'tool_call') {
				addToolCallEntry({
					toolName: data.tool_name,
					toolArgs: data.tool_args,
					status: 'running',
					timestamp: Date.now(),
				});
			} else if (data.type === 'tool_result') {
				const log = getToolCallLog();
				const idx = log.reduce((acc, entry, i) => {
					if (entry.status === 'running') return i;
					return acc;
				}, -1);
				if (idx >= 0) {
					updateToolCallResult(idx, data.result, data.success ? 'success' : 'error');
				}
			}
		} catch {
			// ignore
		}
	});
}
