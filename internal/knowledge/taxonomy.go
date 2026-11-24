package knowledge

import (
	"path"
	"strings"
)

const minTierParents = 2

type Taxonomy struct {
	Tiers   map[string]bool
	Buckets map[string]bool
	Roles   map[string]string
}

// Count parents per bucket. A subject area in two buckets would reach the threshold across the tree.
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

// Segments on both sides of a tier form the area, so a deeper layout keeps every path component.
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

func pathSegments(relativePath string) []string {
	directory := path.Dir(normalizePath(relativePath))
	directory = strings.Trim(directory, "/")
	if directory == "" || directory == "." {
		return nil
	}
	return strings.Split(directory, "/")
}

// Detection needs the finished page set, so it runs after parsing. A broken settings file keeps detection.
func applyTaxonomy(root string, pages []KnowledgePage) ([]KnowledgePage, string, []KnowledgeWarning) {
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
	return pages, journeysBucketFor(taxonomy), warnings
}

func journeysBucketFor(taxonomy Taxonomy) string {
	for bucket, role := range taxonomy.Roles {
		if role == RoleJourneys {
			return bucket
		}
	}
	return DefaultJourneysBucket
}

const DefaultJourneysBucket = "e2e-testing"

func (w KnowledgeWiki) journeysBucket() string {
	if strings.TrimSpace(w.JourneysBucket) == "" {
		return DefaultJourneysBucket
	}
	return w.JourneysBucket
}

// A page with no bucket comes from an older index. Whole segments match, so e2e-testing-archive does not.
func pageIsJourney(wiki KnowledgeWiki, page KnowledgePage) bool {
	bucket := wiki.journeysBucket()
	if page.Bucket != "" {
		return page.Bucket == bucket
	}
	return page.Domain == bucket || strings.HasPrefix(page.Domain, bucket+"/")
}
