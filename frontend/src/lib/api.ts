import axios from 'axios';
import type { User, TokenResponse, NaratelFile, UploadResponse } from './types';

export const api = axios.create({
	baseURL: 'http://localhost:8080/api/v1',
	headers: { 'Content-Type': 'application/json' }
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

export async function listFiles(): Promise<NaratelFile[]> {
	const res = await api.get<NaratelFile[]>('/files');
	return res.data;
}

export async function uploadFile(
	file: File,
	onProgress?: (pct: number) => void
): Promise<UploadResponse> {
	const form = new FormData();
	form.append('file', file);
	const res = await api.post<UploadResponse>('/files', form, {
		headers: { 'Content-Type': 'multipart/form-data' },
		onUploadProgress: (e) => {
			if (onProgress && e.total) onProgress(Math.round((e.loaded * 100) / e.total));
		}
	});
	return res.data;
}

export async function deleteFile(id: number): Promise<void> {
	await api.delete(`/files/${id}`);
}

export function downloadUrl(id: number): string {
	const token = localStorage.getItem('token');
	// Return a URL — download triggered via anchor with auth header via fetch
	return `http://localhost:8080/api/v1/files/${id}?token=${token}`;
}

export async function downloadFile(id: number, name: string): Promise<void> {
	const token = localStorage.getItem('token');
	const res = await fetch(`http://localhost:8080/api/v1/files/${id}`, {
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
