<script lang="ts">
	let { onsend, onstop, disabled = false }: { onsend: (text: string) => void; onstop: () => void; disabled?: boolean } = $props();

	let inputText = $state('');
	let textareaEl: HTMLTextAreaElement | undefined = $state();

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && e.ctrlKey) {
			e.preventDefault();
			handleSend();
		}
	}

	function handleSend() {
		if (!inputText.trim() || disabled) return;
		onsend(inputText.trim());
		inputText = '';
		resetHeight();
	}

	function autoResize() {
		if (!textareaEl) return;
		textareaEl.style.height = 'auto';
		textareaEl.style.height = Math.min(textareaEl.scrollHeight, 150) + 'px';
	}

	function resetHeight() {
		if (!textareaEl) return;
		textareaEl.style.height = 'auto';
	}
</script>

<div class="chat-input-container">
	<textarea
		bind:this={textareaEl}
		bind:value={inputText}
		onkeydown={handleKeydown}
		oninput={autoResize}
		placeholder="Type a message... (Ctrl+Enter to send)"
		disabled={disabled}
		rows="2"
		class="chat-textarea"
	></textarea>
	{#if disabled}
		<div class="streaming-controls">
			<span class="pilot-lamp"></span>
			<span class="streaming-label">Working...</span>
			<button class="stop-btn" onclick={onstop}>&#9632; Stop</button>
		</div>
	{:else}
		<button class="send-btn" onclick={handleSend} disabled={!inputText.trim()}>
			Send
		</button>
	{/if}
</div>

<style>
	.chat-input-container {
		display: flex;
		gap: 0.5rem;
		padding: 0.8rem 1rem;
		border-top: 1px solid #2d3748;
		background-color: #0f1724;
	}

	.chat-textarea {
		flex: 1;
		padding: 0.6rem 0.8rem;
		font-size: 0.9rem;
		font-family: inherit;
		background-color: #1a2744;
		color: #e2e8f0;
		border: 1px solid #2d3748;
		border-radius: 6px;
		resize: none;
		outline: none;
		min-height: 42px;
		max-height: 150px;
		line-height: 1.4;
	}

	.chat-textarea:focus {
		border-color: #3b82f6;
	}

	.chat-textarea:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.chat-textarea::placeholder {
		color: #4a5568;
	}

	.send-btn {
		padding: 0.6rem 1.2rem;
		font-size: 0.9rem;
		background-color: #3b82f6;
		color: white;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		white-space: nowrap;
		align-self: flex-end;
	}

	.send-btn:hover:not(:disabled) {
		background-color: #2563eb;
	}

	.send-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.streaming-controls {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		align-self: flex-end;
		padding: 0.4rem 0;
	}

	.pilot-lamp {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background-color: #48bb78;
		animation: pulse 1.5s ease-in-out infinite;
		flex-shrink: 0;
	}

	@keyframes pulse {
		0%, 100% { opacity: 1; box-shadow: 0 0 4px #48bb78; }
		50% { opacity: 0.3; box-shadow: 0 0 0px #48bb78; }
	}

	.streaming-label {
		font-size: 0.8rem;
		color: #a0aec0;
		white-space: nowrap;
	}

	.stop-btn {
		padding: 0.5rem 1rem;
		font-size: 0.85rem;
		background-color: #c53030;
		color: white;
		border: none;
		border-radius: 6px;
		cursor: pointer;
		white-space: nowrap;
		transition: background-color 0.15s;
	}

	.stop-btn:hover {
		background-color: #e53e3e;
	}
</style>
