import { api } from '../../shared/api';

export interface ItemFolder {
	workspaceId: string;
	itemPath: string;
}

export function relativePathParts(input: string): string[] | null {
	const parts = input.trim().split('/');
	return parts.some((part) => !part || part === '.' || part === '..') ? null : parts;
}

export function renamedPath(path: string, name: string): string | null {
	const trimmed = name.trim();
	if (!trimmed || trimmed === '.' || trimmed === '..' || trimmed.includes('/') || trimmed.includes('\\')) return null;
	const separator = path.lastIndexOf('/');
	return separator >= 0 ? `${path.slice(0, separator)}/${trimmed}` : trimmed;
}

export async function createItemPath(folder: ItemFolder, kind: 'file' | 'directory', parts: string[], parentDirectory: string, existingDirectories: Set<string>) {
	const absolute = (relative: string) => relative ? `${folder.itemPath}/${relative}` : folder.itemPath;
	let relativeParent = parentDirectory;
	for (const directory of kind === 'file' ? parts.slice(0, -1) : parts) {
		const relativePath = relativeParent ? `${relativeParent}/${directory}` : directory;
		if (!existingDirectories.has(relativePath)) {
			await api.createWorkspaceDirectory(folder.workspaceId, { parentPath: absolute(relativeParent), name: directory });
			existingDirectories.add(relativePath);
		}
		relativeParent = relativePath;
	}
	if (kind === 'file') await api.createWorkspaceFile(folder.workspaceId, { parentPath: absolute(relativeParent), name: parts.at(-1)!, content: '' });
}

export function renameItemPath(folder: ItemFolder, path: string, destination: string) {
	return api.renameWorkspacePath(folder.workspaceId, { path: `${folder.itemPath}/${path}`, destinationPath: `${folder.itemPath}/${destination}` });
}
