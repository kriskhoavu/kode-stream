import { type ItemStatus } from './common';
import { type ScanResult } from './item';

export interface SourceStructureSettings {
  version: number;
  cards: SourceStructureCard[];
}

export interface SourceStructureCard {
  pathPattern: string;
  fields: SourceStructureFields;
}

export interface SourceStructureFields {
  source?: string;
  item?: string;
  title?: string;
  status?: string;
  owner?: string;
  tags?: string[];
}

export interface SourceStructureProposal {
  id: string;
  label: string;
  summary: string;
  confidence: 'high' | 'medium' | 'low' | string;
  card: SourceStructureCard;
  preview: SourceStructurePreview[];
}

export interface SourceStructurePreview {
  path: string;
  source?: string;
  item?: string;
  scope: string;
  identifier: string;
  title: string;
  status: ItemStatus;
  tags: string[];
}

export interface SourceSettingsResult {
  directory: string;
  exists: boolean;
  mode?: 'structured' | 'unstructured' | 'empty' | 'unknown';
  settings: SourceStructureSettings;
  warnings: { itemPath?: string; message: string }[];
  proposals?: SourceStructureProposal[];
  preview?: SourceStructurePreview[];
  scan?: ScanResult;
}
