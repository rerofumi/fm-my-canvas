<script lang="ts">
	import type { ChatMessage } from '../../lib/stores/chat.svelte';

	let { message }: { message: ChatMessage } = $props();

	let isUser = $derived(message.role === 'user');
	let content = $derived(message.content);
</script>

<div class="message" class:user={isUser} class:assistant={!isUser}>
	<div class="message-role">{isUser ? 'You' : 'Assistant'}</div>
	<div class="message-content">
		<pre>{content}</pre>
	</div>
</div>

<style>
	.message {
		padding: 0.8rem 1rem;
		border-radius: 8px;
		max-width: 85%;
	}

	.user {
		background-color: #2563eb;
		color: white;
		align-self: flex-end;
		margin-left: auto;
	}

	.assistant {
		background-color: #2d3748;
		color: #e2e8f0;
		align-self: flex-start;
	}

	.message-role {
		font-size: 0.7rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		margin-bottom: 0.3rem;
		opacity: 0.7;
	}

	.message-content {
		font-size: 0.9rem;
		line-height: 1.5;
	}

	.message-content pre {
		margin: 0;
		white-space: pre-wrap;
		word-break: break-word;
		font-family: inherit;
	}
</style>
