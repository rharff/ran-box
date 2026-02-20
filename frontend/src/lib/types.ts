export interface User {
	user_id: number;
	email: string;
	created_at: string;
}

export interface TokenResponse {
	token: string;
	expires_at: string;
}

export interface NaratelFile {
	id: number;
	user_id: number;
	folder_id: number | null;
	name: string;
	mime_type: string;
	total_size: number;
	created_at: string;
	updated_at: string;
}

export interface Folder {
	id: number;
	user_id: number;
	parent_id: number | null;
	name: string;
	created_at: string;
	updated_at: string;
}

export interface FolderContents {
	folders: Folder[];
	files: NaratelFile[];
}

export interface ShareLink {
	id: number;
	file_id: number;
	token: string;
	url: string;
	expires_at: string | null;
	created_at: string;
}

export interface UploadResponse {
	file_id: number;
	name: string;
	mime_type: string;
	size: number;
	created_at: string;
}

export interface ApiError {
	error: string;
	message: string;
}

export type ViewMode = 'grid' | 'list';
export type SortField = 'name' | 'total_size' | 'created_at';
export type SortDir = 'asc' | 'desc';
