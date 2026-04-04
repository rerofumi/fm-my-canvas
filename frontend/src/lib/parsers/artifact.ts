export interface ArtifactFile {
	path: string;
	language: string;
	content: string;
}

const codeBlockRe = /```(\w+)(?:\s+path=(\S+))?\s*\n([\s\S]*?)```/g;

const langToExt: Record<string, string> = {
	html: '.html',
	css: '.css',
	javascript: '.js',
	js: '.js',
	typescript: '.ts',
	ts: '.ts',
	json: '.json',
};

function inferPath(lang: string, path: string | undefined): string {
	if (path) return path;
	switch (lang) {
		case 'html': return 'index.html';
		case 'css': return 'style.css';
		case 'javascript':
		case 'js': return 'script.js';
		default: return `file${langToExt[lang] || '.txt'}`;
	}
}

export function parseArtifacts(text: string): ArtifactFile[] {
	const files: ArtifactFile[] = [];
	let match: RegExpExecArray | null;
	const re = new RegExp(codeBlockRe.source, codeBlockRe.flags);
	while ((match = re.exec(text)) !== null) {
		const lang = match[1];
		const path = inferPath(lang, match[2] || undefined);
		const content = match[3].trimEnd();
		files.push({ path, language: lang, content });
	}
	return files;
}

export function parseStreamingArtifacts(text: string): {
	files: ArtifactFile[];
	activeFilePath: string | null;
} {
	const files = parseArtifacts(text);
	let activeFilePath: string | null = null;

	const lastBlockOpen = text.lastIndexOf('```');
	if (lastBlockOpen !== -1) {
		const afterOpen = text.substring(lastBlockOpen);
		if (!afterOpen.includes('```', 3)) {
			const headerMatch = afterOpen.match(/^```(\w+)(?:\s+path=(\S+))?\s*\n?([\s\S]*)$/);
			if (headerMatch) {
				const lang = headerMatch[1];
				const path = inferPath(lang, headerMatch[2] || undefined);
				const content = (headerMatch[3] || '').trimEnd();
				activeFilePath = path;
				const existing = files.find(f => f.path === path);
				if (existing) {
					existing.content = content;
				} else {
					files.push({ path, language: lang, content });
				}
			}
		}
	}

	return { files, activeFilePath };
}
