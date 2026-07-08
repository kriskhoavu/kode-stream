import { describe, expect, it } from 'vitest';
import { applySegmentRole, buildWorkspaceInput, defaultWorkspaceImportSelection, inferCompatibilityFields, inferWorkspaceNameFromRemoteURL, normalizeDroppedPath, normalizeKnowledgeSettings, parseSources, previewPathSegments, settingsEditorFromResult, workspaceRemovalMessage } from './WorkspacesPage';

describe('normalizeDroppedPath', () => {
  it('decodes file URLs dropped onto the path field', () => {
    expect(normalizeDroppedPath('file:///Users/me/My%20Repo')).toBe('/Users/me/My Repo');
  });

  it('keeps plain paths intact', () => {
    expect(normalizeDroppedPath('"/Users/me/repo"')).toBe('/Users/me/repo');
  });
});

describe('parseSources', () => {
  it('parses comma-separated plan roots', () => {
    expect(parseSources('plans, docs, docs/plans')).toEqual(['plans', 'docs', 'docs/plans']);
  });

  it('deduplicates and ignores empty entries', () => {
    expect(parseSources('plans, , docs, plans')).toEqual(['plans', 'docs']);
  });
});

describe('buildWorkspaceInput', () => {
  it('builds local mode payload with path', () => {
    expect(buildWorkspaceInput({
      name: 'Local',
      registrationMode: 'local_path',
      path: '/repo',
      remoteUrl: '',
      cloneRoot: '',
      baselineBranch: 'main',
      sources: 'plans, docs'
    })).toEqual({
      name: 'Local',
      registrationMode: 'local_path',
      path: '/repo',
      baselineBranch: 'main',
      sources: ['plans', 'docs']
    });
  });

  it('builds remote mode payload with URL and clone root', () => {
    expect(buildWorkspaceInput({
      name: 'Remote',
      registrationMode: 'remote_clone',
      path: '/ignored',
      remoteUrl: ' git@bitbucket.org:team/repo.git ',
      cloneRoot: ' /Users/me/Library/Application Support/kode-stream/clone-root ',
      baselineBranch: 'develop',
      sources: 'plans'
    })).toEqual({
      name: 'Remote',
      registrationMode: 'remote_clone',
      remoteUrl: 'git@bitbucket.org:team/repo.git',
      cloneRoot: '/Users/me/Library/Application Support/kode-stream/clone-root',
      baselineBranch: 'develop',
      sources: ['plans']
    });
  });

  it('normalizes optional Jira metadata without a token value', () => {
    expect(buildWorkspaceInput({ name: 'Jira', registrationMode: 'local_path', path: '/repo', remoteUrl: '', cloneRoot: '', baselineBranch: 'main', sources: 'plans', jira: { deploymentType: 'cloud', baseUrl: 'https://company.atlassian.net/', projectKey: 'di', accountEmail: ' user@example.com ', tokenEnvVar: ' JIRA_TOKEN ' } })).toEqual(expect.objectContaining({
      jira: { deploymentType: 'cloud', baseUrl: 'https://company.atlassian.net', projectKey: 'DI', accountEmail: 'user@example.com', tokenEnvVar: 'JIRA_TOKEN' }
    }));
  });
});

describe('workspace import selection', () => {
	it('selects only selectable candidates recommended by the backend', () => {
		expect(defaultWorkspaceImportSelection({
			sourcePath: '/source.yaml', destinationPath: '/data.yaml', sourceFingerprint: 'hash',
			summary: { valid: 2, invalid: 1, duplicate: 0, alreadyRegistered: 0 },
			candidates: [
				{ candidateKey: 'selected', position: 1, status: 'valid', selected: true, issues: [], workspace: { name: 'One', path: '/one', baselineBranch: 'main', sources: [] } },
				{ candidateKey: 'not-recommended', position: 2, status: 'valid', selected: false, issues: [], workspace: { name: 'Two', path: '/two', baselineBranch: 'main', sources: [] } },
				{ candidateKey: 'invalid', position: 3, status: 'invalid', selected: true, issues: [], workspace: { name: 'Three', path: '/three', baselineBranch: 'main', sources: [] } }
			]
		})).toEqual(['selected']);
	});
});

describe('Knowledge settings', () => {
	it('defaults detection on and preserves literal argument order', () => {
		expect(normalizeKnowledgeSettings({ enrichExecutable: ' wiki-enrich ', enrichArgs: [' --source ', '', 'docs/$HOME'] })).toEqual({ enabled: true, enrichExecutable: 'wiki-enrich', enrichArgs: [' --source ', '', 'docs/$HOME'] });
	});
	it('preserves an explicit disabled state', () => expect(normalizeKnowledgeSettings({ enabled: false })).toEqual({ enabled: false, enrichExecutable: '', enrichArgs: [] }));
});

