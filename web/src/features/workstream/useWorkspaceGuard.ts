import { useCallback, useEffect, useRef } from 'react';

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
