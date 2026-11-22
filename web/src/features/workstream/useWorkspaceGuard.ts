import { useCallback, useEffect, useRef } from 'react';

/**
 * Scopes a page's asynchronous work to one workspace. `track` returns a check
 * that turns false once the workspace changes or the page unmounts, so a late
 * response never paints a different workspace.
 */
export function useWorkspaceGuard(workspaceId: string | undefined) {
	const generation = useRef(0);
	const current = useRef(workspaceId);
	useEffect(() => {
		current.current = workspaceId;
		generation.current += 1;
		return () => { generation.current += 1; };
	}, [workspaceId]);
	return useCallback((id: string | undefined = current.current) => {
		const started = generation.current;
		return () => current.current === id && generation.current === started;
	}, []);
}
