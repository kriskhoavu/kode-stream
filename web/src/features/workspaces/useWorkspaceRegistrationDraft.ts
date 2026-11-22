import { useState } from 'react';
import type { JiraConnection, WorkspaceRegistrationMode } from '../../lib/types';

/** What a person has typed into the Add workspace dialog so far. */
export function useWorkspaceRegistrationDraft() {
  const [name, setName] = useState('');
  const [registrationMode, setRegistrationMode] = useState<WorkspaceRegistrationMode>('local_path');
  const [path, setPath] = useState('');
  const [remoteUrl, setRemoteUrl] = useState('');
  const [cloneRoot, setCloneRoot] = useState('');
  const [baselineBranch, setBaselineBranch] = useState('master');
  const [sources, setSources] = useState('');
  const [jira, setJira] = useState<JiraConnection | null>(null);
  const [registrationStep, setRegistrationStep] = useState<1 | 2>(1);
  const [registrationNameEdited, setRegistrationNameEdited] = useState(false);

  /** Empties the draft; the clone folder goes back to the configured one. */
  const reset = (defaultCloneRoot: string) => {
    setName('');
    setRegistrationMode('local_path');
    setPath('');
    setRemoteUrl('');
    setCloneRoot(defaultCloneRoot);
    setBaselineBranch('master');
    setSources('');
    setJira(null);
    setRegistrationStep(1);
    setRegistrationNameEdited(false);
  };
  const dirty = Boolean(name.trim() || path.trim() || remoteUrl.trim() || sources.trim() || jira);

  return { name, setName, registrationMode, setRegistrationMode, path, setPath, remoteUrl, setRemoteUrl, cloneRoot, setCloneRoot, baselineBranch, setBaselineBranch, sources, setSources, jira, setJira, registrationStep, setRegistrationStep, registrationNameEdited, setRegistrationNameEdited, reset, dirty };
}
