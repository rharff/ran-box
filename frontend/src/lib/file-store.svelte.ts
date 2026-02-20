import {
	listFolderContents,
	uploadFile,
	deleteFile,
	downloadFile,
	renameFile,
	moveFile,
	createFolder,
	renameFolder as apiFolderRename,
	moveFolder as apiFolderMove,
	deleteFolder as apiFolderDelete,
	getBreadcrumb,
	listFiles
} from './api';
import type { NaratelFile, Folder } from './types';

// ── Reactive file store using Svelte 5 runes ─────────────────────────────────

let files = $state<NaratelFile[]>([]);
let folders = $state<Folder[]>([]);
let breadcrumb = $state<Folder[]>([]);
let currentFolderId = $state<number | null>(null);
let loading = $state(false);
let error = $state('');

// Upload state
let uploading = $state(false);
let uploadProgress = $state(0);
let uploadQueue = $state<string[]>([]);
let uploadErrors = $state<string[]>([]);

// Selection
let selectedFileIds = $state<Set<number>>(new Set());
let selectedFolderIds = $state<Set<number>>(new Set());

// Search
let searchQuery = $state('');
let searchResults = $state<NaratelFile[]>([]);
let isSearching = $state(false);

export const fileStore = {
	get files() { return files; },
	get folders() { return folders; },
	get breadcrumb() { return breadcrumb; },
	get currentFolderId() { return currentFolderId; },
	get loading() { return loading; },
	get error() { return error; },
	get uploading() { return uploading; },
	get uploadProgress() { return uploadProgress; },
	get uploadQueue() { return uploadQueue; },
	get uploadErrors() { return uploadErrors; },
	get selectedFileIds() { return selectedFileIds; },
	get selectedFolderIds() { return selectedFolderIds; },
	get searchQuery() { return searchQuery; },
	get searchResults() { return searchResults; },
	get isSearching() { return isSearching; },

	get displayFiles(): NaratelFile[] {
		if (isSearching && searchQuery.trim()) return searchResults;
		return files;
	},

	get displayFolders(): Folder[] {
		if (isSearching && searchQuery.trim()) return [];
		return folders;
	},

	get totalSize(): number {
		return files.reduce((acc, f) => acc + f.total_size, 0);
	},

	get hasSelection(): boolean {
		return selectedFileIds.size > 0 || selectedFolderIds.size > 0;
	},

	get selectionCount(): number {
		return selectedFileIds.size + selectedFolderIds.size;
	},

	async loadFolder(folderId: number | null = null) {
		loading = true;
		error = '';
		currentFolderId = folderId;
		isSearching = false;
		searchQuery = '';

		try {
			const contents = await listFolderContents(folderId);
			folders = contents.folders ?? [];
			files = contents.files ?? [];

			// Load breadcrumb
			if (folderId != null) {
				breadcrumb = await getBreadcrumb(folderId);
			} else {
				breadcrumb = [];
			}
		} catch {
			error = 'Failed to load folder contents. Please refresh.';
		} finally {
			loading = false;
		}
	},

	async search(query: string) {
		searchQuery = query;
		if (!query.trim()) {
			isSearching = false;
			searchResults = [];
			return;
		}
		isSearching = true;
		try {
			searchResults = await listFiles(undefined, query);
		} catch {
			searchResults = [];
		}
	},

	async upload(fileList: FileList | File[]) {
		const items = Array.from(fileList);
		uploading = true;
		uploadErrors = [];
		uploadQueue = items.map((f) => f.name);

		for (let i = 0; i < items.length; i++) {
			const file = items[i];
			uploadQueue = items.slice(i).map((f) => f.name);
			uploadProgress = 0;
			try {
				await uploadFile(file, currentFolderId, (pct) => { uploadProgress = pct; });
			} catch (err: any) {
				const msg = err?.response?.data?.message ?? `Failed to upload "${file.name}"`;
				uploadErrors = [...uploadErrors, msg];
			}
		}

		uploading = false;
		uploadQueue = [];
		uploadProgress = 0;
		await fileStore.loadFolder(currentFolderId);
	},

	// ── File operations ───────────────────────────────────────────────────

	async deleteFileItem(id: number) {
		await deleteFile(id);
		files = files.filter((f) => f.id !== id);
		selectedFileIds.delete(id);
		selectedFileIds = new Set(selectedFileIds);
	},

	async download(id: number, name: string) {
		await downloadFile(id, name);
	},

	async renameFileItem(id: number, newName: string) {
		const updated = await renameFile(id, newName);
		files = files.map((f) => (f.id === id ? updated : f));
	},

	async moveFileItem(id: number, folderId: number | null) {
		await moveFile(id, folderId);
		// Reload current folder
		await fileStore.loadFolder(currentFolderId);
	},

	// ── Folder operations ─────────────────────────────────────────────────

	async createFolder(name: string) {
		await createFolder(name, currentFolderId);
		await fileStore.loadFolder(currentFolderId);
	},

	async renameFolderItem(id: number, newName: string) {
		const updated = await apiFolderRename(id, newName);
		folders = folders.map((f) => (f.id === id ? updated : f));
	},

	async moveFolderItem(id: number, parentId: number | null) {
		await apiFolderMove(id, parentId);
		await fileStore.loadFolder(currentFolderId);
	},

	async deleteFolderItem(id: number) {
		await apiFolderDelete(id);
		folders = folders.filter((f) => f.id !== id);
		selectedFolderIds.delete(id);
		selectedFolderIds = new Set(selectedFolderIds);
	},

	navigateToFolder(folderId: number | null) {
		fileStore.clearSelection();
		fileStore.loadFolder(folderId);
	},

	// ── Selection ─────────────────────────────────────────────────────────

	toggleFileSelect(id: number) {
		if (selectedFileIds.has(id)) {
			selectedFileIds.delete(id);
		} else {
			selectedFileIds.add(id);
		}
		selectedFileIds = new Set(selectedFileIds);
	},

	toggleFolderSelect(id: number) {
		if (selectedFolderIds.has(id)) {
			selectedFolderIds.delete(id);
		} else {
			selectedFolderIds.add(id);
		}
		selectedFolderIds = new Set(selectedFolderIds);
	},

	selectAll() {
		selectedFileIds = new Set(files.map((f) => f.id));
		selectedFolderIds = new Set(folders.map((f) => f.id));
	},

	clearSelection() {
		selectedFileIds = new Set();
		selectedFolderIds = new Set();
	},

	async deleteSelected() {
		const fileIds = Array.from(selectedFileIds);
		const folderIds = Array.from(selectedFolderIds);
		await Promise.all([
			...fileIds.map((id) => deleteFile(id)),
			...folderIds.map((id) => apiFolderDelete(id))
		]);
		files = files.filter((f) => !fileIds.includes(f.id));
		folders = folders.filter((f) => !folderIds.includes(f.id));
		selectedFileIds = new Set();
		selectedFolderIds = new Set();
	},

	setSearch(q: string) {
		fileStore.search(q);
	}
};
