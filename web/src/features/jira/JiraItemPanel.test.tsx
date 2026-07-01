import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../lib/api';
import { JiraItemPanel } from './JiraItemPanel';

vi.mock('../../lib/api', () => ({ api: { jiraIssue: vi.fn(), refreshJiraIssue: vi.fn(), jiraAttachmentURL: (itemId:string,id:string)=>`/api/items/${itemId}/jira/attachments/${id}` } }));

describe('JiraItemPanel', () => {
  afterEach(() => vi.clearAllMocks());
  it('renders normalized text and metadata-only attachment actions', async () => {
    vi.mocked(api.jiraIssue).mockResolvedValue({ state:'available', issue:{ key:'DI-170', summary:'Search', status:'Open', description:'Remote <script>alert(1)</script>', issueType:'Story', priority:'High', labels:['backend'], browserUrl:'https://jira/browse/DI-170', attachments:[{id:'9',filename:'spec.pdf',mediaType:'application/pdf',sizeBytes:2048,author:{displayName:'Kim'}}] } });
    render(<JiraItemPanel itemId="item-1" />);
    expect(await screen.findByText('DI-170')).toBeInTheDocument();
    expect(screen.getByText('Remote <script>alert(1)</script>')).toBeInTheDocument();
    expect(document.querySelector('script')).toBeNull();
    expect(screen.getByRole('link',{name:'Open spec.pdf'})).toHaveAttribute('href','/api/items/item-1/jira/attachments/9');
  });
  it('renders typed absence and refreshes without discarding the panel', async () => {
    vi.mocked(api.jiraIssue).mockResolvedValue({state:'not_found',message:'No Jira ticket exists for this item'});
    vi.mocked(api.refreshJiraIssue).mockResolvedValue({state:'unavailable',message:'Jira offline',recoveryHint:'Try later'});
    render(<JiraItemPanel itemId="item-1" />);
    expect(await screen.findByText('No Jira ticket')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button',{name:/refresh/i}));
    await waitFor(()=>expect(api.refreshJiraIssue).toHaveBeenCalledWith('item-1'));
    expect(await screen.findByText('Jira offline')).toBeInTheDocument();
  });
  it('isolates request failures', async () => {
    vi.mocked(api.jiraIssue).mockRejectedValue(new Error('Network failed'));
    render(<JiraItemPanel itemId="item-1" />);
    expect(await screen.findByRole('alert')).toHaveTextContent('Network failed');
  });
});
