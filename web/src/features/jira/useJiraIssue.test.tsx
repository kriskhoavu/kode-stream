import { renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../shared/api';
import { useJiraIssue } from './useJiraIssue';
vi.mock('../../shared/api',()=>({api:{jiraIssue:vi.fn(),refreshJiraIssue:vi.fn()}}));
describe('useJiraIssue',()=>{afterEach(()=>vi.clearAllMocks());it('clears stale data when item changes',async()=>{vi.mocked(api.jiraIssue).mockResolvedValue({state:'not_found'});const{result,rerender}=renderHook(({id})=>useJiraIssue(id),{initialProps:{id:'one'}});await waitFor(()=>expect(result.current.loading).toBe(false));rerender({id:'two'});await waitFor(()=>expect(api.jiraIssue).toHaveBeenLastCalledWith('two',expect.any(AbortSignal)));expect(api.jiraIssue).toHaveBeenCalledTimes(2);});});
