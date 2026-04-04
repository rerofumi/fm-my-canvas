<script lang="ts">
	import {
		getSessions,
		getCurrentSessionId,
	} from '../../lib/stores/chat.svelte';
	import { createNewSession, switchSession, deleteSession } from '../../lib/services/wails';

	let { onopensettings }: { onopensettings: () => void } = $props();

	let sessions = $derived(getSessions());
	let currentId = $derived(getCurrentSessionId());

	function handleNewChat() {
		createNewSession();
	}

	function handleSelect(id: string) {
		switchSession(id);
	}

	function handleDelete(e: MouseEvent, id: string) {
		e.stopPropagation();
		deleteSession(id);
	}
</script>

<aside class="sidebar">
	<div class="sidebar-header">
		<h2 class="sidebar-title">Sessions</h2>
		<button class="new-chat-btn" onclick={handleNewChat}>+ New</button>
	</div>

	<div class="session-list">
		{#if sessions.length === 0}
			<p class="empty-message">No sessions yet</p>
		{:else}
			{#each sessions as session (session.id)}
				<div
					class="session-item"
					class:active={session.id === currentId}
					onclick={() => handleSelect(session.id)}
					role="button"
					tabindex="0"
					onkeydown={(e) => e.key === 'Enter' && handleSelect(session.id)}
				>
					<span class="session-title">{session.title}</span>
					<button class="delete-btn" onclick={(e) => handleDelete(e, session.id)}>x</button>
				</div>
			{/each}
		{/if}
	</div>

	<div class="sidebar-footer">
		<button class="settings-btn" onclick={onopensettings}>Settings</button>
	</div>
</aside>

<style>
	.sidebar {
		width: 260px;
		min-width: 260px;
		background-color: #0f1724;
		border-right: 1px solid #2d3748;
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
	}

	.sidebar-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 1rem;
		border-bottom: 1px solid #2d3748;
	}

	.sidebar-title {
		font-size: 0.9rem;
		font-weight: 600;
		color: #a0aec0;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		margin: 0;
	}

	.new-chat-btn {
		padding: 0.3rem 0.7rem;
		font-size: 0.8rem;
		background-color: #3b82f6;
		color: white;
		border: none;
		border-radius: 4px;
		cursor: pointer;
	}

	.new-chat-btn:hover {
		background-color: #2563eb;
	}

	.session-list {
		flex: 1;
		overflow-y: auto;
		padding: 0.5rem;
	}

	.session-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		width: 100%;
		padding: 0.6rem 0.8rem;
		background: none;
		border: none;
		border-radius: 6px;
		color: #cbd5e0;
		font-size: 0.85rem;
		text-align: left;
		cursor: pointer;
		transition: background-color 0.15s;
	}

	.session-item:hover {
		background-color: #1a2744;
	}

	.session-item.active {
		background-color: #1e3a5f;
		color: #ffffff;
	}

	.session-title {
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.delete-btn {
		flex-shrink: 0;
		padding: 0.15rem 0.4rem;
		font-size: 0.7rem;
		background: none;
		border: none;
		border-radius: 3px;
		color: #718096;
		cursor: pointer;
		opacity: 0;
		transition: opacity 0.15s, color 0.15s;
	}

	.session-item:hover .delete-btn {
		opacity: 1;
	}

	.delete-btn:hover {
		color: #fc8181;
		background-color: rgba(252, 129, 129, 0.1);
	}

	.empty-message {
		color: #4a5568;
		font-size: 0.8rem;
		text-align: center;
		padding: 2rem 1rem;
	}

	.sidebar-footer {
		padding: 0.8rem 1rem;
		border-top: 1px solid #2d3748;
	}

	.settings-btn {
		width: 100%;
		padding: 0.5rem;
		font-size: 0.85rem;
		background-color: #1a2744;
		color: #a0aec0;
		border: 1px solid #2d3748;
		border-radius: 6px;
		cursor: pointer;
		transition: background-color 0.15s, color 0.15s;
	}

	.settings-btn:hover {
		background-color: #243352;
		color: #e2e8f0;
	}
</style>
