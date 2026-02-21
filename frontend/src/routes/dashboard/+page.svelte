<script lang="ts">
	import { page } from '$app/stores';
	import { fileStore } from '$lib/file-store.svelte';
	import { formatBytes, formatDate, formatDateFull, mimeIcon, mimeColor, mimeLabel } from '$lib/utils/format';
	import { getFilePreviewBlob, createShareLink, getShareLinks, deleteShareLink, shareDownloadUrl, listAllFolders } from '$lib/api';
	import type { NaratelFile, Folder, ShareLink, ViewMode, SortField, SortDir } from '$lib/types';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { Input } from '$lib/components/ui/input';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Progress } from '$lib/components/ui/progress';

	// ── Load folder from URL (reacts to browser back/forward) ─────────────
	let urlFolderId = $derived.by(() => {
		const param = $page.url.searchParams.get('folder');
		return param ? Number(param) : null;
	});

	$effect(() => {
		fileStore.loadFolder(urlFolderId);
	});

	// ── View / Sort state ────────────────────────────────────────────────────
	let viewMode = $state<ViewMode>('grid');
	let sortField = $state<SortField>('created_at');
	let sortDir = $state<SortDir>('desc');

	// ── Detail panel ─────────────────────────────────────────────────────────
	let detailFile = $state<NaratelFile | null>(null);

	// ── Delete confirmation ───────────────────────────────────────────────────
	let deleteTarget = $state<{ type: 'file' | 'folder'; item: NaratelFile | Folder } | null>(null);
	let deleting = $state(false);
	let downloadingId = $state<number | null>(null);

	// ── Rename dialog ────────────────────────────────────────────────────────
	let renameTarget = $state<{ type: 'file' | 'folder'; item: NaratelFile | Folder } | null>(null);
	let renameValue = $state('');
	let renaming = $state(false);

	// ── New folder dialog ────────────────────────────────────────────────────
	let showNewFolder = $state(false);
	let newFolderName = $state('');
	let creatingFolder = $state(false);

	// ── Move dialog ──────────────────────────────────────────────────────────
	let moveTarget = $state<{ type: 'file' | 'folder'; item: NaratelFile | Folder } | null>(null);
	let allFolders = $state<Folder[]>([]);
	let moveDestination = $state<number | null>(null);
	let moving = $state(false);
	let expandedFolders = $state<Set<number | null>>(new Set([null]));
	let loadingFolders = $state(false);

	interface FolderNode {
		id: number | null;
		name: string;
		children: FolderNode[];
		depth: number;
	}

	let folderTree = $derived.by(() => {
		const excludeId = moveTarget?.type === 'folder' ? moveTarget.item.id : -1;
		const folders = allFolders.filter(f => f.id !== excludeId);

		// Collect all descendants of excluded folder so we don't show them
		const excludeIds = new Set<number>();
		if (excludeId !== -1) {
			excludeIds.add(excludeId);
			let changed = true;
			while (changed) {
				changed = false;
				for (const f of allFolders) {
					if (!excludeIds.has(f.id) && f.parent_id !== null && excludeIds.has(f.parent_id)) {
						excludeIds.add(f.id);
						changed = true;
					}
				}
			}
		}
		const validFolders = allFolders.filter(f => !excludeIds.has(f.id));

		function buildChildren(parentId: number | null, depth: number): FolderNode[] {
			return validFolders
				.filter(f => f.parent_id === parentId)
				.sort((a, b) => a.name.localeCompare(b.name))
				.map(f => ({
					id: f.id,
					name: f.name,
					children: buildChildren(f.id, depth + 1),
					depth,
				}));
		}

		return { id: null as number | null, name: 'All Files', children: buildChildren(null, 1), depth: 0 } as FolderNode;
	});

	function toggleExpand(id: number | null) {
		const next = new Set(expandedFolders);
		if (next.has(id)) next.delete(id);
		else next.add(id);
		expandedFolders = next;
	}

	function flattenTree(node: FolderNode): FolderNode[] {
		const result: FolderNode[] = [node];
		if (expandedFolders.has(node.id)) {
			for (const child of node.children) {
				result.push(...flattenTree(child));
			}
		}
		return result;
	}

	let flatFolderList = $derived(flattenTree(folderTree));

	// ── Preview dialog ───────────────────────────────────────────────────────
	let previewFile = $state<NaratelFile | null>(null);
	let previewUrl = $state('');
	let previewText = $state('');
	let previewLoading = $state(false);

	// ── Share dialog ─────────────────────────────────────────────────────────
	let shareFile = $state<NaratelFile | null>(null);
	let shareLinks = $state<ShareLink[]>([]);
	let shareLoading = $state(false);
	let shareCopied = $state(false);

	// ── Drag & drop ──────────────────────────────────────────────────────────
	let dragOver = $state(false);

	// ── Derived: sorted items ─────────────────────────────────────────────────
	let sortedFiles = $derived.by(() => {
		const list = [...fileStore.displayFiles];
		list.sort((a, b) => {
			let cmp = 0;
			if (sortField === 'name') cmp = a.name.localeCompare(b.name);
			else if (sortField === 'total_size') cmp = a.total_size - b.total_size;
			else cmp = new Date(a.created_at).getTime() - new Date(b.created_at).getTime();
			return sortDir === 'asc' ? cmp : -cmp;
		});
		return list;
	});

	let sortedFolders = $derived.by(() => {
		const list = [...fileStore.displayFolders];
		list.sort((a, b) => a.name.localeCompare(b.name));
		return list;
	});

	let isEmpty = $derived(sortedFiles.length === 0 && sortedFolders.length === 0);

	let pageTitle = $derived(
		fileStore.isSearching
			? 'Search Results'
			: fileStore.breadcrumb.length > 0
				? fileStore.breadcrumb[fileStore.breadcrumb.length - 1].name
				: 'All Files'
	);

	function toggleSort(field: SortField) {
		if (sortField === field) sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		else { sortField = field; sortDir = 'asc'; }
	}

	function sortIcon(field: SortField) {
		if (sortField !== field) return '↕';
		return sortDir === 'asc' ? '↑' : '↓';
	}

	// ── Delete action ────────────────────────────────────────────────────────
	async function confirmDelete() {
		if (!deleteTarget) return;
		deleting = true;
		try {
			if (deleteTarget.type === 'file') {
				await fileStore.deleteFileItem((deleteTarget.item as NaratelFile).id);
				if (detailFile?.id === (deleteTarget.item as NaratelFile).id) detailFile = null;
			} else {
				await fileStore.deleteFolderItem((deleteTarget.item as Folder).id);
			}
		} finally {
			deleting = false;
			deleteTarget = null;
		}
	}

	// ── Rename action ────────────────────────────────────────────────────────
	function openRename(type: 'file' | 'folder', item: NaratelFile | Folder) {
		renameTarget = { type, item };
		renameValue = item.name;
	}

	async function confirmRename() {
		if (!renameTarget || !renameValue.trim()) return;
		renaming = true;
		try {
			if (renameTarget.type === 'file') {
				await fileStore.renameFileItem((renameTarget.item as NaratelFile).id, renameValue.trim());
			} else {
				await fileStore.renameFolderItem((renameTarget.item as Folder).id, renameValue.trim());
			}
		} finally {
			renaming = false;
			renameTarget = null;
		}
	}

	// ── New folder action ────────────────────────────────────────────────────
	async function confirmNewFolder() {
		if (!newFolderName.trim()) return;
		creatingFolder = true;
		try {
			await fileStore.createFolder(newFolderName.trim());
		} finally {
			creatingFolder = false;
			showNewFolder = false;
			newFolderName = '';
		}
	}

	// ── Move action ──────────────────────────────────────────────────────────
	async function openMove(type: 'file' | 'folder', item: NaratelFile | Folder) {
		moveTarget = { type, item };
		moveDestination = null;
		expandedFolders = new Set([null]);
		loadingFolders = true;
		try {
			allFolders = await listAllFolders();
		} catch { allFolders = []; }
		finally { loadingFolders = false; }
	}

	async function confirmMove() {
		if (!moveTarget) return;
		moving = true;
		try {
			if (moveTarget.type === 'file') {
				await fileStore.moveFileItem((moveTarget.item as NaratelFile).id, moveDestination);
			} else {
				await fileStore.moveFolderItem((moveTarget.item as Folder).id, moveDestination);
			}
		} finally {
			moving = false;
			moveTarget = null;
		}
	}

	// ── Preview action ───────────────────────────────────────────────────────
	function canPreview(mime: string): boolean {
		return mime.startsWith('image/') || mime === 'application/pdf' || mime.startsWith('text/');
	}

	async function openPreview(file: NaratelFile) {
		previewFile = file;
		previewUrl = '';
		previewText = '';
		previewLoading = true;
		try {
			const { blob, mimeType } = await getFilePreviewBlob(file.id);
			if (mimeType.startsWith('text/') || file.mime_type.startsWith('text/')) {
				previewText = await blob.text();
			} else {
				previewUrl = URL.createObjectURL(blob);
			}
		} catch {
			previewText = 'Failed to load preview.';
		} finally {
			previewLoading = false;
		}
	}

	function closePreview() {
		if (previewUrl) URL.revokeObjectURL(previewUrl);
		previewFile = null;
		previewUrl = '';
		previewText = '';
	}

	// ── Share action ─────────────────────────────────────────────────────────
	async function openShare(file: NaratelFile) {
		shareFile = file;
		shareLinks = [];
		shareLoading = true;
		shareCopied = false;
		try {
			shareLinks = await getShareLinks(file.id);
		} catch { /* ignore */ }
		finally { shareLoading = false; }
	}

	async function handleCreateShareLink() {
		if (!shareFile) return;
		shareLoading = true;
		try {
			const link = await createShareLink(shareFile.id);
			shareLinks = [link, ...shareLinks];
		} finally { shareLoading = false; }
	}

	async function handleDeleteShareLink(linkId: number) {
		await deleteShareLink(linkId);
		shareLinks = shareLinks.filter(l => l.id !== linkId);
	}

	function copyShareUrl(token: string) {
		const url = `${window.location.origin}${shareDownloadUrl(token)}`;
		navigator.clipboard.writeText(url);
		shareCopied = true;
		setTimeout(() => shareCopied = false, 2000);
	}

	// ── Download ─────────────────────────────────────────────────────────────
	async function handleDownload(file: NaratelFile) {
		downloadingId = file.id;
		try { await fileStore.download(file.id, file.name); }
		finally { downloadingId = null; }
	}

	// ── Drag & drop ──────────────────────────────────────────────────────────
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
				{#if fileStore.currentFolderId}
					<p class="text-sm text-muted-foreground mt-1">Files will be uploaded to the current folder</p>
				{/if}
			</div>
		</div>
	{/if}

	<div class="flex-1 overflow-y-auto px-6 py-6">

		<!-- ── Breadcrumb ──────────────────────────────────────────────── -->
		<nav class="mb-4 flex items-center gap-1 text-sm text-muted-foreground" aria-label="Breadcrumb">
			<button
				onclick={() => fileStore.navigateToFolder(null)}
				class="hover:text-foreground font-medium transition-colors"
				class:text-foreground={fileStore.breadcrumb.length === 0 && !fileStore.isSearching}
			>
				All Files
			</button>
			{#if fileStore.isSearching}
				<span class="mx-1">/</span>
				<span class="text-foreground font-medium">Search results</span>
			{:else}
				{#each fileStore.breadcrumb as crumb, i}
					<span class="mx-1">/</span>
					{#if i === fileStore.breadcrumb.length - 1}
						<span class="text-foreground font-medium">{crumb.name}</span>
					{:else}
						<button
							onclick={() => fileStore.navigateToFolder(crumb.id)}
							class="hover:text-foreground transition-colors"
						>{crumb.name}</button>
					{/if}
				{/each}
			{/if}
		</nav>

		<!-- ── Toolbar ────────────────────────────────────────────────── -->
		<div class="mb-5 flex items-center justify-between gap-4 flex-wrap">
			<div class="flex items-center gap-2">
				<h1 class="text-lg font-semibold">
					{pageTitle}
				</h1>
				{#if !fileStore.loading}
					<span class="text-sm text-muted-foreground">
						({sortedFolders.length + sortedFiles.length})
					</span>
				{/if}
			</div>

			<div class="flex items-center gap-2 flex-wrap">
				<!-- Bulk actions -->
				{#if fileStore.hasSelection}
					<span class="text-sm text-muted-foreground">{fileStore.selectionCount} selected</span>
					<Button variant="outline" size="sm" onclick={() => fileStore.clearSelection()}>Clear</Button>
					<Button variant="destructive" size="sm"
						onclick={async () => { if (confirm(`Delete ${fileStore.selectionCount} items?`)) await fileStore.deleteSelected(); }}>
						Delete selected
					</Button>
				{/if}

				<!-- New folder -->
				<Button variant="outline" size="sm" class="gap-1.5" onclick={() => { showNewFolder = true; newFolderName = ''; }}>
					<svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
						<path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
					</svg>
					New folder
				</Button>

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
						aria-label="Grid view"
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
						aria-label="List view"
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
				<Button variant="outline" class="mt-4" onclick={() => fileStore.loadFolder(fileStore.currentFolderId)}>Retry</Button>
			</div>

		<!-- ── Empty ──────────────────────────────────────────────────── -->
		{:else if isEmpty}
			<div class="flex flex-col items-center justify-center py-32 text-center">
				<p class="text-5xl mb-4">📂</p>
				<p class="text-lg font-medium">
					{fileStore.isSearching ? 'No files match your search' : 'This folder is empty'}
				</p>
				<p class="mt-1 text-sm text-muted-foreground">
					{fileStore.isSearching ? 'Try a different search term' : 'Drag and drop files here, or use the upload button'}
				</p>
				{#if !fileStore.isSearching}
					<div class="mt-6 flex gap-3">
						<Button onclick={() => document.getElementById('sidebar-upload')?.click()}>
							Upload files
						</Button>
						<Button variant="outline" onclick={() => { showNewFolder = true; newFolderName = ''; }}>
							New folder
						</Button>
					</div>
				{/if}
			</div>

		<!-- ── Grid view ──────────────────────────────────────────────── -->
		{:else if viewMode === 'grid'}
			<div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
				<!-- Folders -->
				{#each sortedFolders as folder (folder.id)}
					{@const selected = fileStore.selectedFolderIds.has(folder.id)}
					<div
						class={`group relative flex flex-col rounded-xl border bg-card p-4 cursor-pointer transition-all hover:shadow-md hover:border-primary/50 ${selected ? 'border-primary ring-1 ring-primary bg-primary/5' : ''}`}
						ondblclick={() => fileStore.navigateToFolder(folder.id)}
						role="button"
						tabindex="0"
						onkeydown={(e) => e.key === 'Enter' && fileStore.navigateToFolder(folder.id)}
					>
						<!-- Checkbox -->
						<div class="absolute top-2 left-2 opacity-0 group-hover:opacity-100 transition-opacity" class:opacity-100={selected}>
							<input type="checkbox" class="h-4 w-4 rounded accent-primary"
								checked={selected}
								onclick={(e) => { e.stopPropagation(); fileStore.toggleFolderSelect(folder.id); }}
							/>
						</div>

						<!-- More menu -->
						<div class="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity">
							<DropdownMenu.Root>
								<DropdownMenu.Trigger onclick={(e) => e.stopPropagation()}
									class="rounded p-0.5 hover:bg-muted" aria-label="More options">
									<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24">
										<circle cx="5" cy="12" r="1.5"/><circle cx="12" cy="12" r="1.5"/><circle cx="19" cy="12" r="1.5"/>
									</svg>
								</DropdownMenu.Trigger>
								<DropdownMenu.Content align="end">
									<DropdownMenu.Item onclick={(e) => { e.stopPropagation(); fileStore.navigateToFolder(folder.id); }}>Open</DropdownMenu.Item>
									<DropdownMenu.Item onclick={(e) => { e.stopPropagation(); openRename('folder', folder); }}>Rename</DropdownMenu.Item>
									<DropdownMenu.Item onclick={(e) => { e.stopPropagation(); openMove('folder', folder); }}>Move</DropdownMenu.Item>
									<DropdownMenu.Separator />
									<DropdownMenu.Item class="text-destructive focus:text-destructive"
										onclick={(e) => { e.stopPropagation(); deleteTarget = { type: 'folder', item: folder }; }}>Delete</DropdownMenu.Item>
								</DropdownMenu.Content>
							</DropdownMenu.Root>
						</div>

						<!-- Folder icon -->
						<div class="mx-auto mb-3 mt-2 flex h-14 w-14 items-center justify-center rounded-2xl text-2xl bg-blue-100 text-blue-600">
							📁
						</div>

						<p class="truncate text-center text-sm font-medium">{folder.name}</p>
						<p class="mt-0.5 text-center text-xs text-muted-foreground">Folder</p>
					</div>
				{/each}

				<!-- Files -->
				{#each sortedFiles as file (file.id)}
					{@const selected = fileStore.selectedFileIds.has(file.id)}
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
								onclick={(e) => { e.stopPropagation(); fileStore.toggleFileSelect(file.id); }}
							/>
						</div>

						<!-- More menu -->
						<div class="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity">
							<DropdownMenu.Root>
								<DropdownMenu.Trigger onclick={(e) => e.stopPropagation()}
									class="rounded p-0.5 hover:bg-muted" aria-label="More options">
									<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24">
										<circle cx="5" cy="12" r="1.5"/><circle cx="12" cy="12" r="1.5"/><circle cx="19" cy="12" r="1.5"/>
									</svg>
								</DropdownMenu.Trigger>
								<DropdownMenu.Content align="end">
									{#if canPreview(file.mime_type)}
										<DropdownMenu.Item onclick={(e) => { e.stopPropagation(); openPreview(file); }}>Preview</DropdownMenu.Item>
									{/if}
									<DropdownMenu.Item onclick={(e) => { e.stopPropagation(); handleDownload(file); }}>Download</DropdownMenu.Item>
									<DropdownMenu.Item onclick={(e) => { e.stopPropagation(); openRename('file', file); }}>Rename</DropdownMenu.Item>
									<DropdownMenu.Item onclick={(e) => { e.stopPropagation(); openMove('file', file); }}>Move</DropdownMenu.Item>
									<DropdownMenu.Item onclick={(e) => { e.stopPropagation(); openShare(file); }}>Share</DropdownMenu.Item>
									<DropdownMenu.Separator />
									<DropdownMenu.Item class="text-destructive focus:text-destructive"
										onclick={(e) => { e.stopPropagation(); deleteTarget = { type: 'file', item: file }; }}>Delete</DropdownMenu.Item>
								</DropdownMenu.Content>
							</DropdownMenu.Root>
						</div>

						<!-- Icon -->
						<div class={`mx-auto mb-3 mt-2 flex h-14 w-14 items-center justify-center rounded-2xl text-2xl ${mimeColor(file.mime_type)}`}>
							{mimeIcon(file.mime_type)}
						</div>

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
						checked={fileStore.selectionCount === (sortedFiles.length + sortedFolders.length) && (sortedFiles.length + sortedFolders.length) > 0}
						onchange={() => fileStore.selectionCount === (sortedFiles.length + sortedFolders.length) ? fileStore.clearSelection() : fileStore.selectAll()}
					/>
					<button class="flex-1 text-left hover:text-foreground" onclick={() => toggleSort('name')}>Name {sortIcon('name')}</button>
					<button class="w-24 text-right hover:text-foreground" onclick={() => toggleSort('total_size')}>Size {sortIcon('total_size')}</button>
					<button class="w-32 text-right hover:text-foreground" onclick={() => toggleSort('created_at')}>Modified {sortIcon('created_at')}</button>
					<div class="w-24 text-right">Type</div>
					<div class="w-28"></div>
				</div>

				<!-- Folders -->
				{#each sortedFolders as folder (folder.id)}
					{@const selected = fileStore.selectedFolderIds.has(folder.id)}
					<div
						class={`group flex items-center gap-4 border-b last:border-0 px-4 py-3 cursor-pointer hover:bg-accent/50 transition-colors ${selected ? 'bg-primary/5' : ''}`}
						ondblclick={() => fileStore.navigateToFolder(folder.id)}
						role="button"
						tabindex="0"
						onkeydown={(e) => e.key === 'Enter' && fileStore.navigateToFolder(folder.id)}
					>
						<input type="checkbox" class="h-4 w-4 rounded accent-primary flex-shrink-0"
							checked={selected}
							onclick={(e) => { e.stopPropagation(); fileStore.toggleFolderSelect(folder.id); }}
						/>
						<div class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg text-base bg-blue-100 text-blue-600">
							📁
						</div>
						<span class="flex-1 min-w-0 truncate text-sm font-medium">{folder.name}</span>
						<span class="w-24 text-right text-sm text-muted-foreground">—</span>
						<span class="w-32 text-right text-xs text-muted-foreground">{formatDate(folder.created_at)}</span>
						<div class="w-24 text-right">
							<Badge variant="secondary" class="text-xs">Folder</Badge>
						</div>
						<div class="w-28 flex justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
							<button class="rounded p-1 hover:bg-muted" title="Open"
								onclick={(e) => { e.stopPropagation(); fileStore.navigateToFolder(folder.id); }}>
								<svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" />
								</svg>
							</button>
							<button class="rounded p-1 hover:bg-muted" title="Rename"
								onclick={(e) => { e.stopPropagation(); openRename('folder', folder); }}>
								<svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/>
								</svg>
							</button>
							<button class="rounded p-1 hover:bg-destructive/10 text-destructive" title="Delete"
								onclick={(e) => { e.stopPropagation(); deleteTarget = { type: 'folder', item: folder }; }}>
								<svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6M9 7V4h6v3M3 7h18"/>
								</svg>
							</button>
						</div>
					</div>
				{/each}

				<!-- Files -->
				{#each sortedFiles as file (file.id)}
					{@const selected = fileStore.selectedFileIds.has(file.id)}
					<div
						class={`group flex items-center gap-4 border-b last:border-0 px-4 py-3 cursor-pointer hover:bg-accent/50 transition-colors ${selected ? 'bg-primary/5' : ''}`}
						onclick={() => detailFile = file}
						role="button"
						tabindex="0"
						onkeydown={(e) => e.key === 'Enter' && (detailFile = file)}
					>
						<input type="checkbox" class="h-4 w-4 rounded accent-primary flex-shrink-0"
							checked={selected}
							onclick={(e) => { e.stopPropagation(); fileStore.toggleFileSelect(file.id); }}
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
						<div class="w-28 flex justify-end gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
							{#if canPreview(file.mime_type)}
								<button class="rounded p-1 hover:bg-muted" title="Preview"
									onclick={(e) => { e.stopPropagation(); openPreview(file); }}>
									<svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
										<path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/><path stroke-linecap="round" stroke-linejoin="round" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"/>
									</svg>
								</button>
							{/if}
							<button class="rounded p-1 hover:bg-muted" title="Download"
								onclick={(e) => { e.stopPropagation(); handleDownload(file); }}
								disabled={downloadingId === file.id}>
								<svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2M7 10l5 5 5-5M12 15V3"/>
								</svg>
							</button>
							<button class="rounded p-1 hover:bg-muted" title="Share"
								onclick={(e) => { e.stopPropagation(); openShare(file); }}>
								<svg xmlns="http://www.w3.org/2000/svg" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M8.684 13.342C8.886 12.938 9 12.482 9 12c0-.482-.114-.938-.316-1.342m0 2.684a3 3 0 110-2.684m0 2.684l6.632 3.316m-6.632-6l6.632-3.316m0 0a3 3 0 105.367-2.684 3 3 0 00-5.367 2.684zm0 9.316a3 3 0 105.368 2.684 3 3 0 00-5.368-2.684z"/>
								</svg>
							</button>
							<button class="rounded p-1 hover:bg-destructive/10 text-destructive" title="Delete"
								onclick={(e) => { e.stopPropagation(); deleteTarget = { type: 'file', item: file }; }}>
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
		<aside class="w-72 flex-shrink-0 border-l bg-card overflow-y-auto hidden lg:block">
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
						<span class="text-muted-foreground">Created</span>
						<span class="text-right text-xs">{formatDateFull(f.created_at)}</span>
					</div>
					<div class="flex justify-between">
						<span class="text-muted-foreground">Modified</span>
						<span class="text-right text-xs">{formatDateFull(f.updated_at)}</span>
					</div>
					<div class="flex justify-between">
						<span class="text-muted-foreground">MIME</span>
						<span class="text-right text-xs font-mono text-muted-foreground">{f.mime_type}</span>
					</div>
				</div>

				<div class="mt-6 flex flex-col gap-2">
					{#if canPreview(f.mime_type)}
						<Button variant="outline" class="w-full gap-2" onclick={() => openPreview(f)}>
							<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
								<path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/><path stroke-linecap="round" stroke-linejoin="round" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"/>
							</svg>
							Preview
						</Button>
					{/if}
					<Button class="w-full gap-2" onclick={() => handleDownload(f)} disabled={downloadingId === f.id}>
						<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2M7 10l5 5 5-5M12 15V3"/>
						</svg>
						{downloadingId === f.id ? 'Downloading…' : 'Download'}
					</Button>
					<Button variant="outline" class="w-full gap-2" onclick={() => openRename('file', f)}>
						<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/>
						</svg>
						Rename
					</Button>
					<Button variant="outline" class="w-full gap-2" onclick={() => openMove('file', f)}>
						<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M7 16V4m0 0L3 8m4-4l4 4m6 0v12m0 0l4-4m-4 4l-4-4"/>
						</svg>
						Move
					</Button>
					<Button variant="outline" class="w-full gap-2" onclick={() => openShare(f)}>
						<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M8.684 13.342C8.886 12.938 9 12.482 9 12c0-.482-.114-.938-.316-1.342m0 2.684a3 3 0 110-2.684m0 2.684l6.632 3.316m-6.632-6l6.632-3.316m0 0a3 3 0 105.367-2.684 3 3 0 00-5.367 2.684zm0 9.316a3 3 0 105.368 2.684 3 3 0 00-5.368-2.684z"/>
						</svg>
						Share
					</Button>
					<Button variant="outline" class="w-full gap-2 text-destructive hover:bg-destructive/10 hover:text-destructive border-destructive/30"
						onclick={() => deleteTarget = { type: 'file', item: f }}>
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
			<Dialog.Title>Delete {deleteTarget?.type}?</Dialog.Title>
			<Dialog.Description>
				<strong class="break-all">{deleteTarget?.item?.name}</strong> will be permanently deleted.
				{#if deleteTarget?.type === 'folder'}
					All files and subfolders inside will also be deleted.
				{/if}
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

<!-- ── Rename dialog ──────────────────────────────────────────────────────── -->
<Dialog.Root open={!!renameTarget} onOpenChange={(o) => { if (!o) renameTarget = null; }}>
	<Dialog.Content class="max-w-sm">
		<Dialog.Header>
			<Dialog.Title>Rename {renameTarget?.type}</Dialog.Title>
			<Dialog.Description>Enter a new name for <strong>{renameTarget?.item?.name}</strong></Dialog.Description>
		</Dialog.Header>
		<form onsubmit={(e) => { e.preventDefault(); confirmRename(); }}>
			<Input bind:value={renameValue} placeholder="New name" class="mb-4" autofocus />
			<Dialog.Footer class="gap-2">
				<Button variant="outline" onclick={() => renameTarget = null} disabled={renaming}>Cancel</Button>
				<Button type="submit" disabled={renaming || !renameValue.trim()}>
					{renaming ? 'Renaming…' : 'Rename'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

<!-- ── New folder dialog ──────────────────────────────────────────────────── -->
<Dialog.Root open={showNewFolder} onOpenChange={(o) => { if (!o) showNewFolder = false; }}>
	<Dialog.Content class="max-w-sm">
		<Dialog.Header>
			<Dialog.Title>New folder</Dialog.Title>
			<Dialog.Description>Create a new folder{fileStore.breadcrumb.length > 0 ? ` in "${fileStore.breadcrumb[fileStore.breadcrumb.length - 1].name}"` : ''}</Dialog.Description>
		</Dialog.Header>
		<form onsubmit={(e) => { e.preventDefault(); confirmNewFolder(); }}>
			<Input bind:value={newFolderName} placeholder="Folder name" class="mb-4" autofocus />
			<Dialog.Footer class="gap-2">
				<Button variant="outline" onclick={() => showNewFolder = false} disabled={creatingFolder}>Cancel</Button>
				<Button type="submit" disabled={creatingFolder || !newFolderName.trim()}>
					{creatingFolder ? 'Creating…' : 'Create'}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

<!-- ── Move dialog ────────────────────────────────────────────────────────── -->
<Dialog.Root open={!!moveTarget} onOpenChange={(o) => { if (!o) moveTarget = null; }}>
	<Dialog.Content class="max-w-md">
		<Dialog.Header>
			<Dialog.Title>Move "{moveTarget?.item?.name}"</Dialog.Title>
			<Dialog.Description>Choose a destination folder</Dialog.Description>
		</Dialog.Header>
		<div class="my-4 rounded-lg border bg-muted/20 overflow-hidden">
			{#if loadingFolders}
				<div class="flex items-center justify-center py-10 text-muted-foreground">
					<svg class="h-4 w-4 animate-spin mr-2" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
					</svg>
					<span class="text-sm">Loading folders…</span>
				</div>
			{:else}
				<div class="max-h-72 overflow-y-auto py-1">
					{#each flatFolderList as node (node.id ?? '__root__')}
						{@const isSelected = moveDestination === node.id}
						{@const hasChildren = node.children.length > 0}
						{@const isExpanded = expandedFolders.has(node.id)}
						<div
							class="flex items-center group hover:bg-accent/60 transition-colors"
							style="padding-left: {node.depth * 20 + 8}px"
						>
							<!-- Expand/collapse chevron -->
							<button
								class="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded hover:bg-accent text-muted-foreground"
								onclick={() => toggleExpand(node.id)}
								aria-label={isExpanded ? 'Collapse' : 'Expand'}
								disabled={!hasChildren}
								style="visibility: {hasChildren ? 'visible' : 'hidden'}"
							>
								<svg xmlns="http://www.w3.org/2000/svg"
									class="h-3.5 w-3.5 transition-transform {isExpanded ? 'rotate-90' : ''}"
									fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
								</svg>
							</button>

							<!-- Folder row (click to select) -->
							<button
								class="flex flex-1 items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors {isSelected ? 'bg-primary text-primary-foreground font-medium' : 'hover:bg-accent'}"
								onclick={() => moveDestination = node.id}
							>
								<!-- Folder icon -->
								{#if isExpanded && hasChildren}
									<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 flex-shrink-0 {isSelected ? 'text-primary-foreground' : 'text-blue-500'}" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
										<path stroke-linecap="round" stroke-linejoin="round" d="M5 19a2 2 0 01-2-2V7a2 2 0 012-2h4l2 2h4a2 2 0 012 2v1M5 19h14a2 2 0 002-2v-5a2 2 0 00-2-2H9a2 2 0 00-2 2v5a2 2 0 01-2 2z"/>
									</svg>
								{:else}
									<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 flex-shrink-0 {isSelected ? 'text-primary-foreground' : 'text-blue-500'}" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
										<path stroke-linecap="round" stroke-linejoin="round" d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V7z"/>
									</svg>
								{/if}
								<span class="truncate">{node.name}</span>
							</button>
						</div>
					{/each}
				</div>
			{/if}
		</div>
		<Dialog.Footer class="gap-2">
			<Button variant="outline" onclick={() => moveTarget = null} disabled={moving}>Cancel</Button>
			<Button onclick={confirmMove} disabled={moving}>
				{moving ? 'Moving…' : 'Move here'}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- ── Preview dialog ─────────────────────────────────────────────────────── -->
<Dialog.Root open={!!previewFile} onOpenChange={(o) => { if (!o) closePreview(); }}>
	<Dialog.Content class="sm:max-w-4xl max-h-[90vh] flex flex-col p-0 gap-0 overflow-hidden">
		<!-- Header -->
		<div class="flex items-center justify-between gap-4 border-b px-6 py-4">
			<div class="min-w-0 flex-1">
				<Dialog.Title class="truncate text-base font-semibold">{previewFile?.name}</Dialog.Title>
				<Dialog.Description class="text-xs text-muted-foreground mt-0.5">
					{previewFile ? `${mimeLabel(previewFile.mime_type)} · ${formatBytes(previewFile.total_size)}` : ''}
				</Dialog.Description>
			</div>
			<div class="flex items-center gap-1.5 flex-shrink-0">
				{#if previewFile}
					<Button variant="ghost" size="sm" class="gap-1.5 text-muted-foreground" onclick={() => previewFile && handleDownload(previewFile)}>
						<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
							<path stroke-linecap="round" stroke-linejoin="round" d="M4 16v2a2 2 0 002 2h12a2 2 0 002-2v-2M7 10l5 5 5-5M12 15V3"/>
						</svg>
						<span class="hidden sm:inline">Download</span>
					</Button>
				{/if}
			</div>
		</div>

		<!-- Preview body -->
		<div class="flex-1 overflow-auto min-h-0">
			{#if previewLoading}
				<div class="flex flex-col items-center justify-center py-24 gap-3">
					<svg class="h-6 w-6 animate-spin text-muted-foreground" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
						<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
						<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
					</svg>
					<p class="text-sm text-muted-foreground">Loading preview…</p>
				</div>
			{:else if previewText}
				<div class="bg-muted/30 border-b">
					<pre class="p-6 text-sm whitespace-pre-wrap break-all font-mono leading-relaxed max-h-[70vh] overflow-auto">{previewText}</pre>
				</div>
			{:else if previewUrl && previewFile?.mime_type.startsWith('image/')}
				<div class="flex items-center justify-center bg-muted/10 p-4 sm:p-6">
					<img
						src={previewUrl}
						alt={previewFile?.name}
						class="max-w-full max-h-[70vh] rounded-lg object-contain shadow-sm"
					/>
				</div>
			{:else if previewUrl && previewFile?.mime_type === 'application/pdf'}
				<iframe src={previewUrl} title={previewFile?.name} class="w-full h-[75vh] border-0"></iframe>
			{:else}
				<div class="flex flex-col items-center justify-center py-24 gap-2">
					<p class="text-4xl">📄</p>
					<p class="text-muted-foreground text-sm">Preview not available for this file type</p>
				</div>
			{/if}
		</div>
	</Dialog.Content>
</Dialog.Root>

<!-- ── Share dialog ───────────────────────────────────────────────────────── -->
<Dialog.Root open={!!shareFile} onOpenChange={(o) => { if (!o) shareFile = null; }}>
	<Dialog.Content class="max-w-md">
		<Dialog.Header>
			<Dialog.Title>Share "{shareFile?.name}"</Dialog.Title>
			<Dialog.Description>Create a link anyone can use to download this file</Dialog.Description>
		</Dialog.Header>
		<div class="my-4 space-y-3">
			<Button class="w-full gap-2" onclick={handleCreateShareLink} disabled={shareLoading}>
				<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
					<path stroke-linecap="round" stroke-linejoin="round" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"/>
				</svg>
				Create new link
			</Button>

			{#if shareLinks.length > 0}
				<div class="space-y-2">
					{#each shareLinks as link}
						<div class="flex items-center gap-2 rounded-lg border p-3 text-sm">
							<div class="flex-1 min-w-0">
								<p class="truncate font-mono text-xs text-muted-foreground">{link.token.slice(0, 16)}…</p>
								{#if link.expires_at}
									<p class="text-xs text-muted-foreground mt-0.5">Expires {formatDate(link.expires_at)}</p>
								{/if}
							</div>
							<button class="rounded p-1.5 hover:bg-accent text-muted-foreground" title="Copy link"
								onclick={() => copyShareUrl(link.token)}>
								{#if shareCopied}
									<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
										<path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
									</svg>
								{:else}
									<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
										<rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>
									</svg>
								{/if}
							</button>
							<button class="rounded p-1.5 hover:bg-destructive/10 text-destructive" title="Delete link"
								onclick={() => handleDeleteShareLink(link.id)}>
								<svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
									<path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6M9 7V4h6v3M3 7h18"/>
								</svg>
							</button>
						</div>
					{/each}
				</div>
			{:else if !shareLoading}
				<p class="text-center text-sm text-muted-foreground py-4">No share links yet. Create one above.</p>
			{/if}
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={() => shareFile = null}>Close</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
