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
	addConsoleLog,
} from '../stores/chat.svelte';
import { EventsOn } from '../../../wailsjs/runtime/runtime';
import type { config } from '../../../wailsjs/go/models';
import { parseStreamingArtifacts } from '../parsers/artifact';
import type { ArtifactFile } from '../parsers/artifact';

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

async function loadArtifactFilesFromDisk(sessionID: string) {
	const { GetArtifactFileContents } = await import('../../../wailsjs/go/main/ChatService');
	const fileInfos = await GetArtifactFileContents(sessionID) as Array<{ path: string; language: string; content: string }>;
	if (fileInfos && fileInfos.length > 0) {
		const files: ArtifactFile[] = fileInfos.map(f => ({
			path: f.path,
			language: f.language,
			content: f.content,
		}));
		setArtifactFiles(files);
		setSelectedFilePath(files[0].path);
	} else {
		setArtifactFiles([]);
		setSelectedFilePath(null);
	}
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
			await loadArtifactFilesFromDisk(id);
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

export async function cancelSend() {
	const { CancelSend } = await import('../../../wailsjs/go/main/ChatService');
	await CancelSend();
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
		}
	}, 400);
}

function initGlobalConsoleCapture() {
	const originalConsole = {
		log: console.log.bind(console),
		error: console.error.bind(console),
		warn: console.warn.bind(console),
		info: console.info.bind(console),
	};

	function captureAppConsole(type: 'log' | 'error' | 'warn' | 'info', args: unknown[]) {
		const message = args
			.map(arg => {
				if (typeof arg === 'object') {
					try {
						return JSON.stringify(arg, null, 2);
					} catch {
						return String(arg);
					}
				}
				return String(arg);
			})
			.join(' ');
		addConsoleLog({
			type,
			message,
			timestamp: new Date().toLocaleTimeString(),
			source: 'app',
		});
	}

	console.log = (...args: unknown[]) => {
		originalConsole.log(...args);
		captureAppConsole('log', args);
	};
	console.error = (...args: unknown[]) => {
		originalConsole.error(...args);
		captureAppConsole('error', args);
	};
	console.warn = (...args: unknown[]) => {
		originalConsole.warn(...args);
		captureAppConsole('warn', args);
	};
	console.info = (...args: unknown[]) => {
		originalConsole.info(...args);
		captureAppConsole('info', args);
	};

	window.addEventListener('message', (event: MessageEvent) => {
		if (event.data && event.data.type === 'iframe-console') {
			const message = (event.data.args as string[]).join(' ');
			addConsoleLog({
				type: event.data.level as 'log' | 'error' | 'warn' | 'info',
				message,
				timestamp: new Date(event.data.timestamp || Date.now()).toLocaleTimeString(),
				source: 'iframe',
			});
		}
	});
}

export function registerLLMListener() {
	if (llmListenerRegistered) return;
	llmListenerRegistered = true;

	initGlobalConsoleCapture();

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
		}
	});

	EventsOn('artifact-update', async (data: Record<string, string>) => {
		try {
			const sessionId = data.session_id;
			if (data.preview_url) {
				setPreviewUrl(data.preview_url);
			}
			if (sessionId) {
				await loadArtifactFilesFromDisk(sessionId);
			}
		} catch {
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
		}
	});
}
