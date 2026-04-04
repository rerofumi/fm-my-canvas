<script lang="ts">
	import { getToolCallLog } from '../../lib/stores/chat.svelte';

	let toolCallLog = $derived(getToolCallLog());
</script>

{#if toolCallLog.length > 0}
	<div class="tool-call-log">
		<div class="tool-log-header">Tool Calls</div>
		{#each toolCallLog as entry, i (i)}
			<div class="tool-entry" class:running={entry.status === 'running'} class:success={entry.status === 'success'} class:error={entry.status === 'error'}>
				<div class="tool-entry-header">
					<span class="tool-status-icon">
						{#if entry.status === 'running'}
							<span class="spinner"></span>
						{:else if entry.status === 'success'}
							&#10003;
						{:else}
							&#10007;
						{/if}
					</span>
					<span class="tool-name">{entry.toolName}</span>
				</div>
				<details class="tool-details">
					<summary class="tool-summary">Details</summary>
					<div class="tool-detail-content">
						<div class="tool-args">
							<pre>{entry.toolArgs}</pre>
						</div>
						{#if entry.result}
							<div class="tool-result">
								<pre>{entry.result}</pre>
							</div>
						{/if}
					</div>
				</details>
			</div>
		{/each}
	</div>
{/if}

<style>
	.tool-call-log {
		background-color: #1a2744;
		border: 1px solid #2d3748;
		border-radius: 8px;
		padding: 0.6rem 0.8rem;
		max-width: 85%;
		align-self: flex-start;
		margin: 0.3rem 0;
	}

	.tool-log-header {
		font-size: 0.7rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: #a0aec0;
		margin-bottom: 0.4rem;
	}

	.tool-entry {
		padding: 0.35rem 0;
		border-bottom: 1px solid #2d3748;
	}

	.tool-entry:last-child {
		border-bottom: none;
	}

	.tool-entry-header {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		font-size: 0.85rem;
	}

	.tool-status-icon {
		font-size: 0.75rem;
		width: 16px;
		text-align: center;
	}

	.success .tool-status-icon {
		color: #68d391;
	}

	.error .tool-status-icon {
		color: #fc8181;
	}

	.running .tool-status-icon {
		color: #63b3ed;
	}

	.tool-name {
		color: #63b3ed;
		font-family: 'Cascadia Code', 'Fira Code', 'Consolas', monospace;
		font-size: 0.8rem;
	}

	.spinner {
		display: inline-block;
		width: 10px;
		height: 10px;
		border: 2px solid #63b3ed;
		border-top-color: transparent;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}

	.tool-details {
		margin-top: 0.2rem;
		margin-left: 1.2rem;
	}

	.tool-summary {
		font-size: 0.7rem;
		color: #718096;
		cursor: pointer;
		user-select: none;
		list-style: none;
	}

	.tool-summary::-webkit-details-marker {
		display: none;
	}

	.tool-detail-content {
		margin-top: 0.3rem;
	}

	.tool-args,
	.tool-result {
		margin: 0.2rem 0;
	}

	.tool-args pre,
	.tool-result pre {
		margin: 0;
		padding: 0.4rem;
		font-size: 0.75rem;
		line-height: 1.4;
		font-family: 'Cascadia Code', 'Fira Code', 'Consolas', monospace;
		color: #e2e8f0;
		background-color: #111b2e;
		border-radius: 4px;
		white-space: pre-wrap;
		word-break: break-word;
		max-height: 200px;
		overflow-y: auto;
	}

	.tool-result pre {
		border: 1px solid #2d3748;
	}
</style>
