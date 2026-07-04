import type { KnowledgeLocation } from '../app/router';
import type { WorkspaceConfig } from '../lib/types';

export function KnowledgePage({ workspaces, location, onLocationChange }: { workspaces: WorkspaceConfig[]; location?: KnowledgeLocation; onLocationChange: (location: KnowledgeLocation) => void }) {
	void workspaces; void location; void onLocationChange;
	return <section className="empty-state">Loading Knowledge...</section>;
}
