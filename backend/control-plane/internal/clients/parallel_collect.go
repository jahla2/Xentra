package clients

import (
	"context"
	"sync"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

func collectEvidenceParallel(
	ctx context.Context,
	requests []domain.ToolRequest,
	execute func(context.Context, domain.ToolRequest) (domain.Evidence, error),
) []domain.Evidence {
	result := make([]domain.Evidence, len(requests))
	var wg sync.WaitGroup
	wg.Add(len(requests))
	for index, request := range requests {
		index, request := index, request
		go func() {
			defer wg.Done()
			item, err := execute(ctx, request)
			if err != nil {
				item = domain.Evidence{
					Source: request.Tool, Output: err.Error(), Success: false,
					OccurredAt: time.Now().UTC(),
				}
			}
			result[index] = item
		}()
	}
	wg.Wait()
	return result
}
