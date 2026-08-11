import type { SourceSettingsResult, SourceStructureCard, SourceStructurePreview, SourceStructureProposal } from '../../lib/types';
import { lastPathSegment } from './sourceSettings';

export const UNSORTED_SOURCE_SELECTION_ID = 'unsorted';

export type SourceSettingsEditorModel = {
  directory: string;
  exists: boolean;
  mode?: string;
  card: SourceStructureCard;
  cards: SourceStructureCard[];
  warnings: string[];
  proposals: SourceStructureProposal[];
  selectedProposalId?: string;
  unsortedPreview: SourceStructurePreview[];
  preview: SourceStructurePreview[];
};

export function sourceSettingsEditorFromResult(directory: string, result: SourceSettingsResult): SourceSettingsEditorModel {
  const proposals = result.proposals ?? [];
  const selectedProposal = !result.exists && proposals.length > 0 ? proposals[0] : undefined;
  const unsortedPreview = [unsortedSourcePreview(directory)];
  const selectedProposalId = selectedProposal?.id ?? (!result.exists ? UNSORTED_SOURCE_SELECTION_ID : undefined);
  const cards = (result.settings?.cards?.length ? result.settings.cards : [selectedProposal?.card])
    .filter((card): card is SourceStructureCard => Boolean(card))
    .map((card) => normalizeSourceSettingsCard(card, directory));
  const activeCard = normalizeSourceSettingsCard(selectedProposal?.card ?? cards[0], directory);
  return {
    directory,
    exists: result.exists,
    mode: result.mode,
    card: activeCard,
    cards: cards.length > 0 ? [activeCard, ...cards.slice(1)] : [activeCard],
    warnings: (result.warnings ?? []).map((warning) => warning.message),
    proposals,
    selectedProposalId,
    unsortedPreview,
    preview: selectedProposal?.preview ?? (!result.exists ? unsortedPreview : result.preview ?? [])
  };
}

export function normalizeSourceSettingsCard(card?: SourceStructureCard, directory = 'source'): SourceStructureCard {
  return canonicalSourceSettingsCard({
    pathPattern: canonicalTemplate(card?.pathPattern || '{folder}/feature/{item}'),
    fields: {
      source: canonicalTemplate(card?.fields?.source || directory),
      item: canonicalTemplate(card?.fields?.item || '{item}'),
      title: card?.fields?.title || 'readme_heading',
      status: card?.fields?.status || 'draft',
      owner: card?.fields?.owner || '',
      tags: Array.isArray(card?.fields?.tags) ? card.fields.tags : [lastPathSegment(directory) || 'source']
    }
  });
}

export function canonicalSourceSettingsCard(card: SourceStructureCard): SourceStructureCard {
  return {
    ...card,
    fields: { ...card.fields, source: card.fields.source, item: card.fields.item }
  };
}

function canonicalTemplate(value: string): string {
  return value
    .replaceAll('{service}', '{folder}')
    .replaceAll('{scope}', '{folder}')
    .replaceAll('{ticket}', '{item}')
    .replaceAll('{identifier}', '{item}');
}

function unsortedSourcePreview(directory: string): SourceStructurePreview {
  const sourceName = lastPathSegment(directory) || 'source';
  return {
    path: directory,
    source: sourceName,
    item: sourceName,
    scope: sourceName,
    identifier: sourceName,
    title: sourceName,
    status: 'unsorted',
    tags: [sourceName]
  };
}
