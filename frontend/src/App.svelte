<script lang="ts">
	import { onMount } from 'svelte';
	import { loadSessions, registerLLMListener, loadConfig } from './lib/services/wails';
	import type { config } from '../wailsjs/go/models';
	import Sidebar from './components/layout/Sidebar.svelte';
	import ChatArea from './components/chat/ChatArea.svelte';
	import SettingsModal from './components/layout/SettingsModal.svelte';

	let showSettings = $state(false);
	let appConfig = $state<config.Config | null>(null);

	onMount(async () => {
		registerLLMListener();
		loadSessions();
		appConfig = await loadConfig();
	});

	function handleOpenSettings() {
		showSettings = true;
	}

	function handleCloseSettings() {
		showSettings = false;
	}
</script>

<div class="app-layout">
	<Sidebar onopensettings={handleOpenSettings} />
	<ChatArea />
</div>

{#if showSettings && appConfig}
	<SettingsModal onclose={handleCloseSettings} cfg={appConfig} />
{/if}
