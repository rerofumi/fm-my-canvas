<script lang="ts">
	import FileTree from './FileTree.svelte';
	import CodeEditor from './CodeEditor.svelte';
	import PreviewPane from './PreviewPane.svelte';
	import ConsolePane from './ConsolePane.svelte';
	import { getArtifactFiles } from '../../lib/stores/chat.svelte';

	let activeTab = $state<'code' | 'preview' | 'console'>('preview');
	let files = $derived(getArtifactFiles());

	$effect(() => {
		if (files.length > 0) {
			activeTab = 'code';
		}
	});

	function handleClose() {
		const event = new CustomEvent('artifact-close', { bubbles: true });
		dispatchEvent(event);
	}
</script>

<div class="artifact-panel">
	<div class="panel-tabs">
		<button class="tab" class:active={activeTab === 'preview'} onclick={() => activeTab = 'preview'}>Preview</button>
		<button class="tab" class:active={activeTab === 'code'} onclick={() => activeTab = 'code'}>Code</button>
		<button class="tab" class:active={activeTab === 'console'} onclick={() => activeTab = 'console'}>Console</button>
		<button class="close-btn" onclick={handleClose}>×</button>
	</div>

	<div class="panel-body">
		{#if activeTab === 'code'}
			<div class="code-split">
				<FileTree />
				<CodeEditor />
			</div>
		{:else if activeTab === 'console'}
			<ConsolePane />
		{:else}
			<PreviewPane />
		{/if}
	</div>
</div>

<style>
	.artifact-panel {
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
	}

	.panel-tabs {
		display: flex;
		background-color: #0f1724;
		border-bottom: 1px solid #2d3748;
	}

	.tab {
		flex: 1;
		padding: 0.5rem;
		font-size: 0.8rem;
		font-weight: 600;
		background: none;
		border: none;
		border-bottom: 2px solid transparent;
		color: #718096;
		cursor: pointer;
		transition: color 0.15s, border-color 0.15s;
	}

	.tab:hover {
		color: #a0aec0;
	}

	.tab.active {
		color: #63b3ed;
		border-bottom-color: #3b82f6;
	}

	.close-btn {
		padding: 0 0.8rem;
		background: none;
		border: none;
		color: #718096;
		cursor: pointer;
		font-size: 1.2rem;
		line-height: 1;
		transition: color 0.15s;
	}

	.close-btn:hover {
		color: #e53e3e;
	}

	.panel-body {
		flex: 1;
		overflow: hidden;
	}

	.code-split {
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
	}
</style>
