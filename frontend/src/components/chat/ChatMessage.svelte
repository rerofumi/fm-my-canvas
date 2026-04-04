<script lang="ts">
	import type { ChatMessage } from '../../lib/stores/chat.svelte';

	let { message, defaultCollapsed = true }: { message: ChatMessage; defaultCollapsed?: boolean } = $props();

	let isUser = $derived(message.role === 'user');

	interface Segment {
		type: 'text' | 'code';
		content: string;
		language?: string;
		path?: string;
	}

	let segments = $derived(parseContent(message.content));

	function parseContent(text: string): Segment[] {
		if (!text) return [];
		const segs: Segment[] = [];
		const re = /```(\w+)(?:\s+path=(\S+))?\s*\n([\s\S]*?)```/g;
		let lastIndex = 0;
		let match: RegExpExecArray | null;

		while ((match = re.exec(text)) !== null) {
			if (match.index > lastIndex) {
				const t = text.substring(lastIndex, match.index).trim();
				if (t) segs.push({ type: 'text', content: t });
			}
			segs.push({
				type: 'code',
				language: match[1],
				path: match[2] || undefined,
				content: match[3].trimEnd(),
			});
			lastIndex = match.index + match[0].length;
		}

		if (lastIndex < text.length) {
			const t = text.substring(lastIndex).trim();
			if (t) segs.push({ type: 'text', content: t });
		}

		return segs;
	}
</script>

{#if isUser}
	<div class="message user">
		<div class="message-role">You</div>
		<div class="message-content">
			<pre>{message.content}</pre>
		</div>
	</div>
{:else}
	<div class="message assistant">
		<div class="message-role">Assistant</div>
		<div class="message-content">
			{#each segments as segment}
				{#if segment.type === 'text'}
					<p class="text-segment">{segment.content}</p>
				{:else}
					<details class="code-block" open={!defaultCollapsed}>
						<summary class="code-summary">
							<span class="code-label">{segment.language || 'code'}{segment.path ? ' \u00B7 ' + segment.path : ''}</span>
						</summary>
						<pre class="code-pre"><code>{segment.content}</code></pre>
					</details>
				{/if}
			{/each}
		</div>
	</div>
{/if}

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

	.text-segment {
		margin: 0.3rem 0;
		white-space: pre-wrap;
		word-break: break-word;
	}

	.code-block {
		margin: 0.5rem 0;
		border: 1px solid #3d4f65;
		border-radius: 6px;
		overflow: hidden;
	}

	.code-summary {
		padding: 0.35rem 0.7rem;
		background-color: #1a2744;
		color: #63b3ed;
		font-size: 0.78rem;
		cursor: pointer;
		user-select: none;
		list-style: none;
		display: flex;
		align-items: center;
		gap: 0.4rem;
	}

	.code-summary::-webkit-details-marker {
		display: none;
	}

	.code-summary::before {
		content: '\25B6';
		font-size: 0.6rem;
		transition: transform 0.15s;
	}

	.code-block[open] .code-summary::before {
		transform: rotate(90deg);
	}

	.code-summary:hover {
		background-color: #243352;
	}

	.code-pre {
		margin: 0;
		padding: 0.6rem;
		font-size: 0.8rem;
		line-height: 1.5;
		font-family: 'Cascadia Code', 'Fira Code', 'JetBrains Mono', 'Consolas', monospace;
		color: #e2e8f0;
		background-color: #111b2e;
		overflow-x: auto;
		white-space: pre;
		max-height: 400px;
	}

	.code-pre code {
		font-family: inherit;
	}
</style>
