<script lang="ts">
	import { getConsoleLogs, clearConsoleLogs } from '../../lib/stores/chat.svelte';

	let logs = $derived(getConsoleLogs());
	let container: HTMLDivElement | undefined = $state();
	let filter = $state<'all' | 'log' | 'error' | 'warn' | 'info'>('all');

	let logCount = $derived(logs.length);

	$effect(() => {
		logCount;
		if (container) {
			container.scrollTop = container.scrollHeight;
		}
	});

	let filteredLogs = $derived(
		filter === 'all' ? logs : logs.filter(log => log.type === filter)
	);

	function clearLogs() {
		clearConsoleLogs();
	}
</script>

<div class="console-pane">
	<div class="console-header">
		<div class="console-title">Console</div>
		<div class="console-controls">
			<select
				bind:value={filter}
				class="filter-select"
			>
				<option value="all">All</option>
				<option value="log">Log</option>
				<option value="error">Error</option>
				<option value="warn">Warn</option>
				<option value="info">Info</option>
			</select>
			<button class="clear-btn" onclick={clearLogs}>Clear</button>
		</div>
	</div>
	<div class="console-body" bind:this={container}>
		{#each filteredLogs as log, i (i)}
			<div class="console-entry" class:log-error={log.type === 'error'} class:log-warn={log.type === 'warn'} class:log-info={log.type === 'info'} class:log-iframe={log.source === 'iframe'}>
				<span class="log-timestamp">[{log.timestamp}]</span>
				<span class="log-type">{log.type.toUpperCase()}:</span>
				{#if log.source === 'iframe'}
					<span class="log-source">[preview]</span>
				{/if}
				<pre class="log-message">{log.message}</pre>
			</div>
		{/each}
		{#if filteredLogs.length === 0}
			<div class="console-empty">No console output yet</div>
		{/if}
	</div>
</div>

<style>
	.console-pane {
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
	}

	.console-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0.4rem 0.8rem;
		background-color: #0f1724;
		border-bottom: 1px solid #2d3748;
	}

	.console-title {
		font-size: 0.8rem;
		color: #a0aec0;
		font-weight: 600;
	}

	.console-controls {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.filter-select {
		padding: 0.15rem 0.5rem;
		font-size: 0.7rem;
		background-color: #2d3748;
		color: #a0aec0;
		border: 1px solid #4a5568;
		border-radius: 3px;
		cursor: pointer;
	}

	.filter-select:hover {
		background-color: #4a5568;
	}

	.clear-btn {
		padding: 0.15rem 0.5rem;
		font-size: 0.7rem;
		background-color: #2d3748;
		color: #a0aec0;
		border: none;
		border-radius: 3px;
		cursor: pointer;
	}

	.clear-btn:hover {
		background-color: #4a5568;
		color: #e2e8f0;
	}

	.console-body {
		flex: 1;
		overflow-y: auto;
		padding: 0.5rem;
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
	}

	.console-entry {
		display: flex;
		gap: 0.5rem;
		font-size: 0.75rem;
		line-height: 1.4;
		align-items: flex-start;
	}

	.log-timestamp {
		color: #718096;
		flex-shrink: 0;
	}

	.log-type {
		color: #63b3ed;
		flex-shrink: 0;
		font-weight: 600;
	}

	.log-source {
		color: #68d391;
		flex-shrink: 0;
		font-size: 0.65rem;
		opacity: 0.7;
	}

	.console-entry.log-error .log-type {
		color: #fc8181;
	}

	.console-entry.log-warn .log-type {
		color: #f6ad55;
	}

	.console-entry.log-info .log-type {
		color: #68d391;
	}

	.log-message {
		margin: 0;
		flex: 1;
		white-space: pre-wrap;
		word-break: break-word;
		font-family: 'Cascadia Code', 'Fira Code', 'JetBrains Mono', 'Consolas', monospace;
		color: #e2e8f0;
	}

	.console-empty {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 100%;
		color: #4a5568;
		font-size: 0.85rem;
	}
</style>
