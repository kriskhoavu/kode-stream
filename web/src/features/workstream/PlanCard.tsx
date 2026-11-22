import { memo } from 'react';
import type { DragEvent, MouseEvent } from 'react';
import { FileText, Ticket } from 'lucide-react';
import { StatusMenu } from '../../components/StatusMenu';
import type { ItemStatus, ItemSummary, WorkspaceConfig } from '../../lib/types';
import { isDocumentationMetadataSource } from '../../lib/vocabulary';
import { sourceLabel } from './filtering';
import { isItemDraggable } from './dragAndDrop';
import { AgentCluster, type AgentFocus } from './AgentsPanel';
import { type ItemAgents } from './useItemSessions';
import { isSyntheticItem, type CardWork } from './harnessJoin';
import { ByHandChip, HarnessCardChips } from './HarnessCardChips';

export const PlanCard = memo(function PlanCard({ item: plan, workspace, pending, active, created, agents, work, harnessOn = false, autoStatus = false, startable = false, onRunWithHarness, onOpenTask, onDragStart, onDragEnd, onPreview, onOpen, onMove, onOpenAgent }: {
  item: ItemSummary;
  workspace?: WorkspaceConfig;
  pending: boolean;
  active: boolean;
  created: boolean;
  agents?: ItemAgents;
  /** What the harness holds for this card, when it works on it. */
  work?: CardWork;
  /** True once the board knows the harness is on, so a card the harness does not hold can say "By hand". */
  harnessOn?: boolean;
  /** True on the Tasks board, where a card's column follows its run and a person never moves it. */
  autoStatus?: boolean;
  /** A task not yet started: it may be dragged to the next column, which starts it. */
  startable?: boolean;
  /** Hands the card to the harness; offered on a card it does not hold. */
  onRunWithHarness?: () => void;
  /** Opens the harness's task behind a ticket card that no plan holds yet. */
  onOpenTask?: () => void;
  onDragStart: (event: DragEvent<HTMLElement>) => void;
  onDragEnd: () => void;
  onPreview: () => void;
  onOpen: () => void;
  onMove: (status: ItemStatus) => void;
  onOpenAgent: (focus: AgentFocus) => void;
}) {
  const source = sourceLabel(plan, workspace);
  const docs = isDocumentationMetadataSource(plan.metadataSource);
  const showItem = plan.identifier.toLowerCase() !== plan.title.toLowerCase();
  const description = plan.description;
  // A tag equal to the source badge is redundant on any card, not just a docs one:
  // the default source-structure card tags every item with its own source name, so
  // wiki items arrived as metadataSource "workspace-settings" and printed it twice.
  const tags = (docs ? plan.tags.filter((tag) => tag !== plan.scope && tag !== plan.identifier) : plan.tags).filter((tag) => tag !== source);
  const synthetic = isSyntheticItem(plan);
  // The card's key opens it where the card shows one; else its title does.
  const keyOpens = !docs && (synthetic || showItem);
  const draggable = startable || (isItemDraggable(plan) && !pending && !synthetic && !autoStatus);
  const navigate = (event: MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    if (synthetic) onOpenTask?.(); else onOpen();
  };
  const preview = () => { if (synthetic) onOpenTask?.(); else onPreview(); };
  const classes = ['plan-card'];
  if (docs) classes.push('docs-plan');
  if (work) classes.push('harness-card');
  if (work?.task.needsYou || work?.waiting.length) classes.push('needs-you-card');
  if (work?.status === 'working' || work?.status === 'starting') classes.push('working-card');
  if (synthetic) classes.push('ticket-card');
  else if (!docs) classes.push('plan-kind-card');
  if (synthetic && !work?.run) classes.push('no-plan');
  if (draggable) classes.push('draggable');
  if (active) classes.push('dragging');
  if (pending) classes.push('move-pending');
  if (created) classes.push('just-created');
  return (
    <article
      data-item-id={plan.id}
      className={classes.join(' ')}
      draggable={draggable}
      aria-busy={pending}
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      onClick={preview}
      role="button"
      tabIndex={0}
      onKeyDown={(event) => {
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault();
        preview();
      }
      }}
    >
      {!docs && (
        <div className="card-kind-row">
          <span className={synthetic ? 'card-kind ticket' : 'card-kind plan'} title={synthetic ? 'A Jira ticket the harness holds. No plan on this branch yet.' : undefined}>
            {synthetic ? <Ticket size={13} aria-hidden="true" /> : <FileText size={13} aria-hidden="true" />}
            <span>{synthetic ? 'Jira ticket' : 'Plan'}</span>
            {keyOpens && <><span aria-hidden="true">·</span><button type="button" className="card-kind-key" title={`Open ${plan.identifier}`} onPointerDown={(event) => event.stopPropagation()} onClick={navigate}>{plan.identifier}</button></>}
          </span>
          {!synthetic && <AgentCluster agents={agents} title={plan.title} onOpen={onOpenAgent} />}
        </div>
      )}
      <div className="plan-card-title">
        {keyOpens
          ? <strong className="plan-card-heading">{plan.title}</strong>
          : <button type="button" className="plan-card-link plan-card-heading" onPointerDown={(event) => event.stopPropagation()} onClick={navigate}>{plan.title}</button>}
        {docs && source && <span className="card-badges"><span className="source-badge docs">{source}</span></span>}
      </div>
      {docs && showItem && <span className="plan-card-identifier">{plan.identifier}</span>}
      {work ? <HarnessCardChips work={work} /> : harnessOn && !docs && plan.status !== 'unsorted' && (
        <div className="plan-card-harness plan-card-harness-chips">
          <ByHandChip />
          {onRunWithHarness && <button type="button" className="secondary plan-card-run" onPointerDown={(event) => event.stopPropagation()} onClick={(event) => { event.stopPropagation(); onRunWithHarness(); }}>Run with the harness</button>}
        </div>
      )}
      {description && <p>{description}</p>}
      <footer>
        <span className="avatar">{(plan.author || plan.owner || '?').slice(0, 1).toUpperCase()}</span>
        <span>{plan.author || plan.owner || 'Unknown'}</span>
        <time>{plan.updatedAt ? new Date(plan.updatedAt).toLocaleDateString() : 'No date'}</time>
      </footer>
      {tags.length > 0 && <div className="tags">{tags.slice(0, 3).map((tag: string) => <span key={tag}>{tag}</span>)}</div>}
      {plan.status !== 'unsorted' && !synthetic && !autoStatus && !isDocumentationMetadataSource(plan.metadataSource) && (
        <StatusMenu value={plan.status} onChange={onMove} ariaLabel="Move item status" />
      )}
    </article>
  );
});
