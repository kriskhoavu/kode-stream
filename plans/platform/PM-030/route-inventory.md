# PM-030 Route Inventory

## Summary

Current backend API routing is registered through Go `http.ServeMux` route patterns. This inventory captures the pre-Gin baseline and risk labels used to select the first migration group.

| Risk   | Meaning                                                                  |
|--------|--------------------------------------------------------------------------|
| low    | Small read-only route with isolated controller and stable JSON contract  |
| medium | Read route with workspace state, file access, external config, or search |
| high   | Write, stream, Git, AI session, verification, or long-running workflow   |

## First Migration Group

| Method | Path                | Current Handler    | Risk | Baseline Coverage                        |
|--------|---------------------|--------------------|------|------------------------------------------|
| GET    | `/api/health`       | `a.ginHealth`      | low  | controller test + benchmark + Gin parity |
| GET    | `/api/audit-events` | `a.ginAuditEvents` | low  | controller test + benchmark + Gin parity |

## Full API Inventory

| Group         | Method | Path                                                       | Current Handler                  | Risk   |
|---------------|--------|------------------------------------------------------------|----------------------------------|--------|
| health        | GET    | `/api/health`                                              | `c.health`                       | low    |
| audit-events  | GET    | `/api/audit-events`                                        | `c.events`                       | low    |
| saved-filters | GET    | `/api/saved-filters`                                       | `c.filters`                      | medium |
| saved-filters | POST   | `/api/saved-filters`                                       | `c.saveFilter`                   | high   |
| saved-filters | DELETE | `/api/saved-filters/{id}`                                  | `c.deleteFilter`                 | high   |
| recent-items  | GET    | `/api/recent-items`                                        | `c.recents`                      | medium |
| recent-items  | POST   | `/api/recent-items`                                        | `c.recordRecent`                 | high   |
| system        | POST   | `/api/system/select-directory`                             | `c.selectDirectory`              | high   |
| system        | POST   | `/api/system/select-file`                                  | `c.selectFile`                   | high   |
| system        | POST   | `/api/system/open-path`                                    | `c.openPath`                     | high   |
| system        | GET    | `/api/system/config-paths`                                 | `c.configPaths`                  | medium |
| system        | PUT    | `/api/system/config-paths`                                 | `c.updateConfigPaths`            | high   |
| state         | GET    | `/api/state`                                               | `a.state`                        | medium |
| search        | GET    | `/api/search`                                              | `a.searchItems`                  | medium |
| ai            | GET    | `/api/ai/capabilities`                                     | `a.aiCapabilities`               | medium |
| ai            | GET    | `/api/ai/presets`                                          | `a.aiPresets`                    | medium |
| ai            | GET    | `/api/ai/providers/{id}/capabilities`                      | `a.aiProviderCapabilities`       | medium |
| ai            | GET    | `/api/ai/settings`                                         | `a.aiSettings`                   | medium |
| ai            | PUT    | `/api/ai/settings`                                         | `a.saveAISettings`               | high   |
| workspaces    | GET    | `/api/workspaces`                                          | `a.listWorkspaces`               | medium |
| knowledge     | GET    | `/api/knowledge/wikis`                                     | `a.knowledgeWikis`               | medium |
| knowledge     | GET    | `/api/knowledge/wikis/{workspaceID}/{root}/pages`          | `a.knowledgePages`               | medium |
| knowledge     | GET    | `/api/knowledge/wikis/{workspaceID}/{root}/pages/{slug}`   | `a.knowledgePage`                | medium |
| knowledge     | GET    | `/api/knowledge/wikis/{workspaceID}/{root}/graph`          | `a.knowledgeGraph`               | medium |
| knowledge     | POST   | `/api/knowledge/wikis/{workspaceID}/{root}/rescan`         | `a.knowledgeRescan`              | high   |
| knowledge     | POST   | `/api/knowledge/workspaces/{workspaceID}/sync`             | `a.knowledgeSync`                | high   |
| knowledge     | POST   | `/api/knowledge/workspaces/{workspaceID}/enrich`           | `a.knowledgeEnrich`              | high   |
| workspaces    | POST   | `/api/workspaces`                                          | `a.createWorkspace`              | high   |
| workspaces    | POST   | `/api/workspaces/import-preview`                           | `a.previewWorkspaceImport`       | high   |
| workspaces    | POST   | `/api/workspaces/import`                                   | `a.importWorkspaces`             | high   |
| workspaces    | POST   | `/api/workspaces/stream-create`                            | `a.createWorkspaceStream`        | high   |
| workspaces    | PUT    | `/api/workspaces/{id}`                                     | `a.updateWorkspace`              | high   |
| workspaces    | DELETE | `/api/workspaces/{id}`                                     | `a.deleteWorkspace`              | high   |
| workspaces    | POST   | `/api/workspaces/{id}/scan`                                | `a.scanWorkspace`                | high   |
| workspaces    | POST   | `/api/workspaces/{id}/jira/test`                           | `a.testJiraConnection`           | high   |
| workspaces    | GET    | `/api/workspaces/{id}/jira/issues/{issueKey}`              | `a.workspaceJiraIssue`           | medium |
| workspaces    | GET    | `/api/workspaces/{id}/runtime`                             | `a.workspaceRuntime`             | medium |
| workspaces    | PUT    | `/api/workspaces/{id}/runtime`                             | `a.saveWorkspaceRuntime`         | high   |
| workspaces    | POST   | `/api/workspaces/{id}/verification-jobs`                   | `a.createVerificationJob`        | high   |
| workspaces    | POST   | `/api/workspaces/{id}/verification-checkpoints`            | `a.ingestVerificationCheckpoint` | high   |
| workspaces    | GET    | `/api/workspaces/{id}/verification-jobs/{jobId}`           | `a.verificationJob`              | high   |
| workspaces    | GET    | `/api/workspaces/{id}/verification-jobs/{jobId}/artifacts` | `a.verificationArtifacts`        | high   |
| workspaces    | POST   | `/api/workspaces/{id}/verification-jobs/{jobId}/rerun`     | `a.rerunVerificationJob`         | high   |
| workspaces    | POST   | `/api/workspaces/{id}/workstream/branch`                   | `a.loadWorkstreamBranch`         | high   |
| workspaces    | GET    | `/api/workspaces/{id}/health`                              | `a.workspaceHealth`              | medium |
| workspaces    | GET    | `/api/workspaces/{id}/source-structure`                    | `a.getSourceStructure`           | medium |
| workspaces    | PUT    | `/api/workspaces/{id}/source-structure`                    | `a.saveSourceStructure`          | high   |
| workspaces    | DELETE | `/api/workspaces/{id}/source-structure`                    | `a.resetSourceStructure`         | high   |
| workspaces    | GET    | `/api/workspaces/{id}/tree`                                | `a.workspaceTree`                | medium |
| workspaces    | GET    | `/api/workspaces/files/search`                             | `a.workspacePathSearch`          | medium |
| workspaces    | GET    | `/api/workspaces/files/content-search`                     | `a.workspaceContentSearch`       | medium |
| workspaces    | GET    | `/api/workspaces/{id}/files`                               | `a.workspaceFile`                | medium |
| workspaces    | PUT    | `/api/workspaces/{id}/files`                               | `a.saveWorkspaceFile`            | high   |
| workspaces    | POST   | `/api/workspaces/{id}/files`                               | `a.createWorkspaceFile`          | high   |
| workspaces    | POST   | `/api/workspaces/{id}/directories`                         | `a.createWorkspaceDirectory`     | high   |
| workspaces    | POST   | `/api/workspaces/{id}/paths/rename`                        | `a.renameWorkspacePath`          | high   |
| workspaces    | GET    | `/api/workspaces/{id}/files/diff`                          | `a.workspaceFileDiff`            | medium |
| workspaces    | POST   | `/api/workspaces/{id}/files/revert`                        | `a.revertWorkspaceFile`          | high   |
| workspaces    | GET    | `/api/workspaces/{id}/git/path-status`                     | `a.workspacePathGitStates`       | high   |
| items         | GET    | `/api/items`                                               | `a.listItems`                    | medium |
| items         | GET    | `/api/items/{id}`                                          | `a.itemDetail`                   | medium |
| items         | GET    | `/api/items/{id}/ai-session-eligibility`                   | `a.aiSessionEligibility`         | medium |
| items         | POST   | `/api/items/{id}/ai-sessions`                              | `a.launchAISession`              | high   |
| items         | POST   | `/api/items/{id}/ai-sessions/embedded`                     | `a.startEmbeddedAISession`       | high   |
| ai            | GET    | `/api/ai/sessions/{sessionId}`                             | `a.embeddedAISession`            | medium |
| ai            | DELETE | `/api/ai/sessions/{sessionId}`                             | `a.cancelEmbeddedAISession`      | high   |
| ai            | GET    | `/api/ai/sessions/{sessionId}/channel`                     | `a.embeddedAISessionChannel`     | high   |
| items         | GET    | `/api/items/{id}/jira`                                     | `a.jiraIssue`                    | medium |
| items         | POST   | `/api/items/{id}/jira/refresh`                             | `a.refreshJiraIssue`             | high   |
| items         | GET    | `/api/items/{id}/jira/attachments/{attachmentId}`          | `a.jiraAttachment`               | medium |
| items         | GET    | `/api/items/{id}/verification-tests`                       | `a.itemVerificationTests`        | high   |
| items         | PUT    | `/api/items/{id}/verification-tests`                       | `a.saveItemVerificationTests`    | high   |
| items         | GET    | `/api/items/{id}/files`                                    | `a.itemFiles`                    | medium |
| items         | GET    | `/api/items/{id}/content-search`                           | `a.itemContentSearch`            | medium |
| items         | GET    | `/api/items/{id}/files/{fileID}`                           | `a.itemFileContent`              | medium |
| items         | POST   | `/api/items/{id}/files/{fileID}`                           | `a.saveItemFile`                 | high   |
| items         | POST   | `/api/items/{id}/files/{fileID}/revert`                    | `a.revertItemFile`               | high   |
| items         | GET    | `/api/items/{id}/diff`                                     | `a.itemDiff`                     | medium |
| items         | PATCH  | `/api/items/{id}/metadata`                                 | `a.saveItemMetadata`             | high   |
| items         | PATCH  | `/api/items/{id}/status`                                   | `a.updateItemStatus`             | high   |
| items         | POST   | `/api/items`                                               | `a.createItem`                   | high   |
| workspaces    | GET    | `/api/workspaces/{id}/git/status`                          | `a.gitStatus`                    | high   |
| workspaces    | GET    | `/api/workspaces/{id}/git/activity`                        | `a.gitActivity`                  | high   |
| workspaces    | GET    | `/api/workspaces/{id}/git/branches`                        | `a.gitBranches`                  | high   |
| workspaces    | POST   | `/api/workspaces/{id}/git/fetch`                           | `a.gitFetch`                     | high   |
| workspaces    | POST   | `/api/workspaces/{id}/git/pull`                            | `a.gitPull`                      | high   |
| workspaces    | POST   | `/api/workspaces/{id}/git/push`                            | `a.gitPush`                      | high   |
| workspaces    | POST   | `/api/workspaces/{id}/git/commit`                          | `a.gitCommit`                    | high   |
| workspaces    | POST   | `/api/workspaces/{id}/git/branches`                        | `a.gitCreateBranch`              | high   |
| workspaces    | POST   | `/api/workspaces/{id}/git/switch`                          | `a.gitSwitchBranch`              | high   |

## Baseline Benchmark Command

Run this before and after each transport migration:

`rtk go test -bench 'Benchmark(HealthController|AuditControllerEvents)' -benchmem ./internal/workspace ./internal/audit`

## Baseline Benchmark Result

Measured on Apple M1 Pro, darwin arm64:

| Benchmark                           | Time/op          | Bytes/op | Allocs/op |
|-------------------------------------|------------------|----------|-----------|
| `BenchmarkHealthController-10`      | 795.3 ns/op      | 1424     | 14        |
| `BenchmarkAuditControllerEvents-10` | 1348138458 ns/op | 331344   | 2263      |
