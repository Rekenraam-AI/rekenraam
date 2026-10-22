package app

import (
	"context"

	"rekenraam/backend/internal/db"
)

func (s *InvestmentService) SplitActivity(ctx context.Context, ownerUserID, beforeID int64, limit int) ([]db.SplitActivityRecord, int64, error) {
	ctx, referenceErr := s.settings.referenceContext(ctx, s.now())
	if referenceErr != nil {
		return nil, 0, referenceErr
	}

	if ownerUserID <= 0 || beforeID < 0 || limit < 1 || limit > 100 {
		return nil, 0, ValidationError{Message: "owner and valid split page are required"}
	}
	rows, err := s.repository.SplitActivity(ctx, BookID, beforeID, limit+1, 0)
	if err != nil {
		return nil, 0, err
	}
	var next int64
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[len(rows)-1].OperationID
	}
	return rows, next, nil
}

func (s *InvestmentService) SplitActivityForOperation(ctx context.Context, ownerUserID, operationID int64) ([]db.SplitActivityRecord, error) {
	ctx, referenceErr := s.settings.referenceContext(ctx, s.now())
	if referenceErr != nil {
		return nil, referenceErr
	}

	if ownerUserID <= 0 || operationID <= 0 {
		return nil, ValidationError{Message: "owner and split operation are required"}
	}
	rows, err := s.repository.SplitActivity(ctx, BookID, 0, 1, operationID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrInvestmentSplitNotFound
	}
	return rows, nil
}
