import { SessionsPage as SessionsPageContent } from '../features/sessions/SessionsPage';

export function SessionsPage({ onOpenSession, onOpenWorkbench }: {
	onOpenSession: (workspaceId: string, sessionNodeId: string) => void;
	onOpenWorkbench: () => void;
}) {
	return <SessionsPageContent onOpenSession={onOpenSession} onOpenWorkbench={onOpenWorkbench} />;
}
