import type { ContentSearchSelection, WorkspaceContentSearchResult } from '../../lib/types';

export function selectionFromContentResult(result: WorkspaceContentSearchResult): ContentSearchSelection {
	return {
		workspaceId: result.workspaceId,
		itemId: result.itemId,
		path: result.path,
		fileId: result.fileId,
		lineNumber: result.lineNumber,
		columnStart: result.columnStart,
		columnEnd: result.columnEnd
	};
}
