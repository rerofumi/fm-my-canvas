<script lang="ts">
	import {
		getCurrentSession,
		getStreamingContent,
		getIsStreaming,
		getCurrentSessionId,
		getToolCallLog,
	} from '../../lib/stores/chat.svelte';
	import { sendMessage, createNewSession } from '../../lib/services/wails';
	import ChatMessage from './ChatMessage.svelte';
	import ChatInput from './ChatInput.svelte';
	import ToolCallMessage from './ToolCallMessage.svelte';

	let currentSession = $derived(getCurrentSession());
	let streamingContent = $derived(getStreamingContent());
	let isStreaming = $derived(getIsStreaming());
	let currentSessionId = $derived(getCurrentSessionId());
	let toolCallLog = $derived(getToolCallLog());

	let chatContainer: HTMLDivElement | undefined = $state();

	$effect(() => {
		if (chatContainer) {
			currentSession;
			streamingContent;
			chatContainer.scrollTop = chatContainer.scrollHeight;
		}
	});

	async function handleSend(text: string) {
		if (!currentSessionId) {
			await createNewSession();
		}
		sendMessage(text);
	}
</script>

<div class="main-area">
	{#if currentSession}
		<div class="chat-container" bind:this={chatContainer}>
			{#each currentSession.messages as message (message.created_at)}
				{#if message.role !== 'system' && message.role !== 'tool'}
					<ChatMessage {message} defaultCollapsed={true} />
				{/if}
			{/each}
			{#if toolCallLog.length > 0}
				<ToolCallMessage />
			{/if}
			{#if streamingContent}
				<div class="message streaming">
					<div class="message-role">Assistant</div>
					<div class="message-content">
						<pre>{streamingContent}</pre>
					</div>
					{#if isStreaming}
						<div class="cursor"></div>
					{/if}
				</div>
			{/if}
		</div>
	{:else}
		<div class="welcome">
			<h1>fm-my-canvas</h1>
			<p>Create a new session to start chatting with the AI.</p>
			<button class="welcome-btn" onclick={() => createNewSession()}>New Chat</button>
		</div>
	{/if}

	<ChatInput onsend={handleSend} disabled={isStreaming} />
</div>

<style>
	.main-area {
		flex: 1;
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
	}

	.chat-container {
		flex: 1;
		overflow-y: auto;
		padding: 1rem;
		display: flex;
		flex-direction: column;
		gap: 0.8rem;
	}

	.streaming {
		background-color: #2d3748;
		color: #e2e8f0;
		padding: 0.8rem 1rem;
		border-radius: 8px;
		max-width: 85%;
		align-self: flex-start;
		position: relative;
	}

	.streaming .message-role {
		font-size: 0.7rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		margin-bottom: 0.3rem;
		opacity: 0.7;
	}

	.streaming .message-content {
		font-size: 0.9rem;
		line-height: 1.5;
	}

	.streaming .message-content pre {
		margin: 0;
		white-space: pre-wrap;
		word-break: break-word;
		font-family: inherit;
	}

	.cursor {
		display: inline-block;
		width: 6px;
		height: 14px;
		background-color: #63b3ed;
		animation: blink 1s infinite;
		margin-left: 2px;
		vertical-align: text-bottom;
	}

	@keyframes blink {
		0%, 100% { opacity: 1; }
		50% { opacity: 0; }
	}

	.welcome {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 1rem;
		color: #a0aec0;
	}

	.welcome h1 {
		font-size: 2rem;
		font-weight: 700;
		color: #e2e8f0;
		margin: 0;
	}

	.welcome p {
		font-size: 1rem;
		margin: 0;
	}

	.welcome-btn {
		padding: 0.7rem 2rem;
		font-size: 1rem;
		background-color: #3b82f6;
		color: white;
		border: none;
		border-radius: 8px;
		cursor: pointer;
		margin-top: 0.5rem;
	}

	.welcome-btn:hover {
		background-color: #2563eb;
	}
</style>
