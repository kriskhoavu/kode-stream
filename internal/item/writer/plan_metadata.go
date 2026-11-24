package itemwriter

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"kode-stream/internal/common/models"
	"kode-stream/internal/filesystem/atomicfile"
	gitadapter "kode-stream/internal/git"
	"kode-stream/internal/workspace/scanner"
)

type planYAML struct {
	Plan              planFields                       `yaml:"plan"`
	Documents         []models.ItemDocument            `yaml:"documents,omitempty"`
	AutomationTests   []models.AutomationTestPath      `yaml:"automation-test,omitempty"`
	VerificationTests models.VerificationTestSelection `yaml:"verificationTests,omitempty"`
	Extra             map[string]any                   `yaml:",inline"`
	raw               *yaml.Node
	revision          string
}

type planFields struct {
	Identifier string         `yaml:"identifier,omitempty"`
	Ticket     string         `yaml:"ticket,omitempty"`
	Title      string         `yaml:"title,omitempty"`
	Scope      string         `yaml:"scope,omitempty"`
	Service    string         `yaml:"service,omitempty"`
	Status     string         `yaml:"status,omitempty"`
	Owner      string         `yaml:"owner,omitempty"`
	Tags       []string       `yaml:"tags,omitempty"`
	Extra      map[string]any `yaml:",inline"`
}

func readPlanMetadata(workspace models.WorkspaceConfig, item models.ItemDetail) (planYAML, error) {
	var meta planYAML
	var data []byte
	var err error
	if item.SourceMode == "snapshot" {
		commit := strings.TrimSpace(item.Commit)
		if commit == "" {
			return meta, fmt.Errorf("snapshot commit is missing")
		}
		data, err = gitadapter.New().TreeReadFile(workspace.Path, commit, filepath.ToSlash(filepath.Join(item.ItemPath, "plan.yaml")))
		if err != nil && snapshotMetadataPathMissing(err) {
			meta.Documents = item.Documents
			meta.revision = planRevision(nil)
			return meta, nil
		}
	} else {
		root, pathErr := safeItemPath(workspace, item)
		if pathErr != nil {
			return meta, pathErr
		}
		data, err = os.ReadFile(filepath.Join(root, "plan.yaml"))
		if os.IsNotExist(err) {
			meta.Documents = item.Documents
			meta.revision = planRevision(nil)
			return meta, nil
		}
	}
	if err != nil {
		return meta, err
	}
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return meta, err
	}
	meta.revision = planRevision(data)
	var raw yaml.Node
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return meta, err
	}
	meta.raw = &raw
	if meta.Plan.Identifier == "" {
		meta.Plan.Identifier = meta.Plan.Ticket
	}
	if meta.Plan.Scope == "" {
		meta.Plan.Scope = meta.Plan.Service
	}
	return meta, nil
}

func planRevision(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func snapshotMetadataPathMissing(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "does not exist in") || strings.Contains(message, "exists on disk, but not in") || strings.Contains(message, "unknown revision or path")
}

func applyMetadata(meta *planYAML, item models.ItemDetail, input models.ItemMetadataUpdateInput) {
	if meta.Plan.Identifier == "" {
		meta.Plan.Identifier = item.Identifier
	}
	if meta.Plan.Title == "" {
		meta.Plan.Title = item.Title
	}
	if meta.Plan.Scope == "" {
		meta.Plan.Scope = item.Scope
	}
	if meta.Plan.Status == "" {
		meta.Plan.Status = string(item.Status)
	}
	if input.Identifier != "" {
		meta.Plan.Identifier = strings.TrimSpace(input.Identifier)
	}
	if input.Title != "" {
		meta.Plan.Title = strings.TrimSpace(input.Title)
	}
	if input.Scope != "" {
		meta.Plan.Scope = strings.TrimSpace(input.Scope)
	}
	if input.Status != "" {
		meta.Plan.Status = string(input.Status)
	}
	if input.Owner != "" {
		meta.Plan.Owner = strings.TrimSpace(input.Owner)
	}
	if input.Tags != nil {
		meta.Plan.Tags = cleanTags(input.Tags)
	}
	if len(meta.Documents) == 0 {
		meta.Documents = item.Documents
	}
}

func writePlanMetadata(workspace models.WorkspaceConfig, item models.ItemDetail, meta planYAML) error {
	root, err := safeItemPath(workspace, item)
	if err != nil {
		return err
	}
	return writePlanMetadataAt(root, meta)
}

