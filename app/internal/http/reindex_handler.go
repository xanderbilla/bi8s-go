package http

import (
	"log/slog"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

// ReindexHandler handles admin reindex operations.
type ReindexHandler struct {
	contentService *service.ContentService
	personService  *service.PersonService
	searchService  *service.SearchService
}

// NewReindexHandler constructs a ReindexHandler.
func NewReindexHandler(
	contentService *service.ContentService,
	personService *service.PersonService,
	searchService *service.SearchService,
) *ReindexHandler {
	return &ReindexHandler{
		contentService: contentService,
		personService:  personService,
		searchService:  searchService,
	}
}

type reindexResult struct {
	People           int `json:"people"`
	Content          int `json:"content"`
	JoinTableEntries int `json:"joinTableEntries"`
}

// Trigger runs a full reindex: ensures OpenSearch indexes exist, re-indexes all
// people and content documents, and resyncs all DynamoDB join-table entries.
func (h *ReindexHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.searchService.EnsureIndexes(ctx); err != nil {
		errs.Write(w, r, err)
		return
	}

	// Reindex people.
	var (
		peopleKey map[string]types.AttributeValue
		people    int
	)
	for {
		batch, nextKey, err := h.personService.GetAll(ctx, 100, peopleKey)
		if err != nil {
			errs.Write(w, r, err)
			return
		}
		for _, person := range batch {
			if err := h.searchService.IndexPerson(ctx, person); err != nil {
				errs.Write(w, r, err)
				return
			}
		}
		people += len(batch)
		if len(nextKey) == 0 {
			break
		}
		peopleKey = nextKey
	}

	// Reindex content.
	var (
		contentKey map[string]types.AttributeValue
		content    int
	)
	for {
		batch, nextKey, err := h.contentService.GetAllAdmin(ctx, 100, contentKey)
		if err != nil {
			errs.Write(w, r, err)
			return
		}
		for _, movie := range batch {
			if err := h.searchService.IndexContent(ctx, movie); err != nil {
				errs.Write(w, r, err)
				return
			}
			content++
		}
		if len(nextKey) == 0 {
			break
		}
		contentKey = nextKey
	}

	// Resync DynamoDB join tables.
	joinTableEntries, err := h.contentService.ResyncAllJoinTables(ctx)
	if err != nil {
		errs.Write(w, r, err)
		return
	}

	slog.InfoContext(ctx, "reindex via HTTP complete",
		"people", people,
		"content", content,
		"joinTableEntries", joinTableEntries,
	)

	writeOK(w, r, http.StatusOK, "reindex complete", reindexResult{
		People:           people,
		Content:          content,
		JoinTableEntries: joinTableEntries,
	})
}
