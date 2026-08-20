package knowledge

import (
	"path"
	"strings"
)

// minTierParents is the number of distinct parent directories a name must
// appear under, inside a single bucket, to be recognised as a tier.
const minTierParents = 2

// Taxonomy describes the structural shape of one Wiki Root: which directory
// names are tiers, which buckets exist, and what each bucket means.
type Taxonomy struct {
	Tiers   map[string]bool
	Buckets map[string]bool
	Roles   map[string]string
}

// DetectTaxonomy infers tier names from the layout of the pages already
// indexed. A directory name is a tier when, inside at least one bucket, it
// appears under two or more distinct parents.
//
// Counting parents within a bucket rather than across the whole tree is
// load-bearing: a subject area appearing in two buckets reaches the threshold
// globally and would be mistaken for a tier.
func DetectTaxonomy(paths []string) Taxonomy {
	parentsByBucket := make(map[string]map[string]map[string]struct{})
	buckets := make(map[string]bool)
	for _, relativePath := range paths {
		segments := pathSegments(relativePath)
		if len(segments) == 0 {
			continue
		}
		bucket := segments[0]
		buckets[bucket] = true
		byName, ok := parentsByBucket[bucket]
		if !ok {
			byName = make(map[string]map[string]struct{})
			parentsByBucket[bucket] = byName
		}
		for index := 1; index < len(segments); index++ {
			name := segments[index]
			parents, ok := byName[name]
			if !ok {
				parents = make(map[string]struct{})
				byName[name] = parents
			}
			parents[strings.Join(segments[:index], "/")] = struct{}{}
		}
	}
	tiers := make(map[string]bool)
	for _, byName := range parentsByBucket {
		for name, parents := range byName {
			if len(parents) >= minTierParents {
				tiers[name] = true
			}
		}
	}
	return Taxonomy{Tiers: tiers, Buckets: buckets, Roles: make(map[string]string)}
}

// Classify resolves a page path into its bucket, area, and tier. Segments on
// either side of a recognised tier form the area, so a layout deeper than the
// taxonomy expects yields a longer area rather than a dropped path component.
func (t Taxonomy) Classify(relativePath string) (bucket, area, tier string) {
	segments := pathSegments(relativePath)
	if len(segments) == 0 {
		return "", "", ""
	}
	bucket = segments[0]
	areaSegments := make([]string, 0, len(segments)-1)
	for _, segment := range segments[1:] {
		if tier == "" && t.Tiers[segment] {
			tier = segment
			continue
		}
		areaSegments = append(areaSegments, segment)
	}
	return bucket, strings.Join(areaSegments, "/"), tier
}

// pathSegments returns the directory segments of a page path, dropping the file
// name. A page at the Wiki Root has none.
func pathSegments(relativePath string) []string {
	directory := path.Dir(normalizePath(relativePath))
	directory = strings.Trim(directory, "/")
	if directory == "" || directory == "." {
		return nil
	}
	return strings.Split(directory, "/")
}

// applyTaxonomy classifies every page in one Wiki Root. Detection needs the
// finished page set, so it runs here rather than per file during parsing.
//
// A settings file that could not be used leaves the detected taxonomy in place,
// so a broken override costs the wiki its overrides and nothing else.
func applyTaxonomy(root string, pages []KnowledgePage) ([]KnowledgePage, []KnowledgeWarning) {
	paths := make([]string, 0, len(pages))
	for _, page := range pages {
		paths = append(paths, page.Path)
	}
	taxonomy := DetectTaxonomy(paths)
	settings, present, warnings := ReadTaxonomySettings(root)
	if present {
		warnings = append(warnings, ValidateTaxonomyBuckets(taxonomy, settings)...)
		taxonomy = ApplyTaxonomySettings(taxonomy, settings)
	}
	for index := range pages {
		pages[index].Bucket, pages[index].Area, pages[index].Tier = taxonomy.Classify(pages[index].Path)
	}
	return pages, warnings
}
