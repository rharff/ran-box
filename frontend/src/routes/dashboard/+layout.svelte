<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { auth } from '$lib/auth.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Separator } from '$lib/components/ui/separator';

	let { children } = $props();

	$effect(() => {
		if (!auth.isAuthenticated) {
			goto('/login');
		}
	});

	// Init user profile once on mount
	$effect(() => {
		auth.init();
	});
</script>

{#if auth.isAuthenticated}
	<div class="min-h-screen bg-background">
		<!-- Top navigation -->
		<header class="border-b bg-card">
			<div class="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">
				<div class="flex items-center gap-2">
					<span class="text-xl">📦</span>
					<span class="text-lg font-bold tracking-tight">Naratel Box</span>
				</div>
				<div class="flex items-center gap-4">
					{#if auth.user}
						<span class="text-sm text-muted-foreground">{auth.user.email}</span>
					{/if}
					<Separator orientation="vertical" class="h-5" />
					<Button variant="ghost" size="sm" onclick={() => auth.logout()}>Sign out</Button>
				</div>
			</div>
		</header>

		<!-- Page content -->
		<main class="mx-auto max-w-5xl px-6 py-8">
			{@render children()}
		</main>
	</div>
{/if}
