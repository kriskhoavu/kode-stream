package knowledge

import (
	"time"

	"kode-stream/internal/common/models"
)

type LinkResolution string

const (
	LinkResolved   LinkResolution = "resolved"
	LinkUnresolved LinkResolution = "unresolved"
)

type WarningCode string

const (
	WarningInvalidFrontMatter WarningCode = "invalid_front_matter"
	WarningMissingIdentity    WarningCode = "missing_identity"
	WarningDuplicateSlug      WarningCode = "duplicate_slug"
	WarningUnresolvedLink     WarningCode = "unresolved_link"
	WarningInvalidMetadata    WarningCode = "invalid_metadata"
)

type KnowledgeWiki struct {
	WorkspaceID string             `json:"workspaceId" yaml:"workspaceId"`
	Root        string             `json:"root" yaml:"root"`
	DisplayName string             `json:"displayName" yaml:"displayName"`
	Pages       []KnowledgePage    `json:"pages" yaml:"pages"`
	Warnings    []KnowledgeWarning `json:"warnings" yaml:"warnings"`
	IndexedAt   time.Time          `json:"indexedAt" yaml:"indexedAt"`
}

type KnowledgePage struct {
	Slug        string          `json:"slug" yaml:"slug"`
	Title       string          `json:"title" yaml:"title"`
	Path        string          `json:"path" yaml:"path"`
	Domain      string          `json:"domain" yaml:"domain"`
	PageType    string          `json:"pageType,omitempty" yaml:"pageType,omitempty"`
	Roles       []string        `json:"roles" yaml:"roles"`
	Topics      []string        `json:"topics" yaml:"topics"`
	Summary     string          `json:"summary,omitempty" yaml:"summary,omitempty"`
	SourceRefs  []string        `json:"sourceRefs" yaml:"sourceRefs"`
	SourceCount int             `json:"sourceCount,omitempty" yaml:"sourceCount,omitempty"`
	Links       []KnowledgeLink `json:"links" yaml:"links"`
	Backlinks   []string        `json:"backlinks" yaml:"backlinks"`
}

type KnowledgePageDetail struct {
	KnowledgePage
	Content  models.FileContent `json:"content" yaml:"content"`
	Warnings []KnowledgeWarning `json:"warnings" yaml:"warnings"`
}

type KnowledgeLink struct {
	SourceSlug string         `json:"sourceSlug" yaml:"sourceSlug"`
	RawTarget  string         `json:"rawTarget" yaml:"rawTarget"`
	Label      string         `json:"label,omitempty" yaml:"label,omitempty"`
	TargetSlug string         `json:"targetSlug,omitempty" yaml:"targetSlug,omitempty"`
	Resolution LinkResolution `json:"resolution" yaml:"resolution"`
}

type KnowledgeWarning struct {
	WorkspaceID string      `json:"workspaceId,omitempty" yaml:"workspaceId,omitempty"`
	WikiRoot    string      `json:"wikiRoot,omitempty" yaml:"wikiRoot,omitempty"`
	Path        string      `json:"path,omitempty" yaml:"path,omitempty"`
	Slug        string      `json:"slug,omitempty" yaml:"slug,omitempty"`
	Code        WarningCode `json:"code" yaml:"code"`
	Message     string      `json:"message" yaml:"message"`
}

type KnowledgeGraph struct {
	Nodes      []KnowledgeGraphNode `json:"nodes" yaml:"nodes"`
	Edges      []KnowledgeGraphEdge `json:"edges" yaml:"edges"`
	TotalNodes int                  `json:"totalNodes" yaml:"totalNodes"`
	TotalEdges int                  `json:"totalEdges" yaml:"totalEdges"`
	Truncated  bool                 `json:"truncated" yaml:"truncated"`
}

type KnowledgeGraphNode struct {
	ID       string   `json:"id" yaml:"id"`
	Title    string   `json:"title" yaml:"title"`
	Domain   string   `json:"domain" yaml:"domain"`
	PageType string   `json:"pageType,omitempty" yaml:"pageType,omitempty"`
	Roles    []string `json:"roles" yaml:"roles"`
	Topics   []string `json:"topics" yaml:"topics"`
	Path     string   `json:"path" yaml:"path"`
	Inbound  int      `json:"inbound" yaml:"inbound"`
	Outbound int      `json:"outbound" yaml:"outbound"`
}

type KnowledgeGraphEdge struct {
	Source string `json:"source" yaml:"source"`
	Target string `json:"target" yaml:"target"`
}

type KnowledgeActionResult struct {
	OK           bool               `json:"ok" yaml:"ok"`
	Operation    string             `json:"operation" yaml:"operation"`
	Message      string             `json:"message,omitempty" yaml:"message,omitempty"`
	Wikis        []KnowledgeWiki    `json:"wikis" yaml:"wikis"`
	Warnings     []KnowledgeWarning `json:"warnings" yaml:"warnings"`
	Log          string             `json:"log,omitempty" yaml:"log,omitempty"`
	LogTruncated bool               `json:"logTruncated" yaml:"logTruncated"`
	CompletedAt  time.Time          `json:"completedAt" yaml:"completedAt"`
}
