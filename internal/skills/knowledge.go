package skills

import (
	"context"
	"fmt"
	"strings"

	"github.com/overhuman/overhuman/internal/instruments"
	"github.com/overhuman/overhuman/internal/storage"
)

// KnowledgeSearchSkill performs semantic search over the knowledge base.
type KnowledgeSearchSkill struct {
	store storage.Store
}

func NewKnowledgeSearchSkill(store storage.Store) *KnowledgeSearchSkill {
	return &KnowledgeSearchSkill{store: store}
}

func (s *KnowledgeSearchSkill) Execute(ctx context.Context, input instruments.SkillInput) (*instruments.SkillOutput, error) {
	action := input.Parameters["action"]

	switch action {
	case "search":
		return s.search(ctx, input.Parameters["query"])
	case "store":
		return s.storeDoc(ctx, input)
	case "list":
		return s.listDocs(ctx, input.Parameters["prefix"])
	case "get":
		return s.getDoc(ctx, input.Parameters["key"])
	default:
		// Default to search.
		query := input.Parameters["query"]
		if query == "" {
			query = input.Goal
		}
		return s.search(ctx, query)
	}
}

func (s *KnowledgeSearchSkill) search(ctx context.Context, query string) (*instruments.SkillOutput, error) {
	if s.store == nil {
		return &instruments.SkillOutput{Success: false, Error: "knowledge store not configured"}, nil
	}
	if query == "" {
		return &instruments.SkillOutput{Success: false, Error: "query required"}, nil
	}

	records, err := s.store.Search(ctx, query, 10)
	if err != nil {
		return &instruments.SkillOutput{Success: false, Error: err.Error()}, nil
	}

	if len(records) == 0 {
		return &instruments.SkillOutput{
			Result:  fmt.Sprintf("No results for %q", query),
			Success: true,
		}, nil
	}

	var results []string
	for i, r := range records {
		preview := string(r.Value)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		results = append(results, fmt.Sprintf("[%d] %s: %s", i+1, r.Key, preview))
	}

	return &instruments.SkillOutput{
		Result:  fmt.Sprintf("Found %d results:\n%s", len(records), strings.Join(results, "\n")),
		Success: true,
	}, nil
}

func (s *KnowledgeSearchSkill) storeDoc(ctx context.Context, input instruments.SkillInput) (*instruments.SkillOutput, error) {
	if s.store == nil {
		return &instruments.SkillOutput{Success: false, Error: "knowledge store not configured"}, nil
	}

	key := input.Parameters["key"]
	content := input.Parameters["content"]
	if key == "" || content == "" {
		return &instruments.SkillOutput{Success: false, Error: "key and content required"}, nil
	}

	err := s.store.Put(ctx, storage.Record{
		Key:   "kb:" + key,
		Value: []byte(content),
		Metadata: map[string]string{
			"type": input.Parameters["type"],
			"tags": input.Parameters["tags"],
		},
	})
	if err != nil {
		return &instruments.SkillOutput{Success: false, Error: err.Error()}, nil
	}
	return &instruments.SkillOutput{
		Result:  fmt.Sprintf("Stored document %q (%d bytes)", key, len(content)),
		Success: true,
	}, nil
}

func (s *KnowledgeSearchSkill) listDocs(ctx context.Context, prefix string) (*instruments.SkillOutput, error) {
	if s.store == nil {
		return &instruments.SkillOutput{Success: false, Error: "knowledge store not configured"}, nil
	}

	searchPrefix := "kb:"
	if prefix != "" {
		searchPrefix = "kb:" + prefix
	}

	keys, err := s.store.List(ctx, searchPrefix, 50)
	if err != nil {
		return &instruments.SkillOutput{Success: false, Error: err.Error()}, nil
	}

	var names []string
	for _, k := range keys {
		names = append(names, strings.TrimPrefix(k, "kb:"))
	}

	return &instruments.SkillOutput{
		Result:  fmt.Sprintf("%d documents:\n%s", len(names), strings.Join(names, "\n")),
		Success: true,
	}, nil
}

func (s *KnowledgeSearchSkill) getDoc(ctx context.Context, key string) (*instruments.SkillOutput, error) {
	if s.store == nil {
		return &instruments.SkillOutput{Success: false, Error: "knowledge store not configured"}, nil
	}

	rec, err := s.store.Get(ctx, "kb:"+key)
	if err != nil {
		return &instruments.SkillOutput{Success: false, Error: err.Error()}, nil
	}
	if rec == nil {
		return &instruments.SkillOutput{Success: false, Error: "document not found: " + key}, nil
	}

	return &instruments.SkillOutput{
		Result:  string(rec.Value),
		Success: true,
	}, nil
}
