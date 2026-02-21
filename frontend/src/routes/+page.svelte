<script lang="ts">
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import { auth } from '$lib/auth.svelte';

	$effect(() => { auth.init(); });

	$effect(() => {
		if (!browser || !auth.initialized) return;
		if (auth.isAuthenticated) {
			goto('/dashboard', { replaceState: true });
		} else {
			goto('/login', { replaceState: true });
		}
	});
</script>

<!-- Blank page while resolving auth -->
<div class="flex min-h-screen items-center justify-center bg-background">
	<div class="flex items-center gap-3 text-muted-foreground">
		<svg class="h-5 w-5 animate-spin" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
			<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
			<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
		</svg>
		<span class="text-sm">Loading…</span>
	</div>
</div>
