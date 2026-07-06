package knowledge

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

func ResolveRelationships(pages []KnowledgePage) ([]KnowledgePage, []KnowledgeWarning, KnowledgeGraph) {
	pages = append([]KnowledgePage(nil), pages...)
	sort.SliceStable(pages, func(i, j int) bool { return pages[i].Path < pages[j].Path })
	bySlug := make(map[string]int, len(pages))
	warnings := make([]KnowledgeWarning, 0)
	canonical := make([]KnowledgePage, 0, len(pages))
	for _, page := range pages {
		page.Backlinks = []string{}
		if first, exists := bySlug[page.Slug]; exists {
			warnings = append(warnings, KnowledgeWarning{Path: page.Path, Slug: page.Slug, Code: WarningDuplicateSlug, Message: fmt.Sprintf("duplicate slug; using %s", canonical[first].Path)})
		} else {
			bySlug[page.Slug] = len(canonical)
			canonical = append(canonical, page)
		}
	}
	pages = canonical
	byPath := make(map[string]int, len(pages))
	for index := range pages {
		byPath[normalizePath(pages[index].Path)] = index
	}

	edges := make(map[string]KnowledgeGraphEdge)
	for sourceIndex := range pages {
		for linkIndex := range pages[sourceIndex].Links {
			link := &pages[sourceIndex].Links[linkIndex]
			targetIndex, ok := resolveTarget(*link, pages[sourceIndex].Path, bySlug, byPath)
			if !ok {
				warnings = append(warnings, KnowledgeWarning{Path: pages[sourceIndex].Path, Slug: pages[sourceIndex].Slug, Code: WarningUnresolvedLink, Message: "unresolved link: " + link.RawTarget})
				continue
			}
			link.TargetSlug = pages[targetIndex].Slug
			link.Resolution = LinkResolved
			pages[targetIndex].Backlinks = append(pages[targetIndex].Backlinks, pages[sourceIndex].Slug)
			key := pages[sourceIndex].Slug + "\x00" + pages[targetIndex].Slug
			edges[key] = KnowledgeGraphEdge{Source: pages[sourceIndex].Slug, Target: pages[targetIndex].Slug}
		}
	}
	for index := range pages {
		pages[index].Backlinks = uniqueStrings(pages[index].Backlinks)
	}

	graphEdges := make([]KnowledgeGraphEdge, 0, len(edges))
	for _, edge := range edges {
		graphEdges = append(graphEdges, edge)
	}
	sort.Slice(graphEdges, func(i, j int) bool {
		if graphEdges[i].Source == graphEdges[j].Source {
			return graphEdges[i].Target < graphEdges[j].Target
		}
		return graphEdges[i].Source < graphEdges[j].Source
	})
	graph := buildGraph(pages, graphEdges)
	return pages, warnings, graph
}

func resolveTarget(link KnowledgeLink, sourcePath string, bySlug, byPath map[string]int) (int, bool) {
	if index, ok := bySlug[link.RawTarget]; ok {
		return index, true
	}
	target := link.RawTarget
	if strings.HasSuffix(strings.ToLower(target), ".md") {
		target = normalizePath(path.Join(path.Dir(sourcePath), target))
		index, ok := byPath[target]
		return index, ok
	}
	return 0, false
}

func buildGraph(pages []KnowledgePage, edges []KnowledgeGraphEdge) KnowledgeGraph {
	inbound := make(map[string]int)
	outbound := make(map[string]int)
	for _, edge := range edges {
		outbound[edge.Source]++
		inbound[edge.Target]++
	}
	nodes := make([]KnowledgeGraphNode, 0, len(pages))
	seen := make(map[string]struct{})
	for _, page := range pages {
		if _, ok := seen[page.Slug]; ok {
			continue
		}
		seen[page.Slug] = struct{}{}
		nodes = append(nodes, KnowledgeGraphNode{ID: page.Slug, Title: page.Title, Domain: page.Domain, PageType: page.PageType, Roles: page.Roles, Topics: page.Topics, Path: page.Path, Inbound: inbound[page.Slug], Outbound: outbound[page.Slug]})
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Domain != nodes[j].Domain {
			return nodes[i].Domain < nodes[j].Domain
		}
		if nodes[i].Title != nodes[j].Title {
			return nodes[i].Title < nodes[j].Title
		}
		return nodes[i].ID < nodes[j].ID
	})
	return KnowledgeGraph{Nodes: nodes, Edges: edges, TotalNodes: len(nodes), TotalEdges: len(edges)}
}
