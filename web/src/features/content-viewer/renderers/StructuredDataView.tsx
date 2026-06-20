import { useMemo } from 'react';
import { parse as parseYAML } from 'yaml';
import { TreeNode } from '../components/TreeNode';
import type { StructuredValue } from '../components/TreeNode';

const maximumStructuredNodes = 5_000;
const maximumStructuredDepth = 100;

export function parseStructuredContent(content: string, language: 'json' | 'yaml'): StructuredValue {
  if (!content.trim()) return {};
  const parsed: unknown = language === 'json'
    ? JSON.parse(content)
    : parseYAML(content, { maxAliasCount: 50, customTags: [] });
  return normalizeValue(parsed, new WeakSet<object>(), { count: 0 }, 0);
}

export function StructuredDataView({ content, language }: { content: string; language: 'json' | 'yaml' }) {
  const result = useMemo(() => {
    try {
      return { value: parseStructuredContent(content, language), error: '' };
    } catch (error) {
      return { value: null, error: error instanceof Error ? error.message : `${language.toUpperCase()} could not be parsed.` };
    }
  }, [content, language]);

  if (result.error) {
    return (
      <div className="viewer-error" role="alert">
        <strong>{language.toUpperCase()} could not be parsed.</strong>
        <span>{result.error}</span>
      </div>
    );
  }
  return <div className="structured-tree"><TreeNode value={result.value as StructuredValue} /></div>;
}

function normalizeValue(value: unknown, seen: WeakSet<object>, state: { count: number }, depth: number): StructuredValue {
  state.count += 1;
  if (state.count > maximumStructuredNodes) throw new Error(`Structured view is limited to ${maximumStructuredNodes} nodes.`);
  if (depth > maximumStructuredDepth) throw new Error(`Structured view is limited to ${maximumStructuredDepth} levels.`);
  if (value === null || typeof value === 'string' || typeof value === 'boolean') return value;
  if (typeof value === 'number') return Number.isFinite(value) ? value : String(value);
  if (typeof value !== 'object') return String(value);
  if (seen.has(value)) throw new Error('Circular or repeated aliases are not supported.');
  seen.add(value);

  if (Array.isArray(value)) return value.map((item) => normalizeValue(item, seen, state, depth + 1));
  const record: { [key: string]: StructuredValue } = {};
  for (const [key, child] of Object.entries(value)) record[key] = normalizeValue(child, seen, state, depth + 1);
  return record;
}
