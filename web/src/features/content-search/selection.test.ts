import { describe, expect, it } from 'vitest';
import { selectionFromContentResult } from './selection';

describe('content search selection', () => {
	it('preserves file and line context', () => {
		expect(selectionFromContentResult({
			id: 'result', workspaceId: 'ws', workspaceName: 'Workspace', itemId: 'item', path: 'plans/a.md', fileId: 'a_md', name: 'a.md',
			kind: 'markdown', language: 'markdown', lineNumber: 4, columnStart: 3, columnEnd: 9, snippet: 'a needle', ignored: false
		})).toEqual({ workspaceId: 'ws', itemId: 'item', path: 'plans/a.md', fileId: 'a_md', lineNumber: 4, columnStart: 3, columnEnd: 9 });
	});
});
