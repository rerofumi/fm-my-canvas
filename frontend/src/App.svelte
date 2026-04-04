<script lang="ts">
	import { onMount } from 'svelte';
	import { loadSessions, registerLLMListener, loadConfig } from './lib/services/wails';
	import type { config } from '../wailsjs/go/models';
	import Sidebar from './components/layout/Sidebar.svelte';
	import ChatArea from './components/chat/ChatArea.svelte';
	import ArtifactPanel from './components/artifacts/ArtifactPanel.svelte';
	import SettingsModal from './components/layout/SettingsModal.svelte';
	import { getArtifactFiles } from './lib/stores/chat.svelte';

	let showSettings = $state(false);
	let appConfig = $state<config.Config | null>(null);
	let artifactFiles = $derived(getArtifactFiles());
	let showArtifact = $state(false);

	let sidebarCollapsed = $state(false);
	let artifactWidth = $state(500);
	let dragging = $state(false);
	let lastArtifactCount = $state(0);

	$effect(() => {
		const currentCount = artifactFiles.length;
		if (currentCount > lastArtifactCount && currentCount > 0) {
			showArtifact = true;
		}
		lastArtifactCount = currentCount;
	});

	onMount(async () => {
		registerLLMListener();
		loadSessions();
		appConfig = await loadConfig();

		window.addEventListener('artifact-close', () => {
			showArtifact = false;
		});
	});

	function handleOpenSettings() {
		showSettings = true;
	}

	function handleCloseSettings() {
		showSettings = false;
	}

	function handleToggleSidebar() {
		sidebarCollapsed = !sidebarCollapsed;
	}

	function handleToggleArtifact() {
		showArtifact = !showArtifact;
	}

	function handleDividerDown(e: MouseEvent) {
		e.preventDefault();
		dragging = true;
		const startX = e.clientX;
		const startWidth = artifactWidth;

		function onMouseMove(ev: MouseEvent) {
			const delta = startX - ev.clientX;
			artifactWidth = Math.max(300, Math.min(startWidth + delta, window.innerWidth - 400));
		}

		function onMouseUp() {
			dragging = false;
			document.removeEventListener('mousemove', onMouseMove);
			document.removeEventListener('mouseup', onMouseUp);
		}

		document.addEventListener('mousemove', onMouseMove);
		document.addEventListener('mouseup', onMouseUp);
	}
</script>

<div class="app-layout" class:dragging>
	<Sidebar onopensettings={handleOpenSettings} collapsed={sidebarCollapsed} ontoggle={handleToggleSidebar} />
	<div class="main-wrapper">
		<div class="main-header">
			<button class="artifact-toggle-btn" class:active={showArtifact} onclick={handleToggleArtifact}>
				<span class="toggle-icon">{showArtifact ? '▼' : '▶'}</span>
				{#if artifactFiles.length > 0}
					<span>Artifact ({artifactFiles.length} files)</span>
				{:else}
					<span>Artifact</span>
				{/if}
			</button>
		</div>
		<ChatArea />
	</div>
	{#if showArtifact}
		<div class="divider" role="separator" onmousedown={handleDividerDown}></div>
		<div class="artifact-wrapper" style="width: {artifactWidth}px; min-width: {artifactWidth}px;">
			<ArtifactPanel />
		</div>
	{/if}
</div>

{#if showSettings && appConfig}
	<SettingsModal onclose={handleCloseSettings} cfg={appConfig} />
{/if}

<style>
	.app-layout {
		display: flex;
		flex-direction: row;
		height: 100%;
		flex: 1;
		overflow: hidden;
	}

	.app-layout.dragging {
		cursor: col-resize;
		user-select: none;
	}

	.main-wrapper {
		flex: 1;
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
		min-width: 0;
	}

	.main-header {
		display: flex;
		align-items: center;
		padding: 0.5rem 1rem;
		background-color: #0f1724;
		border-bottom: 1px solid #2d3748;
		gap: 0.5rem;
	}

	.artifact-toggle-btn {
		display: flex;
		align-items: center;
		gap: 0.4rem;
		padding: 0.3rem 0.8rem;
		background-color: #1a2744;
		color: #a0aec0;
		border: 1px solid #2d3748;
		border-radius: 4px;
		cursor: pointer;
		font-size: 0.8rem;
		transition: background-color 0.15s, border-color 0.15s, color 0.15s;
	}

	.artifact-toggle-btn:hover {
		background-color: #2d3748;
		border-color: #4a5568;
		color: #e2e8f0;
	}

	.artifact-toggle-btn.active {
		background-color: #1e3a5f;
		border-color: #3b82f6;
		color: #63b3ed;
	}

	.toggle-icon {
		font-size: 0.7rem;
	}

	.divider {
		width: 6px;
		cursor: col-resize;
		background-color: #1a2744;
		flex-shrink: 0;
		transition: background-color 0.15s;
	}

	.divider:hover {
		background-color: #3b82f6;
	}

	.artifact-wrapper {
		display: flex;
		flex-direction: column;
		height: 100%;
		overflow: hidden;
		flex-shrink: 0;
	}
</style>
