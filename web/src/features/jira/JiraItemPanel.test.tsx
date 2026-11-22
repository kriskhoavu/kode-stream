import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { api } from '../../shared/api';
import { JiraItemPanel } from './JiraItemPanel';

vi.mock('../../shared/api', () => ({ api: { jiraIssue: vi.fn(), refreshJiraIssue: vi.fn(), workspaceJiraIssue: vi.fn(), refreshWorkspaceJiraIssue: vi.fn(), jiraAttachmentURL: (itemId:string,id:string)=>`/api/items/${itemId}/jira/attachments/${id}`, workspaceJiraAttachmentURL: (workspaceId:string,key:string,id:string)=>`/api/workspaces/${workspaceId}/jira/issues/${key}/attachments/${id}` } }));

describe('JiraItemPanel', () => {
  afterEach(() => vi.clearAllMocks());
  it('keeps attachments collapsed until requested', async () => {
    vi.mocked(api.jiraIssue).mockResolvedValue({ state:'available', issue:{ key:'DI-170', summary:'Search', status:'Open', description:'Remote <script>alert(1)</script>', issueType:'Story', priority:'High', labels:['backend'], browserUrl:'https://jira/browse/DI-170', attachments:[{id:'9',filename:'spec.pdf',mediaType:'application/pdf',sizeBytes:2048,author:{displayName:'Kim'}}] } });
    render(<JiraItemPanel itemId="item-1" />);
    expect(await screen.findByText('DI-170')).toBeInTheDocument();
    // The description is rendered Markdown, so the sanitizer strips the script
    // element and leaves its content as inert text.
    expect(await screen.findByText('Remote alert(1)')).toBeInTheDocument();
    expect(document.querySelector('script')).toBeNull();
    expect(screen.queryByRole('link',{name:'Open spec.pdf'})).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /Attachments/ }));
    expect(screen.getByRole('link',{name:'Open spec.pdf'})).toHaveAttribute('href','/api/items/item-1/jira/attachments/9');
  });
  it('reads a ticket by key in a workspace, with attachments served from that route', async () => {
    vi.mocked(api.workspaceJiraIssue).mockResolvedValue({ state:'available', issue:{ key:'APP-562', summary:'Articles not shown', status:'In Progress', statusCategory:'indeterminate', description:'Surcharge rows vanish.', issueType:'Bug', labels:[], browserUrl:'https://jira/browse/APP-562', attachments:[{id:'3',filename:'offer.pdf',mediaType:'application/pdf',sizeBytes:100,author:{displayName:'Kim'}}] } });
    render(<JiraItemPanel workspaceId="ws-1" issueKey="APP-562" />);
    expect(await screen.findByText('Articles not shown')).toBeInTheDocument();
    expect(api.workspaceJiraIssue).toHaveBeenCalledWith('ws-1', 'APP-562', expect.any(AbortSignal));
    expect(api.jiraIssue).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: /Attachments/ }));
    expect(screen.getByRole('link',{name:'Open offer.pdf'})).toHaveAttribute('href','/api/workspaces/ws-1/jira/issues/APP-562/attachments/3');
    fireEvent.click(screen.getByRole('button', { name: 'Refresh Jira ticket' }));
    await waitFor(() => expect(api.refreshWorkspaceJiraIssue).toHaveBeenCalledWith('ws-1', 'APP-562', expect.any(AbortSignal)));
  });
  it('loads image thumbnails only after attachments expand', async () => {
    vi.mocked(api.jiraIssue).mockResolvedValue({ state:'available', issue:{ key:'DI-171', summary:'Screenshot', status:'Open', description:'', issueType:'Bug', labels:[], browserUrl:'https://jira/browse/DI-171', attachments:[{id:'10',filename:'screen.png',mediaType:'image/png',sizeBytes:4096,author:{displayName:'Kim'}}] } });
    render(<JiraItemPanel itemId="item-2" />);
    expect(await screen.findByText('DI-171')).toBeInTheDocument();
    expect(screen.queryByRole('img', { name: 'screen.png' })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /Attachments/ }));
    const thumbnail = screen.getByRole('img', { name: 'screen.png' });
    expect(thumbnail).toHaveAttribute('src', '/api/items/item-2/jira/attachments/10');
    expect(screen.getByRole('link', { name: 'View screen.png' })).toHaveAttribute('target', '_blank');
  });
  it('renders typed absence and refreshes without discarding the panel', async () => {
    vi.mocked(api.jiraIssue).mockResolvedValue({state:'not_found',message:'No Jira ticket exists for this item'});
    vi.mocked(api.refreshJiraIssue).mockResolvedValue({state:'unavailable',message:'Jira offline',recoveryHint:'Try later'});
    render(<JiraItemPanel itemId="item-1" />);
    expect(await screen.findByText('No Jira ticket')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button',{name:/refresh/i}));
    await waitFor(()=>expect(api.refreshJiraIssue).toHaveBeenCalledWith('item-1', expect.any(AbortSignal)));
    expect(await screen.findByText('Jira offline')).toBeInTheDocument();
  });
  it('isolates request failures', async () => {
    vi.mocked(api.jiraIssue).mockRejectedValue(new Error('Network failed'));
    render(<JiraItemPanel itemId="item-1" />);
    expect(await screen.findByRole('alert')).toHaveTextContent('Network failed');
  });
});

describe('JiraItemPanel status badge', () => {
  afterEach(() => vi.clearAllMocks());
  const tone = async (statusCategory: string | undefined, status: string) => {
    vi.mocked(api.jiraIssue).mockResolvedValue({ state:'available', issue:{ key:'DI-365', summary:'S', status, statusCategory, description:'', issueType:'User-Story', labels:[], browserUrl:'https://jira/browse/DI-365', attachments:[] } });
    const view = render(<JiraItemPanel itemId="item-4" />);
    const badge = await screen.findByText(status);
    const className = badge.className;
    view.unmount();
    return className;
  };

  it('colours the badge from the Jira status category, not the status name', async () => {
    expect(await tone('done', 'Test Done')).toContain('done');
    expect(await tone('indeterminate', 'QA')).toContain('in-progress');
    expect(await tone('new', 'Backlog')).toContain('todo');
    // An older payload without a category must still render a readable badge.
    expect(await tone(undefined, 'Something Custom')).toContain('unknown');
  });

  it('lists the status alongside the other fields', async () => {
    vi.mocked(api.jiraIssue).mockResolvedValue({ state:'available', issue:{ key:'DI-365', summary:'S', status:'Test Done', statusCategory:'done', description:'', issueType:'User-Story', priority:'Medium', labels:[], browserUrl:'https://jira/browse/DI-365', attachments:[] } });
    render(<JiraItemPanel itemId="item-5" />);
    const terms = (await screen.findAllByRole('term')).map((node) => node.textContent);
    expect(terms).toEqual(['Status', 'Type', 'Priority', 'Assignee', 'Reporter', 'Updated']);
  });
});
