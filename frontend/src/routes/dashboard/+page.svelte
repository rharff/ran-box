<script lang="ts">
	import { listFiles, uploadFile, deleteFile, downloadFile } from '$lib/api';
	import type { NaratelFile } from '$lib/types';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import { Progress } from '$lib/components/ui/progress';
	import { Separator } from '$lib/components/ui/separator';

	// ── State ───────────────────────────────────────────────────────────────────
	let files = $state<NaratelFile[]>([]);
	let loadingFiles = $state(true);
	let fetchError = $state('');

	let uploading = $state(false);
	let uploadProgress = $state(0);
	let uploadError = $state('');
	let uploadSuccess = $state('');

	let deletingId = $state<number | null>(null);
	let downloadingId = $state<number | null>(null);

	let fileInput = $state<HTMLInputElement>();

	// ── Load files on mount ──────────────────────────────────────────────────────
	$effect(() => {
		fetchFiles();
	});

	async function fetchFiles() {
		loadingFiles = true;
		fetchError = '';
		try {
			files = await listFiles();
		} catch {
			fetchError = 'Failed to load files.';
		} finally {
			loadingFiles = false;
		}
	}

	// ── Upload ───────────────────────────────────────────────────────────────────
	async function handleFileChange(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;

		uploading = true;
		uploadError = '';
		uploadSuccess = '';
		uploadProgress = 0;

		try {
			const res = await uploadFile(file, (pct) => { uploadProgress = pct; });
			uploadSuccess = `"${res.name}" uploaded successfully.`;
			await fetchFiles();
		} catch (err: any) {
			uploadError = err?.response?.data?.message ?? 'Upload failed.';
		} finally {
			uploading = false;
			uploadProgress = 0;
			if (fileInput) fileInput.value = '';
		}
	}

	// ── Delete ───────────────────────────────────────────────────────────────────
	async function handleDelete(id: number, name: string) {
		if (!confirm(`Delete "${name}"? This cannot be undone.`)) return;
		deletingId = id;
		try {
			await deleteFile(id);
			files = files.filter((f) => f.id !== id);
		} catch {
			alert('Failed to delete file.');
		} finally {
			deletingId = null;
		}
	}

	// ── Download ─────────────────────────────────────────────────────────────────
	async function handleDownload(id: number, name: string) {
		downloadingId = id;
		try {
			await downloadFile(id, name);
		} catch {
			alert('Download failed.');
		} finally {
			downloadingId = null;
		}
	}

	// ── Helpers ──────────────────────────────────────────────────────────────────
	function formatBytes(bytes: number): string {
		if (bytes < 1024) return `${bytes} B`;
		if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
		if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
		return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`;
	}

	function formatDate(iso: string): string {
		return new Date(iso).toLocaleDateString(undefined, {
			year: 'numeric', month: 'short', day: 'numeric',
			hour: '2-digit', minute: '2-digit'
		});
	}

	function mimeCategory(mime: string): string {
		if (mime.startsWith('image/')) return 'Image';
		if (mime.startsWith('video/')) return 'Video';
		if (mime.startsWith('audio/')) return 'Audio';
		if (mime.includes('pdf')) return 'PDF';
		if (mime.includes('zip') || mime.includes('tar') || mime.includes('gzip')) return 'Archive';
		if (mime.startsWith('text/')) return 'Text';
		return 'File';
	}
</script>

<!-- Page header -->
<div class="mb-6 flex items-center justify-between">
	<div>
		<h1 class="text-2xl font-bold tracking-tight">My Files</h1>
		<p class="text-sm text-muted-foreground">
			{files.length} file{files.length !== 1 ? 's' : ''} stored
		</p>
	</div>

	<!-- Upload button -->
	<div>
		<input
			bind:this={fileInput}
			type="file"
			id="file-upload"
			class="hidden"
			onchange={handleFileChange}
			disabled={uploading}
		/>
		<Button onclick={() => fileInput?.click()} disabled={uploading}>
			{#if uploading}
				Uploading…
			{:else}
				<span class="mr-2">↑</span> Upload file
			{/if}
		</Button>
	</div>
</div>

<!-- Upload progress -->
{#if uploading}
	<Card.Root class="mb-6">
		<Card.Content class="pt-6">
			<p class="mb-2 text-sm font-medium">Uploading… {uploadProgress}%</p>
			<Progress value={uploadProgress} class="h-2" />
		</Card.Content>
	</Card.Root>
{/if}

<!-- Upload feedback -->
{#if uploadSuccess}
	<div class="mb-4 rounded-md border border-green-200 bg-green-50 px-4 py-3 text-sm text-green-800">
		✓ {uploadSuccess}
	</div>
{/if}
{#if uploadError}
	<div class="mb-4 rounded-md border border-destructive/20 bg-destructive/10 px-4 py-3 text-sm text-destructive">
		✗ {uploadError}
	</div>
{/if}

<Separator class="mb-6" />

<!-- File list -->
{#if loadingFiles}
	<div class="grid gap-3">
		{#each [1, 2, 3] as _}
			<div class="h-20 animate-pulse rounded-lg bg-muted"></div>
		{/each}
	</div>
{:else if fetchError}
	<Card.Root>
		<Card.Content class="py-12 text-center">
			<p class="text-muted-foreground">{fetchError}</p>
			<Button variant="outline" class="mt-4" onclick={fetchFiles}>Retry</Button>
		</Card.Content>
	</Card.Root>
{:else if files.length === 0}
	<Card.Root>
		<Card.Content class="py-16 text-center">
			<p class="text-4xl mb-4">📂</p>
			<p class="font-medium">No files yet</p>
			<p class="mt-1 text-sm text-muted-foreground">Upload your first file to get started</p>
			<Button class="mt-6" onclick={() => fileInput?.click()}>Upload file</Button>
		</Card.Content>
	</Card.Root>
{:else}
	<div class="grid gap-3">
		{#each files as file (file.id)}
			<Card.Root class="transition-shadow hover:shadow-md">
				<Card.Content class="flex items-center gap-4 py-4">
					<!-- File icon -->
					<div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-muted text-lg">
						{#if file.mime_type.startsWith('image/')}🖼️
						{:else if file.mime_type.startsWith('video/')}🎬
						{:else if file.mime_type.startsWith('audio/')}🎵
						{:else if file.mime_type.includes('pdf')}📄
						{:else if file.mime_type.includes('zip') || file.mime_type.includes('tar')}🗜️
						{:else}📎{/if}
					</div>

					<!-- File info -->
					<div class="min-w-0 flex-1">
						<p class="truncate font-medium">{file.name}</p>
						<div class="mt-1 flex items-center gap-2">
							<Badge variant="secondary" class="text-xs">{mimeCategory(file.mime_type)}</Badge>
							<span class="text-xs text-muted-foreground">{formatBytes(file.total_size)}</span>
							<span class="text-xs text-muted-foreground">·</span>
							<span class="text-xs text-muted-foreground">{formatDate(file.created_at)}</span>
						</div>
					</div>

					<!-- Actions -->
					<div class="flex items-center gap-2">
						<Button
							variant="outline"
							size="sm"
							disabled={downloadingId === file.id}
							onclick={() => handleDownload(file.id, file.name)}
						>
							{downloadingId === file.id ? '…' : '↓ Download'}
						</Button>
						<Button
							variant="ghost"
							size="sm"
							class="text-destructive hover:bg-destructive/10 hover:text-destructive"
							disabled={deletingId === file.id}
							onclick={() => handleDelete(file.id, file.name)}
						>
							{deletingId === file.id ? '…' : 'Delete'}
						</Button>
					</div>
				</Card.Content>
			</Card.Root>
		{/each}
	</div>
{/if}
