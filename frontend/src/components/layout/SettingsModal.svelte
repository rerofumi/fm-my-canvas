<script lang="ts">
	import type { config } from '../../../wailsjs/go/models';

	let {
		onclose,
		cfg,
	}: {
		onclose: () => void;
		cfg: config.Config;
	} = $props();

	let draft = $state({ ...cfg });
	let saving = $state(false);
	let saved = $state(false);

	async function handleSave() {
		saving = true;
		try {
			const { SaveConfig } = await import('../../../wailsjs/go/main/ChatService');
			await SaveConfig(draft);
			Object.assign(cfg, draft);
			saved = true;
			setTimeout(() => { saved = false; }, 1500);
		} finally {
			saving = false;
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') onclose();
	}
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="overlay" onclick={onclose} role="presentation">
	<div class="modal" onclick={(e) => e.stopPropagation()} role="dialog">
		<div class="modal-header">
			<h2>Settings</h2>
			<button class="close-btn" onclick={onclose}>x</button>
		</div>

		<div class="modal-body">
			<div class="form-group">
				<label for="provider">Provider</label>
				<select id="provider" bind:value={draft.provider}>
					<option value="ollama">Ollama (Local)</option>
					<option value="openrouter">OpenRouter</option>
				</select>
			</div>

			{#if draft.provider === 'ollama'}
				<div class="form-group">
					<label for="ollama-endpoint">Ollama Endpoint</label>
					<input id="ollama-endpoint" type="text" bind:value={draft.ollama_endpoint} placeholder="http://localhost:11434" />
				</div>
				<div class="form-group">
					<label for="ollama-model">Model Name</label>
					<input id="ollama-model" type="text" bind:value={draft.ollama_model} placeholder="llama3" />
				</div>
			{:else}
				<div class="form-group">
					<label for="openrouter-key">API Key</label>
					<input id="openrouter-key" type="password" bind:value={draft.openrouter_api_key} placeholder="sk-..." />
				</div>
				<div class="form-group">
					<label for="openrouter-model">Model</label>
					<input id="openrouter-model" type="text" bind:value={draft.openrouter_model} placeholder="openai/gpt-4o" />
				</div>
			{/if}
		</div>

		<div class="modal-footer">
			{#if saved}
				<span class="saved-msg">Saved!</span>
			{/if}
			<button class="save-btn" onclick={handleSave} disabled={saving}>
				{saving ? 'Saving...' : 'Save'}
			</button>
		</div>
	</div>
</div>

<style>
	.overlay {
		position: fixed;
		inset: 0;
		background-color: rgba(0, 0, 0, 0.6);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 100;
	}

	.modal {
		background-color: #1a2744;
		border: 1px solid #2d3748;
		border-radius: 12px;
		width: 480px;
		max-width: 90vw;
		max-height: 80vh;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
	}

	.modal-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 1rem 1.2rem;
		border-bottom: 1px solid #2d3748;
	}

	.modal-header h2 {
		margin: 0;
		font-size: 1.1rem;
		color: #e2e8f0;
	}

	.close-btn {
		background: none;
		border: none;
		color: #718096;
		font-size: 1.1rem;
		cursor: pointer;
		padding: 0.2rem 0.5rem;
		border-radius: 4px;
	}

	.close-btn:hover {
		color: #e2e8f0;
		background-color: #2d3748;
	}

	.modal-body {
		padding: 1.2rem;
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
	}

	.form-group label {
		font-size: 0.8rem;
		font-weight: 600;
		color: #a0aec0;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	.form-group input,
	.form-group select {
		padding: 0.5rem 0.7rem;
		font-size: 0.9rem;
		font-family: inherit;
		background-color: #0f1724;
		color: #e2e8f0;
		border: 1px solid #2d3748;
		border-radius: 6px;
		outline: none;
	}

	.form-group input:focus,
	.form-group select:focus {
		border-color: #3b82f6;
	}

	.form-group select option {
		background-color: #0f1724;
	}

	.modal-footer {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		gap: 0.8rem;
		padding: 1rem 1.2rem;
		border-top: 1px solid #2d3748;
	}

	.saved-msg {
		color: #68d391;
		font-size: 0.85rem;
	}

	.save-btn {
		padding: 0.5rem 1.5rem;
		font-size: 0.9rem;
		background-color: #3b82f6;
		color: white;
		border: none;
		border-radius: 6px;
		cursor: pointer;
	}

	.save-btn:hover:not(:disabled) {
		background-color: #2563eb;
	}

	.save-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
</style>
