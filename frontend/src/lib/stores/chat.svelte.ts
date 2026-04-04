import type { types } from '../../../wailsjs/go/models';

export type ChatMessage = types.Message;
export type ChatSession = types.Session;

let sessions = $state<ChatSession[]>([]);
let currentSessionId = $state<string | null>(null);
let streamingContent = $state<string>('');
let isStreaming = $state<boolean>(false);

export function getSessions() {
	return sessions;
}

export function getCurrentSessionId() {
	return currentSessionId;
}

export function getCurrentSession(): ChatSession | undefined {
	return sessions.find(s => s.id === currentSessionId);
}

export function getStreamingContent() {
	return streamingContent;
}

export function getIsStreaming() {
	return isStreaming;
}

export function setSessions(s: ChatSession[]) {
	sessions = s;
}

export function setCurrentSessionId(id: string | null) {
	currentSessionId = id;
	streamingContent = '';
}

export function setStreamingContent(content: string) {
	streamingContent = content;
}

export function appendStreamingContent(chunk: string) {
	streamingContent += chunk;
}

export function setIsStreaming(value: boolean) {
	isStreaming = value;
}

export function updateSession(id: string, updated: ChatSession) {
	const idx = sessions.findIndex(s => s.id === id);
	if (idx >= 0) {
		sessions[idx] = updated;
	}
}

export function addSession(s: ChatSession) {
	sessions = [s, ...sessions];
}

export function removeSession(id: string) {
	sessions = sessions.filter(s => s.id !== id);
}
