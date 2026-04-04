<script lang="ts">
	import { getArtifactFiles, getSelectedFilePath } from '../../lib/stores/chat.svelte';

	let files = $derived(getArtifactFiles());
	let selectedPath = $derived(getSelectedFilePath());

	let selectedFile = $derived(files.find(f => f.path === selectedPath));
</script>

<div class="code-editor">
	{#if selectedFile}
		<div class="editor-header">
			<span class="filename">{selectedFile.path}</span>
			<span class="language-badge">{selectedFile.language}</span>
		</div>
		<pre class="code-content"><code>{selectedFile.content}</code></pre>
	{:else if files.length > 0}
		<div class="placeholder">Select a file to view its code</div>
	{:else}
		<div class="placeholder">No files generated yet</div>
	{/if}
</div>

<style>
	.code-editor {
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
	}

	.editor-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0.4rem 0.8rem;
		background-color: #0f1724;
		border-bottom: 1px solid #2d3748;
	}

	.filename {
		font-size: 0.8rem;
		color: #a0aec0;
	}

	.language-badge {
		font-size: 0.65rem;
		padding: 0.1rem 0.5rem;
		background-color: #2d3748;
		color: #63b3ed;
		border-radius: 3px;
		text-transform: uppercase;
	}

	.code-content {
		flex: 1;
		margin: 0;
		padding: 0.8rem;
		font-size: 0.82rem;
		line-height: 1.6;
		font-family: 'Cascadia Code', 'Fira Code', 'JetBrains Mono', 'Consolas', monospace;
		color: #e2e8f0;
		background-color: #111b2e;
		overflow: auto;
		white-space: pre;
		tab-size: 2;
	}

	.code-content code {
		font-family: inherit;
	}

	.placeholder {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 100%;
		color: #4a5568;
		font-size: 0.85rem;
	}
</style>
