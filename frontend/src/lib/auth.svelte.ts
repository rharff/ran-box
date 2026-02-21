import { goto } from '$app/navigation';
import { browser } from '$app/environment';
import { login as apiLogin, register as apiRegister, getMe } from './api';
import type { User } from './types';

// Svelte 5 rune-based reactive store
let token = $state<string | null>(null);
let user = $state<User | null>(null);
let loading = $state(false);
let initialized = $state(false);

// Hydrate from localStorage on module load (client-side only)
if (typeof localStorage !== 'undefined') {
	token = localStorage.getItem('token');
}

export const auth = {
	get token() { return token; },
	get user() { return user; },
	get loading() { return loading; },
	get isAuthenticated() { return !!token; },
	get initialized() { return initialized; },

	async init() {
		if (!browser) return;
		if (initialized) return;
		if (!token) {
			initialized = true;
			return;
		}
		try {
			user = await getMe();
		} catch {
			// Token expired or invalid — clear it
			token = null;
			user = null;
			localStorage.removeItem('token');
		} finally {
			initialized = true;
		}
	},

	async login(email: string, password: string) {
		loading = true;
		try {
			const res = await apiLogin(email, password);
			token = res.token;
			localStorage.setItem('token', res.token);
			user = await getMe();
			goto('/dashboard');
		} finally {
			loading = false;
		}
	},

	async register(email: string, password: string) {
		loading = true;
		try {
			await apiRegister(email, password);
			// Auto-login after register
			await auth.login(email, password);
		} finally {
			loading = false;
		}
	},

	logout() {
		token = null;
		user = null;
		localStorage.removeItem('token');
		goto('/login');
	}
};
