import axios from 'axios';
import { PUBLIC_API_BASE_URL } from '$env/static/public';
import type { User, TokenResponse, NaratelFile, UploadResponse, Folder, FolderContents, ShareLink } from './types';

export const api = axios.create({
	baseURL: `${PUBLIC_API_BASE_URL}/api/v1`
});

// Inject Bearer token from localStorage on every request
api.interceptors.request.use((config) => {
	const token = localStorage.getItem('token');
	if (token) config.headers.Authorization = `Bearer ${token}`;
	return config;
});

// ── Auth ──────────────────────────────────────────────────────────────────────

export async function register(email: string, password: string): Promise<User> {
	const res = await api.post<User>('/auth/register', { email, password });
	return res.data;
}

export async function login(email: string, password: string): Promise<TokenResponse> {
	const res = await api.post<TokenResponse>('/auth/login', { email, password });
	return res.data;
}

export async function getMe(): Promise<User> {
	const res = await api.get<User>('/auth/me');
	return res.data;
}

// ── Files ─────────────────────────────────────────────────────────────────────

export async function listFiles(folderId?: number | null, search?: string): Promise<NaratelFile[]> {
	const params: Record<string, string> = {};
	if (folderId != null) params.folder_id = String(folderId);
	if (search) params.search = search;
	const res = await api.get<NaratelFile[]>('/files', { params });
	return res.data;
}

export async function uploadFile(
	file: File,
	folderId?: number | null,
	onProgress?: (pct: number) => void
): Promise<UploadResponse> {
	const form = new FormData();
	form.append('file', file);
	if (folderId != null) form.append('folder_id', String(folderId));
	const res = await api.post<UploadResponse>('/files', form, {
		headers: { 'Content-Type': undefined },
		onUploadProgress: (e) => {
			if (onProgress && e.total) onProgress(Math.round((e.loaded * 100) / e.total));
		}
	});
	return res.data;
}

export async function deleteFile(id: number): Promise<void> {
	await api.delete(`/files/${id}`);
}

export async function renameFile(id: number, name: string): Promise<NaratelFile> {
	const res = await api.patch<NaratelFile>(`/files/${id}/rename`, { name });
	return res.data;
}

export async function moveFile(id: number, folderId: number | null): Promise<NaratelFile> {
	const res = await api.patch<NaratelFile>(`/files/${id}/move`, { folder_id: folderId });
	return res.data;
}

export async function getFileInfo(id: number): Promise<NaratelFile> {
	const res = await api.get<NaratelFile>(`/files/${id}/info`);
	return res.data;
}

export function downloadUrl(id: number): string {
	const token = localStorage.getItem('token');
	return `${PUBLIC_API_BASE_URL}/api/v1/files/${id}?token=${token}`;
}

export function previewUrl(id: number): string {
	return `${PUBLIC_API_BASE_URL}/api/v1/files/${id}?preview=true`;
}

export async function downloadFile(id: number, name: string): Promise<void> {
	const token = localStorage.getItem('token');
	const res = await fetch(`${PUBLIC_API_BASE_URL}/api/v1/files/${id}`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	if (!res.ok) throw new Error('Download failed');
	const blob = await res.blob();
	const url = URL.createObjectURL(blob);
	const a = document.createElement('a');
	a.href = url;
	a.download = name;
	a.click();
	URL.revokeObjectURL(url);
}

export async function getFilePreviewBlob(id: number): Promise<{ blob: Blob; mimeType: string }> {
	const token = localStorage.getItem('token');
	const res = await fetch(`${PUBLIC_API_BASE_URL}/api/v1/files/${id}?preview=true`, {
		headers: { Authorization: `Bearer ${token}` }
	});
	if (!res.ok) throw new Error('Preview failed');
	const blob = await res.blob();
	return { blob, mimeType: blob.type };
}

// ── Folders ───────────────────────────────────────────────────────────────────

export async function createFolder(name: string, parentId?: number | null): Promise<Folder> {
	const body: Record<string, unknown> = { name };
	if (parentId != null) body.parent_id = parentId;
	const res = await api.post<Folder>('/folders', body);
	return res.data;
}

export async function listFolderContents(folderId?: number | null): Promise<FolderContents> {
	const params: Record<string, string> = {};
	if (folderId != null) params.folder_id = String(folderId);
	const res = await api.get<FolderContents>('/folders/contents', { params });
	return res.data;
}

export async function listAllFolders(): Promise<Folder[]> {
	const res = await api.get<Folder[]>('/folders/all');
	return res.data;
}

export async function getBreadcrumb(folderId: number): Promise<Folder[]> {
	const res = await api.get<Folder[]>(`/folders/${folderId}/breadcrumb`);
	return res.data;
}

export async function renameFolder(id: number, name: string): Promise<Folder> {
	const res = await api.patch<Folder>(`/folders/${id}/rename`, { name });
	return res.data;
}

export async function moveFolder(id: number, parentId: number | null): Promise<Folder> {
	const res = await api.patch<Folder>(`/folders/${id}/move`, { parent_id: parentId });
	return res.data;
}

export async function deleteFolder(id: number): Promise<void> {
	await api.delete(`/folders/${id}`);
}

// ── Share Links ───────────────────────────────────────────────────────────────

export async function createShareLink(fileId: number): Promise<ShareLink> {
	const res = await api.post<ShareLink>(`/files/${fileId}/share`);
	return res.data;
}

export async function getShareLinks(fileId: number): Promise<ShareLink[]> {
	const res = await api.get<ShareLink[]>(`/files/${fileId}/share`);
	return res.data;
}

export async function deleteShareLink(linkId: number): Promise<void> {
	await api.delete(`/share/${linkId}`);
}

export function shareDownloadUrl(token: string): string {
	return `${PUBLIC_API_BASE_URL}/api/v1/share/${token}`;
}

export function sharePreviewUrl(token: string): string {
	return `${PUBLIC_API_BASE_URL}/api/v1/share/${token}?preview=true`;
}