func writePlanMetadataAt(root string, meta planYAML) error {
	var data []byte
	var err error
	if meta.raw != nil {
		mergePlanMetadataNode(meta.raw, meta)
		data, err = yaml.Marshal(meta.raw)
	} else {
		compactPlanMetadata(root, &meta)
		data, err = yaml.Marshal(meta)
	}
	if err != nil {
		return err
	}
	return atomicWritePlan(filepath.Join(root, "plan.yaml"), data)
}

func mergePlanMetadataNode(document *yaml.Node, meta planYAML) {
	root := document
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return
	}
	plan := mappingValue(root, "plan")
	if plan == nil || plan.Kind != yaml.MappingNode {
		plan = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		setMappingValue(root, "plan", plan)
	}
	setScalarIfNotEmpty(plan, "identifier", meta.Plan.Identifier)
	setScalarIfNotEmpty(plan, "ticket", meta.Plan.Ticket)
	setScalarIfNotEmpty(plan, "title", meta.Plan.Title)
	setScalarIfNotEmpty(plan, "scope", meta.Plan.Scope)
	setScalarIfNotEmpty(plan, "service", meta.Plan.Service)
	setScalarIfNotEmpty(plan, "status", meta.Plan.Status)
	setScalarIfNotEmpty(plan, "owner", meta.Plan.Owner)
	if meta.Plan.Tags != nil {
		setYAMLValue(plan, "tags", meta.Plan.Tags)
	}
	if len(meta.VerificationTests.SelectedSpecs) > 0 || meta.VerificationTests.Environment != "" || meta.VerificationTests.DisplayMode != "" {
		setYAMLValue(root, "verificationTests", meta.VerificationTests)
	}
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func setMappingValue(node *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = value
			return
		}
	}
	node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
}

func setScalarIfNotEmpty(node *yaml.Node, key, value string) {
	if value != "" {
		setMappingValue(node, key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
	}
}

func setYAMLValue(node *yaml.Node, key string, value any) {
	var encoded yaml.Node
	if err := encoded.Encode(value); err == nil {
		setMappingValue(node, key, &encoded)
	}
}

// Plan files are user source documents, so they are only ever replaced atomically.
func atomicWritePlan(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	return atomicfile.Replace(path, ".plan.yaml-*", data, mode)
}

func compactPlanMetadata(root string, meta *planYAML) {
	identifier := strings.TrimSpace(meta.Plan.Identifier)
	if identifier == "" {
		identifier = filepath.Base(root)
	}
	if strings.EqualFold(identifier, filepath.Base(root)) {
		meta.Plan.Identifier = ""
	}
	if strings.EqualFold(strings.TrimSpace(meta.Plan.Scope), filepath.Base(filepath.Dir(root))) {
		meta.Plan.Scope = ""
	}
	if inferredTitle := scanner.InferPlanTitle(root, identifier); inferredTitle != "" && meta.Plan.Title == inferredTitle {
		meta.Plan.Title = ""
	}
	// ticket and service are legacy aliases. Keep them so a save never rewrites a legacy plan lossily.
	meta.Documents = compactDocumentOverrides(root, meta.Documents)
	if len(meta.VerificationTests.SelectedSpecs) == 0 && strings.TrimSpace(meta.VerificationTests.Environment) == "" && meta.VerificationTests.DisplayMode == "" {
		meta.VerificationTests = models.VerificationTestSelection{}
	}
}

func compactDocumentOverrides(root string, documents []models.ItemDocument) []models.ItemDocument {
	inferred := scanner.InferDocuments(root)
	byPath := make(map[string]models.ItemDocument, len(inferred))
	for _, doc := range inferred {
		byPath[filepath.ToSlash(doc.Path)] = doc
	}
	overrides := make([]models.ItemDocument, 0, len(documents))
	for _, doc := range documents {
		doc.Path = filepath.ToSlash(strings.TrimSpace(doc.Path))
		base, found := byPath[doc.Path]
		if !found {
			overrides = append(overrides, doc)
			continue
		}
		override := models.ItemDocument{Path: doc.Path}
		if doc.Role != "" && doc.Role != base.Role {
			override.Role = doc.Role
		}
		if doc.Track != "" && doc.Track != base.Track {
			override.Track = doc.Track
		}
		if doc.Label != "" && doc.Label != base.Label {
			override.Label = doc.Label
		}
		if override.Role != "" || override.Track != "" || override.Label != "" {
			overrides = append(overrides, override)
		}
	}
	return overrides
}
