import { listFiles, uploadFile, deleteFile, downloadFile } from './api';
import type { NaratelFile } from './types';

// ── Reactive file store using Svelte 5 runes ─────────────────────────────────

let files = $state<NaratelFile[]>([]);
let loading = $state(false);
let error = $state('');

// Upload state
let uploading = $state(false);
let uploadProgress = $state(0);
let uploadQueue = $state<string[]>([]);   // filenames in progress
let uploadErrors = $state<string[]>([]);

// Selection
let selectedIds = $state<Set<number>>(new Set());

// Search
let searchQuery = $state('');

export const fileStore = {
	get files() { return files; },
	get loading() { return loading; },
	get error() { return error; },
	get uploading() { return uploading; },
	get uploadProgress() { return uploadProgress; },
	get uploadQueue() { return uploadQueue; },
	get uploadErrors() { return uploadErrors; },
	get selectedIds() { return selectedIds; },
	get searchQuery() { return searchQuery; },

	get filteredFiles(): NaratelFile[] {
		if (!searchQuery.trim()) return files;
		const q = searchQuery.toLowerCase();
		return files.filter((f) => f.name.toLowerCase().includes(q));
	},

	get totalSize(): number {
		return files.reduce((acc, f) => acc + f.total_size, 0);
	},

	async load() {
		loading = true;
		error = '';
		try {
			files = await listFiles();
		} catch {
			error = 'Failed to load files. Please refresh.';
		} finally {
			loading = false;
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
				await uploadFile(file, (pct) => { uploadProgress = pct; });
			} catch (err: any) {
				const msg = err?.response?.data?.message ?? `Failed to upload "${file.name}"`;
				uploadErrors = [...uploadErrors, msg];
			}
		}

		uploading = false;
		uploadQueue = [];
		uploadProgress = 0;
		await fileStore.load();
	},

	async delete(id: number) {
		await deleteFile(id);
		files = files.filter((f) => f.id !== id);
		selectedIds.delete(id);
		selectedIds = new Set(selectedIds);
	},

	async download(id: number, name: string) {
		await downloadFile(id, name);
	},

	toggleSelect(id: number) {
		if (selectedIds.has(id)) {
			selectedIds.delete(id);
		} else {
			selectedIds.add(id);
		}
		selectedIds = new Set(selectedIds);
	},

	selectAll() {
		selectedIds = new Set(fileStore.filteredFiles.map((f) => f.id));
	},

	clearSelection() {
		selectedIds = new Set();
	},

	async deleteSelected() {
		const ids = Array.from(selectedIds);
		await Promise.all(ids.map((id) => deleteFile(id)));
		files = files.filter((f) => !ids.includes(f.id));
		selectedIds = new Set();
	},

	setSearch(q: string) {
		searchQuery = q;
	}
};
