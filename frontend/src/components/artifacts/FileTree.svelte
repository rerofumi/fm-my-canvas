<script lang="ts">
	import { getArtifactFiles, getSelectedFilePath, setSelectedFilePath } from '../../lib/stores/chat.svelte';

	let files = $derived(getArtifactFiles());
	let selectedPath = $derived(getSelectedFilePath());
</script>

{#if files.length > 0}
	<div class="file-tree">
		<h3 class="tree-title">Files</h3>
		{#each files as file (file.path)}
			<button
				class="file-item"
				class:active={file.path === selectedPath}
				onclick={() => setSelectedFilePath(file.path)}
			>
				<span class="file-icon">{file.language === 'html' ? '</>' : file.language === 'css' ? '#' : '{}'}</span>
				<span class="file-name">{file.path}</span>
			</button>
		{/each}
	</div>
{/if}

<style>
	.file-tree {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
	}

	.tree-title {
		font-size: 0.75rem;
		font-weight: 600;
		color: #718096;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		margin: 0 0 0.3rem 0;
	}

	.file-item {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		width: 100%;
		padding: 0.35rem 0.6rem;
		background: none;
		border: none;
		border-radius: 4px;
		color: #a0aec0;
		font-size: 0.8rem;
		text-align: left;
		cursor: pointer;
		transition: background-color 0.15s, color 0.15s;
	}

	.file-item:hover {
		background-color: #1a2744;
		color: #e2e8f0;
	}

	.file-item.active {
		background-color: #1e3a5f;
		color: #63b3ed;
	}

	.file-icon {
		font-size: 0.7rem;
		opacity: 0.6;
		width: 1.2rem;
		text-align: center;
	}

	.file-name {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