describe('inferWorkspaceNameFromRemoteURL', () => {
  it('infers name from SSH URLs', () => {
    expect(inferWorkspaceNameFromRemoteURL('git@bitbucket.org:team/kode-stream.git')).toBe('kode-stream');
  });

  it('infers name from HTTPS URLs', () => {
    expect(inferWorkspaceNameFromRemoteURL('https://bitbucket.org/team/repo')).toBe('repo');
  });
});

describe('inferCompatibilityFields', () => {
  it('maps the source root and item variable from the path pattern', () => {
    expect(inferCompatibilityFields('{folder}/feature/{item}', 'docs')).toEqual({
      scope: 'docs',
      identifier: '{item}'
    });
  });

  it('uses the source name when only an item variable exists', () => {
    expect(inferCompatibilityFields('{item}', 'docs')).toEqual({
      scope: 'docs',
      identifier: '{item}'
    });
  });

  it('keeps legacy service and ticket variables compatible', () => {
    expect(inferCompatibilityFields('{service}/{ticket}', 'plans')).toEqual({
      scope: 'plans',
      identifier: '{ticket}'
    });
  });
});

describe('source item path helpers', () => {
  it('returns preview path segments relative to the source directory', () => {
    expect(previewPathSegments('docs/api/feature/DI-101', 'docs')).toEqual(['api', 'feature', 'DI-101']);
  });

  it('applies a clicked segment role to the path pattern', () => {
    expect(applySegmentRole('api/feature/DI-101', ['api', 'feature', 'DI-101'], 0, 'folder')).toBe('{folder}/feature/DI-101');
    expect(applySegmentRole('{folder}/feature/DI-101', ['api', 'feature', 'DI-101'], 2, 'item')).toBe('{folder}/feature/{item}');
    expect(applySegmentRole('{folder}/{item}/DI-101', ['api', 'feature', 'DI-101'], 1, 'literal')).toBe('{folder}/feature/DI-101');
  });
});

describe('settingsEditorFromResult', () => {
  const workspace = { id: 'ws', name: 'Workspace', path: '/repo', baselineBranch: 'main', sources: ['docs'], createdAt: '' };

  it('starts new source settings from the best actual proposal', () => {
    const editor = settingsEditorFromResult(workspace, 'docs', {
      directory: 'docs',
      exists: false,
      settings: {
        version: 1,
        cards: [{
          pathPattern: '{folder}/feature/{item}',
          fields: { source: 'docs', item: '{item}', scope: 'docs', identifier: '{item}', title: 'readme_heading', status: 'draft', tags: ['docs'] }
        }]
      },
      warnings: [],
      proposals: [{
        id: 'actual-identifier',
        label: 'Item folders',
        summary: 'Creates 1 card, for example docs/a12.',
        confidence: 'high',
        card: {
          pathPattern: '{item}',
          fields: { source: 'docs', item: '{item}', scope: 'docs', identifier: '{item}', title: 'readme_heading', status: 'draft', tags: ['docs'] }
        },
        preview: [{ path: 'docs/a12', source: 'docs', item: 'a12', scope: 'docs', identifier: 'a12', title: 'A12', status: 'draft', tags: ['docs'] }]
      }],
      preview: []
    });

    expect(editor.card.pathPattern).toBe('{item}');
    expect(editor.selectedProposalId).toBe('actual-identifier');
    expect(editor.preview).toHaveLength(1);
    expect(editor.preview[0].path).toBe('docs/a12');
  });

  it('uses an unsorted source preview when no suggestion is selected', () => {
    const editor = settingsEditorFromResult(workspace, 'docs', {
      directory: 'docs',
      exists: false,
      settings: { version: 1, cards: [] },
      warnings: [],
      proposals: [],
      preview: []
    });

    expect(editor.selectedProposalId).toBe('unsorted');
    expect(editor.preview).toEqual([{
      path: 'docs',
      source: 'docs',
      item: 'docs',
      scope: 'docs',
      identifier: 'docs',
      title: 'docs',
      status: 'unsorted',
      tags: ['docs']
    }]);
  });
});

describe('workspaceRemovalMessage', () => {
  it('matches single workspace wording', () => {
    expect(workspaceRemovalMessage([
      { id: 'w1', name: 'Workspace A', path: '/repo-a', baselineBranch: 'main', sources: ['docs'], createdAt: '', clonePathManaged: true }
    ])).toContain('Remove Workspace A?');
  });

  it('mentions managed clone folders for multi-delete', () => {
    expect(workspaceRemovalMessage([
      { id: 'w1', name: 'Workspace A', path: '/repo-a', baselineBranch: 'main', sources: ['docs'], createdAt: '', clonePathManaged: true },
      { id: 'w2', name: 'Workspace B', path: '/repo-b', baselineBranch: 'main', sources: ['docs'], createdAt: '', clonePathManaged: false }
    ])).toContain('and 1 managed cloned repository folder will be deleted');
  });
});
