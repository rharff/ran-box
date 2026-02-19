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
	name: string;
	mime_type: string;
	total_size: number;
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
