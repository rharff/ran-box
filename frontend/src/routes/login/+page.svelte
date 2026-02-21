<script lang="ts">
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import { auth } from '$lib/auth.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Card from '$lib/components/ui/card';

	let email = $state('');
	let password = $state('');
	let error = $state('');

	// Redirect authenticated users to dashboard
	$effect(() => {
		if (browser && auth.initialized && auth.isAuthenticated) {
			goto('/dashboard', { replaceState: true });
		}
	});

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		try {
			await auth.login(email, password);
		} catch (err: any) {
			error = err?.response?.data?.message ?? 'Login failed. Please try again.';
		}
	}
</script>

<div class="flex min-h-screen items-center justify-center bg-background px-4">
	<Card.Root class="w-full max-w-sm">
		<Card.Header class="space-y-1">
			<div class="mb-2 flex items-center gap-2">
				<span class="text-2xl">📦</span>
				<span class="text-xl font-bold tracking-tight">Naratel Box</span>
			</div>
			<Card.Title class="text-2xl">Sign in</Card.Title>
			<Card.Description>Enter your credentials to access your files</Card.Description>
		</Card.Header>

		<Card.Content>
			<form onsubmit={handleSubmit} class="space-y-4">
				<div class="space-y-2">
					<Label for="email">Email</Label>
					<Input
						id="email"
						type="email"
						placeholder="you@example.com"
						bind:value={email}
						required
						autocomplete="email"
					/>
				</div>

				<div class="space-y-2">
					<Label for="password">Password</Label>
					<Input
						id="password"
						type="password"
						placeholder="••••••••"
						bind:value={password}
						required
						autocomplete="current-password"
					/>
				</div>

				{#if error}
					<p class="text-sm text-destructive">{error}</p>
				{/if}

				<Button type="submit" class="w-full" disabled={auth.loading}>
					{auth.loading ? 'Signing in…' : 'Sign in'}
				</Button>
			</form>
		</Card.Content>

		<Card.Footer class="justify-center">
			<p class="text-sm text-muted-foreground">
				Don't have an account?
				<a href="/register" class="font-medium text-primary underline-offset-4 hover:underline">
					Register
				</a>
			</p>
		</Card.Footer>
	</Card.Root>
</div>
