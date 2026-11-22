package models

type SourceStructureSettings struct {
	Version int                   `json:"version" yaml:"version"`
	Cards   []SourceStructureCard `json:"cards" yaml:"cards"`
}

type SourceStructureCard struct {
	PathPattern string                `json:"pathPattern" yaml:"pathPattern"`
	Fields      SourceStructureFields `json:"fields" yaml:"fields"`
}

type SourceStructureFields struct {
	Source     string   `json:"source,omitempty" yaml:"source,omitempty"`
	Item       string   `json:"item,omitempty" yaml:"item,omitempty"`
	Scope      string   `json:"scope,omitempty" yaml:"scope,omitempty"`
	Identifier string   `json:"identifier,omitempty" yaml:"identifier,omitempty"`
	Title      string   `json:"title,omitempty" yaml:"title,omitempty"`
	Status     string   `json:"status,omitempty" yaml:"status,omitempty"`
	Owner      string   `json:"owner,omitempty" yaml:"owner,omitempty"`
	Tags       []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

type SourceStructureProposal struct {
	ID         string                   `json:"id" yaml:"id"`
	Label      string                   `json:"label" yaml:"label"`
	Summary    string                   `json:"summary" yaml:"summary"`
	Confidence string                   `json:"confidence" yaml:"confidence"`
	Card       SourceStructureCard      `json:"card" yaml:"card"`
	Preview    []SourceStructurePreview `json:"preview" yaml:"preview"`
}

type SourceStructurePreview struct {
	Path       string     `json:"path" yaml:"path"`
	Source     string     `json:"source,omitempty" yaml:"source,omitempty"`
	Item       string     `json:"item,omitempty" yaml:"item,omitempty"`
	Scope      string     `json:"scope" yaml:"scope"`
	Identifier string     `json:"identifier" yaml:"identifier"`
	Title      string     `json:"title" yaml:"title"`
	Status     ItemStatus `json:"status" yaml:"status"`
	Tags       []string   `json:"tags" yaml:"tags"`
}

type SourceSettingsResult struct {
	Directory string                    `json:"directory" yaml:"directory"`
	Exists    bool                      `json:"exists" yaml:"exists"`
	Mode      string                    `json:"mode" yaml:"mode"`
	Settings  SourceStructureSettings   `json:"settings" yaml:"settings"`
	Warnings  []ScanWarning             `json:"warnings" yaml:"warnings"`
	Proposals []SourceStructureProposal `json:"proposals" yaml:"proposals"`
	Preview   []SourceStructurePreview  `json:"preview" yaml:"preview"`
}
