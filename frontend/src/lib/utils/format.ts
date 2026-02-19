export function formatBytes(bytes: number): string {
	if (bytes === 0) return '0 B';
	if (bytes < 1024) return `${bytes} B`;
	if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
	if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
	return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`;
}

export function formatDate(iso: string): string {
	const d = new Date(iso);
	const now = new Date();
	const diff = now.getTime() - d.getTime();
	const mins = Math.floor(diff / 60000);
	const hours = Math.floor(diff / 3600000);
	const days = Math.floor(diff / 86400000);

	if (mins < 1) return 'Just now';
	if (mins < 60) return `${mins}m ago`;
	if (hours < 24) return `${hours}h ago`;
	if (days < 7) return `${days}d ago`;
	return d.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

export function formatDateFull(iso: string): string {
	return new Date(iso).toLocaleString(undefined, {
		year: 'numeric', month: 'short', day: 'numeric',
		hour: '2-digit', minute: '2-digit'
	});
}

export function mimeLabel(mime: string): string {
	if (mime.startsWith('image/')) return 'Image';
	if (mime.startsWith('video/')) return 'Video';
	if (mime.startsWith('audio/')) return 'Audio';
	if (mime.includes('pdf')) return 'PDF';
	if (mime.includes('zip') || mime.includes('tar') || mime.includes('gzip') || mime.includes('7z') || mime.includes('rar')) return 'Archive';
	if (mime.startsWith('text/')) return 'Text';
	if (mime.includes('word') || mime.includes('document')) return 'Document';
	if (mime.includes('sheet') || mime.includes('excel')) return 'Spreadsheet';
	if (mime.includes('presentation') || mime.includes('powerpoint')) return 'Presentation';
	return 'File';
}

export function mimeIcon(mime: string): string {
	if (mime.startsWith('image/')) return '🖼️';
	if (mime.startsWith('video/')) return '🎬';
	if (mime.startsWith('audio/')) return '🎵';
	if (mime.includes('pdf')) return '📄';
	if (mime.includes('zip') || mime.includes('tar') || mime.includes('gzip')) return '🗜️';
	if (mime.includes('word') || mime.includes('document')) return '📝';
	if (mime.includes('sheet') || mime.includes('excel')) return '📊';
	if (mime.includes('presentation') || mime.includes('powerpoint')) return '📑';
	if (mime.startsWith('text/')) return '📃';
	return '📎';
}

export function mimeColor(mime: string): string {
	if (mime.startsWith('image/')) return 'bg-purple-100 text-purple-700';
	if (mime.startsWith('video/')) return 'bg-red-100 text-red-700';
	if (mime.startsWith('audio/')) return 'bg-pink-100 text-pink-700';
	if (mime.includes('pdf')) return 'bg-orange-100 text-orange-700';
	if (mime.includes('zip') || mime.includes('tar')) return 'bg-yellow-100 text-yellow-700';
	if (mime.includes('word') || mime.includes('document')) return 'bg-blue-100 text-blue-700';
	if (mime.includes('sheet') || mime.includes('excel')) return 'bg-green-100 text-green-700';
	if (mime.startsWith('text/')) return 'bg-gray-100 text-gray-700';
	return 'bg-slate-100 text-slate-700';
}
