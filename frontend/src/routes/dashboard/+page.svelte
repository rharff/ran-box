<script lang="ts">
	import { fileStore } from '$lib/file-store.svelte';
	import { formatBytes, formatDate, formatDateFull, mimeIcon, mimeColor, mimeLabel } from '$lib/utils/format';
	import type { NaratelFile, ViewMode, SortField, SortDir } from '$lib/types';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Progress } from '$lib/components/ui/progress';

	// ── Load on mount ─────────────────────────────────────────────────────────
	$effect(() => { fileStore.load(); });

	// ── View / Sort state ────────────────────────────────────────────────────
	let viewMode = $state<ViewMode>('grid');
	let sortField = $state<SortField>('created_at');
	let sortDir = $state<SortDir>('desc');

	// ── Detail panel ─────────────────────────────────────────────────────────
	let detailFile = $state<NaratelFile | null>(null);

	// ── Delete confirmation ───────────────────────────────────────────────────
	let deleteTarget = $state<NaratelFile | null>(null);
	let deleting = $state(false);
	let downloadingId = $state<number | null>(null);

	// ── Drag & drop ──────────────────────────────────────────────────────────
	let dragOver = $state(false);

	// ── Derived: sorted files ─────────────────────────────────────────────────
	let sorted = $derived.by(() => {
		const list = [...fileStore.filteredFiles];
		list.sort((a, b) => {
			let cmp = 0;
			if (sortField === 'name') cmp = a.name.localeCompare(b.name);
			else if (sortField === 'total_size') cmp = a.total_size - b.total_size;
			else cmp = new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
			return sortDir === 'asc' ? cmp : -cmp;
		});
		return list;
	});

	function toggleSort(field: SortField) {
		if (sortField === field) sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		else { sortField = field; sortDir = 'asc'; }
	}

	function sortIcon(field: SortField) {
		if (sortField !== field) return '↕';
		return sortDir === 'asc' ? '↑' : '↓';
	}

	async function confirmDelete() {
		if (!deleteTarget) return;
		deleting = true;
		try {
			await fileStore.delete(deleteTarget.id);
			if (detailFile?.id === deleteTarget.id) detailFile = null;
		} finally {
			deleting = false;
			deleteTarget = null;
		}
	}

	async function handleDownload(file: NaratelFile) {
		downloadingId = file.id;
		try { await fileStore.download(file.id, file.name); }
		finally { downloadingId = null; }
	}

	function onDrop(e: DragEvent) {
		e.preventDefault();
		dragOver = false;
		const files = e.dataTransfer?.files;
		if (files?.length) fileStore.upload(files);
	}
</script>

