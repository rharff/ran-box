<script lang="ts">
	import { auth } from '$lib/auth.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Card from '$lib/components/ui/card';

	let email = $state('');
	let password = $state('');
	let confirm = $state('');
	let error = $state('');

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		if (password !== confirm) {
			error = 'Passwords do not match.';
			return;
		}
		if (password.length < 8) {
			error = 'Password must be at least 8 characters.';
			return;
		}
		try {
			await auth.register(email, password);
		} catch (err: any) {
			error = err?.response?.data?.message ?? 'Registration failed. Please try again.';
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
			<Card.Title class="text-2xl">Create account</Card.Title>
			<Card.Description>Start storing your files securely</Card.Description>
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
						placeholder="Min. 8 characters"
						bind:value={password}
						required
						autocomplete="new-password"
					/>
				</div>

				<div class="space-y-2">
					<Label for="confirm">Confirm password</Label>
					<Input
						id="confirm"
						type="password"
						placeholder="••••••••"
						bind:value={confirm}
						required
						autocomplete="new-password"
					/>
				</div>

				{#if error}
					<p class="text-sm text-destructive">{error}</p>
				{/if}

				<Button type="submit" class="w-full" disabled={auth.loading}>
					{auth.loading ? 'Creating account…' : 'Create account'}
				</Button>
			</form>
		</Card.Content>

		<Card.Footer class="justify-center">
			<p class="text-sm text-muted-foreground">
				Already have an account?
				<a href="/login" class="font-medium text-primary underline-offset-4 hover:underline">
					Sign in
				</a>
			</p>
		</Card.Footer>
	</Card.Root>
</div>
