<script lang="ts">
	import { getPreviewUrl } from '../../lib/stores/chat.svelte';

	let previewUrl = $derived(getPreviewUrl());
	let reloadCount = $state(0);

	function getSrc() {
		if (!previewUrl) return '';
		const sep = previewUrl.includes('?') ? '&' : '?';
		return previewUrl + sep + 'r=' + reloadCount;
	}

	function handleReload() {
		reloadCount++;
	}
</script>

<div class="preview-pane">
	<div class="preview-header">
		<span class="preview-title">Preview</span>
		{#if previewUrl}
			<button class="reload-btn" onclick={handleReload}>Reload</button>
		{/if}
	</div>
	{#if previewUrl}
		<iframe
			class="preview-iframe"
			src={getSrc()}
			title="Artifact Preview"
			sandbox="allow-scripts allow-forms allow-modals allow-same-origin"
		></iframe>
	{:else}
		<div class="preview-empty">
			<p>Preview will appear here when artifacts are generated</p>
		</div>
	{/if}
</div>

<style>
	.preview-pane {
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
	}

	.preview-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0.4rem 0.8rem;
		background-color: #0f1724;
		border-bottom: 1px solid #2d3748;
	}

	.preview-title {
		font-size: 0.8rem;
		color: #a0aec0;
		font-weight: 600;
	}

	.reload-btn {
		padding: 0.15rem 0.5rem;
		font-size: 0.7rem;
		background-color: #2d3748;
		color: #a0aec0;
		border: none;
		border-radius: 3px;
		cursor: pointer;
	}

	.reload-btn:hover {
		background-color: #4a5568;
		color: #e2e8f0;
	}

	.preview-iframe {
		flex: 1;
		border: none;
		width: 100%;
		background-color: white;
	}

	.preview-empty {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 100%;
		color: #4a5568;
		font-size: 0.85rem;
	}
</style>
