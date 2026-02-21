<script lang="ts">
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import { auth } from '$lib/auth.svelte';
	import { fileStore } from '$lib/file-store.svelte';
	import { formatBytes } from '$lib/utils/format';
	import { Button } from '$lib/components/ui/button';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';

	let { children } = $props();

	$effect(() => {
		if (!browser || !auth.initialized) return;
		if (!auth.isAuthenticated) goto('/login', { replaceState: true });
	});

	const DISPLAY_CAP = 15 * 1024 * 1024 * 1024;
	let usedPct = $derived(Math.min((fileStore.totalSize / DISPLAY_CAP) * 100, 100));

	let sidebarOpen = $state(true);
	let mobileDrawerOpen = $state(false);

	function closeMobileDrawer() { mobileDrawerOpen = false; }

	let searchTimeout: ReturnType<typeof setTimeout>;
	function handleSearch(e: Event) {
		const value = (e.target as HTMLInputElement).value;
		clearTimeout(searchTimeout);
		searchTimeout = setTimeout(() => fileStore.setSearch(value), 300);
	}
</script>

<!-- Hidden upload input shared across triggers -->
<input id="sidebar-upload" type="file" multiple class="hidden"
	onchange={(e) => {
		const f = (e.target as HTMLInputElement).files;
		if (f?.length) fileStore.upload(f);
		(e.target as HTMLInputElement).value = '';
		closeMobileDrawer();
	}}
/>

{#if auth.initialized && auth.isAuthenticated}
<div class="flex h-screen overflow-hidden bg-background">

	<!-- ── Mobile drawer backdrop ── -->
	{#if mobileDrawerOpen}
		<div
			class="fixed inset-0 z-30 bg-black/40 md:hidden"
			onclick={closeMobileDrawer}
			role="presentation"
		></div>
	{/if}

	<!-- ── Sidebar ── -->
	<aside class={[
		'flex flex-col border-r bg-card z-40 transition-all duration-200',
		'md:relative md:translate-x-0',
		sidebarOpen ? 'md:w-60' : 'md:w-16',
		'fixed inset-y-0 left-0 w-72',
		mobileDrawerOpen ? 'translate-x-0 shadow-2xl' : '-translate-x-full md:translate-x-0'
	].join(' ')}>

		<!-- Logo row -->
		<div class="flex h-16 items-center gap-2 px-4 border-b flex-shrink-0">
			<span class="text-2xl flex-shrink-0">📦</span>
			<span class={`font-bold text-base tracking-tight ${sidebarOpen ? 'md:block' : 'md:hidden'} block`}>Naratel Box</span>
			<button onclick={closeMobileDrawer} aria-label="Close menu"
				class="ml-auto rounded-md p-1.5 hover:bg-accent md:hidden">
				<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
				</svg>
			</button>
		</div>

		<!-- Upload button -->
		<div class="px-3 py-4">
			<Button class="w-full gap-2 shadow-sm justify-start"
				onclick={() => document.getElementById('sidebar-upload')?.click()}>
				<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
				</svg>
				<span class={sidebarOpen ? 'md:inline' : 'md:hidden'}>New upload</span>
			</Button>
		</div>

		<!-- Nav -->
		<nav class="flex-1 px-2 space-y-1">
			<button onclick={() => { fileStore.navigateToFolder(null); closeMobileDrawer(); }}
				class="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium bg-accent text-accent-foreground">
				<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" />
				</svg>
				<span class={sidebarOpen ? 'md:inline' : 'md:hidden'}>All Files</span>
			</button>
		</nav>

		<!-- Storage bar -->
		<div class={`border-t px-4 py-4 ${sidebarOpen ? 'md:block' : 'md:hidden'} block`}>
			<p class="text-xs font-medium text-muted-foreground mb-2">Storage used</p>
			<div class="h-1.5 w-full rounded-full bg-muted overflow-hidden">
				<div class="h-full rounded-full bg-primary transition-all" style="width: {usedPct}%"></div>
			</div>
			<p class="mt-1.5 text-xs text-muted-foreground">{formatBytes(fileStore.totalSize)}</p>
		</div>

		<!-- Account -->
		<div class="border-t p-3">
			<DropdownMenu.Root>
				<DropdownMenu.Trigger class="flex w-full items-center gap-2 rounded-lg p-2 text-sm hover:bg-accent">
					<div class="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground text-xs font-bold">
						{auth.user?.email?.[0]?.toUpperCase() ?? '?'}
					</div>
					<span class={`min-w-0 flex-1 truncate text-left text-xs text-muted-foreground ${sidebarOpen ? 'md:block' : 'md:hidden'} block`}>
						{auth.user?.email ?? ''}
					</span>
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="start" class="w-52">
					<DropdownMenu.Label class="text-xs font-normal text-muted-foreground">{auth.user?.email ?? ''}</DropdownMenu.Label>
					<DropdownMenu.Separator />
					<DropdownMenu.Item onclick={() => auth.logout()} class="text-destructive focus:text-destructive">Sign out</DropdownMenu.Item>
				</DropdownMenu.Content>
			</DropdownMenu.Root>
		</div>
	</aside>

	<!-- ── Main content area ── -->
	<div class="flex flex-1 flex-col overflow-hidden">
		<!-- Top bar -->
		<header class="flex h-14 items-center gap-2 border-b bg-card px-4 flex-shrink-0 md:h-16 md:px-6 md:gap-3">
			<button
				onclick={() => { if (window.innerWidth < 768) mobileDrawerOpen = true; else sidebarOpen = !sidebarOpen; }}
				class="rounded-md p-1.5 text-muted-foreground hover:bg-accent hover:text-accent-foreground flex-shrink-0"
				aria-label="Toggle sidebar"
			>
				<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M4 6h16M4 12h16M4 18h16" />
				</svg>
			</button>

			<!-- Search -->
			<div class="relative flex-1 max-w-xl">
				<svg xmlns="http://www.w3.org/2000/svg" class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<circle cx="11" cy="11" r="8"/><path stroke-linecap="round" d="M21 21l-4.35-4.35"/>
				</svg>
				<input type="search" placeholder="Search files…"
					class="w-full rounded-full border bg-muted/40 py-2 pl-10 pr-4 text-sm outline-none focus:ring-2 focus:ring-ring transition-all"
					oninput={handleSearch}
				/>
			</div>

			<!-- Mobile upload button -->
			<button
				onclick={() => document.getElementById('sidebar-upload')?.click()}
				class="flex-shrink-0 rounded-md p-1.5 text-muted-foreground hover:bg-accent hover:text-accent-foreground md:hidden"
				aria-label="Upload file"
			>
				<svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
				</svg>
			</button>
		</header>

		<main class="flex-1 overflow-y-auto">
			{@render children()}
		</main>
	</div>
</div>
{:else}
<div class="flex min-h-screen items-center justify-center bg-background">
	<div class="flex items-center gap-3 text-muted-foreground">
		<svg class="h-5 w-5 animate-spin" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
			<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
			<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
		</svg>
		<span class="text-sm">Loading…</span>
	</div>
</div>
{/if}
