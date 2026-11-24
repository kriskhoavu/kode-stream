// Both Knowledge views share these rules, so the tree and the graph show one hierarchy.

export interface TaxonomyCarrier {
	domain: string;
	bucket?: string;
	area?: string;
	tier?: string;
}

export interface Grouping {
	bucket: string;
	area: string;
	tier: string;
}

export const rootGroupKey = 'root';

// An index without the taxonomy has only `domain`, so bucket and area derive from it.
export function groupingOf(node: TaxonomyCarrier): Grouping {
	if (node.bucket !== undefined || node.area !== undefined || node.tier !== undefined) {
		return { bucket: node.bucket ?? '', area: node.area ?? '', tier: node.tier ?? '' };
	}
	const parts = (node.domain === rootGroupKey ? '' : node.domain).split('/').filter(Boolean);
	return { bucket: parts[0] ?? '', area: parts.slice(1).join('/'), tier: '' };
}

export function bucketKey(grouping: Grouping): string {
	return grouping.bucket || rootGroupKey;
}

export function areaKey(grouping: Grouping): string | undefined {
	return grouping.area ? `${bucketKey(grouping)}/${grouping.area}` : undefined;
}

export function formatSegment(value: string): string {
	return value.split(/[/_-]/).filter(Boolean).map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(' ') || 'Other';
}

export function formatArea(area: string): string {
	return area.split('/').filter(Boolean).map(formatSegment).join(' / ') || 'Other';
}