<!-- ── Upload progress toast ─────────────────────────────────────────────── -->
{#if fileStore.uploading}
	<div class="fixed bottom-6 right-6 z-50 w-72 rounded-xl border bg-card shadow-xl p-4">
		<div class="flex items-center justify-between mb-2">
			<p class="text-sm font-medium">Uploading…</p>
			<span class="text-xs text-muted-foreground">{fileStore.uploadProgress}%</span>
		</div>
		{#if fileStore.uploadQueue[0]}
			<p class="text-xs text-muted-foreground truncate mb-2">{fileStore.uploadQueue[0]}</p>
		{/if}
		<Progress value={fileStore.uploadProgress} class="h-1.5" />
	</div>
{/if}

<!-- ── Upload errors ─────────────────────────────────────────────────────── -->
{#each fileStore.uploadErrors as err}
	<div class="mx-6 mt-4 rounded-md border border-destructive/30 bg-destructive/10 px-4 py-2 text-sm text-destructive">{err}</div>
{/each}

<!-- ── Main panel ─────────────────────────────────────────────────────────── -->
<div
	class="flex h-full"
	ondragover={(e) => { e.preventDefault(); dragOver = true; }}
	ondragleave={() => dragOver = false}
	ondrop={onDrop}
	role="region"
	aria-label="File area"
>
	<!-- Drop overlay -->
	{#if dragOver}
		<div class="pointer-events-none fixed inset-0 z-40 flex items-center justify-center bg-primary/10 backdrop-blur-sm">
			<div class="rounded-2xl border-2 border-dashed border-primary bg-card px-16 py-12 text-center shadow-2xl">
				<p class="text-4xl mb-3">📂</p>
				<p class="text-lg font-semibold">Drop files to upload</p>
			</div>
		</div>
	{/if}

	<div class="flex-1 overflow-y-auto px-6 py-6">

		<!-- ── Toolbar ────────────────────────────────────────────────── -->
		<div class="mb-5 flex items-center justify-between gap-4">
			<div class="flex items-center gap-2">
				<h1 class="text-lg font-semibold">My Files</h1>
				{#if !fileStore.loading}
					<span class="text-sm text-muted-foreground">({fileStore.filteredFiles.length})</span>
				{/if}
			</div>

			<div class="flex items-center gap-2">
				<!-- Bulk actions -->
				{#if fileStore.selectedIds.size > 0}
					<span class="text-sm text-muted-foreground">{fileStore.selectedIds.size} selected</span>
					<Button variant="outline" size="sm" onclick={() => fileStore.clearSelection()}>Clear</Button>
					<Button variant="destructive" size="sm"
						onclick={async () => { if (confirm(`Delete ${fileStore.selectedIds.size} files?`)) await fileStore.deleteSelected(); }}>
						Delete selected
					</Button>
				{/if}

				<!-- Sort -->
				<DropdownMenu.Root>
					<DropdownMenu.Trigger>
						<Button variant="outline" size="sm" class="gap-1.5">
							<svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M3 6h18M7 12h10M11 18h2"/>
							</svg>
							Sort
						</Button>
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end">
						<DropdownMenu.Item onclick={() => toggleSort('name')}>Name {sortIcon('name')}</DropdownMenu.Item>
						<DropdownMenu.Item onclick={() => toggleSort('total_size')}>Size {sortIcon('total_size')}</DropdownMenu.Item>
						<DropdownMenu.Item onclick={() => toggleSort('created_at')}>Date {sortIcon('created_at')}</DropdownMenu.Item>
					</DropdownMenu.Content>
				</DropdownMenu.Root>

				<!-- View toggle -->
				<div class="flex rounded-md border overflow-hidden">
					<button
						onclick={() => viewMode = 'grid'}
						class={`px-2.5 py-1.5 text-sm transition-colors ${viewMode === 'grid' ? 'bg-primary text-primary-foreground' : 'hover:bg-accent'}`}
						title="Grid view"
					>
						<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/>
							<rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/>
						</svg>
					</button>
					<button
						onclick={() => viewMode = 'list'}
						class={`px-2.5 py-1.5 text-sm transition-colors ${viewMode === 'list' ? 'bg-primary text-primary-foreground' : 'hover:bg-accent'}`}
						title="List view"
					>
						<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M4 6h16M4 12h16M4 18h16"/>
						</svg>
					</button>
				</div>
			</div>
		</div>

		<!-- ── Loading skeletons ──────────────────────────────────────── -->
		{#if fileStore.loading}
			{#if viewMode === 'grid'}
				<div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
					{#each Array(10) as _}
						<Skeleton class="aspect-square rounded-xl" />
					{/each}
				</div>
			{:else}
				<div class="space-y-2">
					{#each Array(8) as _}
						<Skeleton class="h-14 rounded-lg" />
					{/each}
				</div>
			{/if}

		<!-- ── Error ──────────────────────────────────────────────────── -->
		{:else if fileStore.error}
			<div class="flex flex-col items-center justify-center py-32 text-center">
				<p class="text-muted-foreground">{fileStore.error}</p>
				<Button variant="outline" class="mt-4" onclick={() => fileStore.load()}>Retry</Button>
			</div>

		<!-- ── Empty ──────────────────────────────────────────────────── -->
		{:else if sorted.length === 0}
			<div class="flex flex-col items-center justify-center py-32 text-center">
				<p class="text-5xl mb-4">📂</p>
				<p class="text-lg font-medium">
					{fileStore.searchQuery ? 'No files match your search' : 'No files yet'}
				</p>
				<p class="mt-1 text-sm text-muted-foreground">
					{fileStore.searchQuery ? 'Try a different search term' : 'Drag and drop files here, or use the upload button'}
				</p>
				{#if !fileStore.searchQuery}
					<Button class="mt-6" onclick={() => document.getElementById('sidebar-upload')?.click()}>
						Upload your first file
					</Button>
				{/if}
			</div>

		<!-- ── Grid view ──────────────────────────────────────────────── -->
		{:else if viewMode === 'grid'}
			<div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
				{#each sorted as file (file.id)}
					{@const selected = fileStore.selectedIds.has(file.id)}
					<div
						class={`group relative flex flex-col rounded-xl border bg-card p-4 cursor-pointer transition-all hover:shadow-md hover:border-primary/50 ${selected ? 'border-primary ring-1 ring-primary bg-primary/5' : ''}`}
						onclick={() => detailFile = file}
						role="button"
						tabindex="0"
						onkeydown={(e) => e.key === 'Enter' && (detailFile = file)}
					>
						<!-- Checkbox -->
						<div class="absolute top-2 left-2 opacity-0 group-hover:opacity-100 transition-opacity" class:opacity-100={selected}>
							<input type="checkbox" class="h-4 w-4 rounded accent-primary"
								checked={selected}
								onclick={(e) => { e.stopPropagation(); fileStore.toggleSelect(file.id); }}
							/>
						</div>

						<!-- More menu -->
						<div class="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity">
							<DropdownMenu.Root>
								<DropdownMenu.Trigger onclick={(e) => e.stopPropagation()}
									class="rounded p-0.5 hover:bg-muted">
									<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24">
										<circle cx="5" cy="12" r="1.5"/><circle cx="12" cy="12" r="1.5"/><circle cx="19" cy="12" r="1.5"/>
									</svg>
								</DropdownMenu.Trigger>
								<DropdownMenu.Content align="end">
									<DropdownMenu.Item onclick={(e) => { e.stopPropagation(); handleDownload(file); }}>Download</DropdownMenu.Item>
									<DropdownMenu.Separator />
									<DropdownMenu.Item class="text-destructive focus:text-destructive"
										onclick={(e) => { e.stopPropagation(); deleteTarget = file; }}>Delete</DropdownMenu.Item>
								</DropdownMenu.Content>
							</DropdownMenu.Root>
						</div>

						<!-- Icon -->
						<div class={`mx-auto mb-3 mt-2 flex h-14 w-14 items-center justify-center rounded-2xl text-2xl ${mimeColor(file.mime_type)}`}>
							{mimeIcon(file.mime_type)}
						</div>

						<!-- Name -->
						<p class="truncate text-center text-sm font-medium">{file.name}</p>
						<p class="mt-1 text-center text-xs text-muted-foreground">{formatBytes(file.total_size)}</p>
						<p class="mt-0.5 text-center text-xs text-muted-foreground">{formatDate(file.created_at)}</p>
					</div>
				{/each}
			</div>

		<!-- ── List view ──────────────────────────────────────────────── -->
		{:else}
			<div class="rounded-lg border overflow-hidden">
				<!-- Header row -->
				<div class="flex items-center gap-4 border-b bg-muted/40 px-4 py-2 text-xs font-medium text-muted-foreground">
					<input type="checkbox" class="h-4 w-4 rounded accent-primary flex-shrink-0"
						checked={fileStore.selectedIds.size === sorted.length && sorted.length > 0}
						onchange={() => fileStore.selectedIds.size === sorted.length ? fileStore.clearSelection() : fileStore.selectAll()}
					/>
					<button class="flex-1 text-left hover:text-foreground" onclick={() => toggleSort('name')}>Name {sortIcon('name')}</button>
					<button class="w-24 text-right hover:text-foreground" onclick={() => toggleSort('total_size')}>Size {sortIcon('total_size')}</button>
					<button class="w-32 text-right hover:text-foreground" onclick={() => toggleSort('created_at')}>Modified {sortIcon('created_at')}</button>
					<div class="w-24 text-right">Type</div>
					<div class="w-20"></div>
				</div>

				{#each sorted as file (file.id)}
					{@const selected = fileStore.selectedIds.has(file.id)}
					<div
						class={`group flex items-center gap-4 border-b last:border-0 px-4 py-3 cursor-pointer hover:bg-accent/50 transition-colors ${selected ? 'bg-primary/5' : ''}`}
						onclick={() => detailFile = file}
						role="button"
						tabindex="0"
						onkeydown={(e) => e.key === 'Enter' && (detailFile = file)}
					>
						<input type="checkbox" class="h-4 w-4 rounded accent-primary flex-shrink-0"
							checked={selected}
							onclick={(e) => { e.stopPropagation(); fileStore.toggleSelect(file.id); }}
						/>
						<div class={`flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg text-base ${mimeColor(file.mime_type)}`}>
							{mimeIcon(file.mime_type)}
						</div>
						<span class="flex-1 min-w-0 truncate text-sm font-medium">{file.name}</span>
						<span class="w-24 text-right text-sm text-muted-foreground">{formatBytes(file.total_size)}</span>
						<span class="w-32 text-right text-xs text-muted-foreground">{formatDate(file.created_at)}</span>
						<div class="w-24 text-right">
							<Badge variant="secondary" class="text-xs">{mimeLabel(file.mime_type)}</Badge>
						</div>
						<div class="w-20 flex justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
							<button class="rounded p-1 hover:bg-muted" title="Download"
								onclick={(e) => { e.stopPropagation(); handleDownload(file); }}
								disabled={downloadingId === file.id}>
								<svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2M7 10l5 5 5-5M12 15V3"/>
								</svg>
							</button>
							<button class="rounded p-1 hover:bg-destructive/10 text-destructive" title="Delete"
								onclick={(e) => { e.stopPropagation(); deleteTarget = file; }}>
								<svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6M9 7V4h6v3M3 7h18"/>
								</svg>
							</button>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>

	<!-- ── Detail panel ───────────────────────────────────────────────────── -->
	{#if detailFile}
		{@const f = detailFile}
		<aside class="w-72 flex-shrink-0 border-l bg-card overflow-y-auto">
			<div class="flex items-center justify-between border-b px-4 py-3">
				<p class="text-sm font-semibold">File details</p>
				<button onclick={() => detailFile = null} aria-label="Close details" class="rounded p-1 hover:bg-muted text-muted-foreground">
					<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
					</svg>
				</button>
			</div>
			<div class="p-4">
				<!-- Preview area -->
				<div class={`mx-auto mb-4 flex h-28 w-28 items-center justify-center rounded-2xl text-5xl ${mimeColor(f.mime_type)}`}>
					{mimeIcon(f.mime_type)}
				</div>
				<p class="break-all text-sm font-semibold text-center mb-4">{f.name}</p>

				<div class="space-y-3 text-sm">
					<div class="flex justify-between">
						<span class="text-muted-foreground">Type</span>
						<Badge variant="secondary">{mimeLabel(f.mime_type)}</Badge>
					</div>
					<div class="flex justify-between">
						<span class="text-muted-foreground">Size</span>
						<span class="font-medium">{formatBytes(f.total_size)}</span>
					</div>
					<div class="flex justify-between">
						<span class="text-muted-foreground">Uploaded</span>
						<span class="text-right text-xs">{formatDateFull(f.created_at)}</span>
					</div>
					<div class="flex justify-between">
						<span class="text-muted-foreground">MIME</span>
						<span class="text-right text-xs font-mono text-muted-foreground">{f.mime_type}</span>
					</div>
				</div>

				<div class="mt-6 flex flex-col gap-2">
					<Button class="w-full gap-2" onclick={() => handleDownload(f)} disabled={downloadingId === f.id}>
						<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2M7 10l5 5 5-5M12 15V3"/>
						</svg>
						{downloadingId === f.id ? 'Downloading…' : 'Download'}
					</Button>
					<Button variant="outline" class="w-full gap-2 text-destructive hover:bg-destructive/10 hover:text-destructive border-destructive/30"
						onclick={() => deleteTarget = f}>
						<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6M9 7V4h6v3M3 7h18"/>
						</svg>
						Delete
					</Button>
				</div>
			</div>
		</aside>
	{/if}
</div>

<!-- ── Delete confirmation dialog ─────────────────────────────────────────── -->
<Dialog.Root open={!!deleteTarget} onOpenChange={(o) => { if (!o) deleteTarget = null; }}>
	<Dialog.Content class="max-w-sm">
		<Dialog.Header>
			<Dialog.Title>Delete file?</Dialog.Title>
			<Dialog.Description>
				<strong class="break-all">{deleteTarget?.name}</strong> will be permanently deleted.
				This cannot be undone.
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer class="gap-2">
			<Button variant="outline" onclick={() => deleteTarget = null} disabled={deleting}>Cancel</Button>
			<Button variant="destructive" onclick={confirmDelete} disabled={deleting}>
				{deleting ? 'Deleting…' : 'Delete'}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
