package syncer

import (
	"context"
	"fmt"
	"freedom_bitrix/internal/bitrix"
	"freedom_bitrix/internal/repo"
	"log"
	"time"
)

type Service struct {
	bitrix      *bitrix.Client
	repo        *repo.DealsRepository
	stateKey    string
	overlap     time.Duration
	staleAfter  time.Duration
	categories  []int
	retryCount  int
	requestWait time.Duration
}

func NewService(bitrixClient *bitrix.Client, repository *repo.DealsRepository, stateKey string, overlap time.Duration) *Service {
	return &Service{
		bitrix:      bitrixClient,
		repo:        repository,
		stateKey:    stateKey,
		overlap:     overlap,
		staleAfter:  2 * time.Hour,
		categories:  []int{1, 31, 29},
		retryCount:  3,
		requestWait: 300 * time.Millisecond,
	}
}

func (s *Service) FullSync(ctx context.Context) error {
	log.Println("FULL SYNC START")

	payload := map[string]any{
		"SELECT": dealSelectFields(),
		"FILTER": map[string]any{
			">=DATE_CREATE": "2024-01-01",
			"@CATEGORY_ID":  s.categories,
		},
		"ORDER": map[string]any{
			"DATE_CREATE": "DESC",
			"ID":          "DESC",
		},
	}

	start := 0
	pageNum := 0
	total := -1
	collected := 0
	var maxModify time.Time

	for {
		pageNum++
		payload["start"] = start

		var page bitrix.ListResponse[bitrix.Deal]
		err := callWithRetry(ctx, s.retryCount, func(c context.Context) error {
			reqCtx, cancel := context.WithTimeout(c, 25*time.Second)
			defer cancel()
			return s.bitrix.Call(reqCtx, "crm.deal.list", payload, &page)
		})
		if err != nil {
			return fmt.Errorf("bitrix page %d start=%d: %w", pageNum, start, err)
		}

		if page.Total != nil {
			total = *page.Total
		}

		if err := s.repo.UpsertDeals(ctx, page.Result); err != nil {
			return fmt.Errorf("upsert deals page %d: %w", pageNum, err)
		}

		for _, d := range page.Result {
			tm, err := parseRFC3339(d.DateModify)
			if err == nil && tm.After(maxModify) {
				maxModify = tm
			}
		}

		collected += len(page.Result)
		nextVal := -1
		if page.Next != nil {
			nextVal = *page.Next
		}
		log.Printf("page=%d got=%d start=%d next=%d total=%d collected=%d",
			pageNum, len(page.Result), start, nextVal, total, collected)

		if page.Next == nil {
			break
		}
		start = *page.Next
		time.Sleep(s.requestWait)
	}

	if !maxModify.IsZero() {
		if err := s.repo.SetWatermark(ctx, s.stateKey, maxModify); err != nil {
			return fmt.Errorf("set watermark: %w", err)
		}
		log.Printf("FULL SYNC watermark=%s", maxModify.UTC().Format(time.RFC3339))
	}

	log.Println("FULL SYNC END")
	return nil
}

func (s *Service) DeltaSync(ctx context.Context) error {
	log.Println("DELTA SYNC START")

	wm, err := s.repo.GetWatermark(ctx, s.stateKey)
	if err != nil {
		return fmt.Errorf("get watermark: %w", err)
	}

	if wm.IsZero() {
		log.Println("no watermark found -> run: go run . full")
		return nil
	}

	from := wm.Add(-s.overlap)
	fromStr := from.UTC().Format(time.RFC3339)
	log.Printf("DELTA SYNC range: watermark=%s from=%s overlap=%s",
		wm.UTC().Format(time.RFC3339), fromStr, s.overlap)

	payload := map[string]any{
		"SELECT": dealSelectFields(),
		"FILTER": map[string]any{
			">=DATE_MODIFY": fromStr,
			"@CATEGORY_ID":  s.categories,
		},
		"ORDER": map[string]any{
			"DATE_MODIFY": "ASC",
			"ID":          "ASC",
		},
	}

	start := 0
	pageNum := 0
	updated := 0
	maxModify := wm

	for {
		pageNum++
		payload["start"] = start

		var page bitrix.ListResponse[bitrix.Deal]
		err := callWithRetry(ctx, s.retryCount, func(c context.Context) error {
			reqCtx, cancel := context.WithTimeout(c, 25*time.Second)
			defer cancel()
			return s.bitrix.Call(reqCtx, "crm.deal.list", payload, &page)
		})
		if err != nil {
			return fmt.Errorf("bitrix delta page %d start=%d: %w", pageNum, start, err)
		}

		if len(page.Result) > 0 {
			if err := s.repo.UpsertDeals(ctx, page.Result); err != nil {
				return fmt.Errorf("upsert delta page %d: %w", pageNum, err)
			}
		}

		for _, d := range page.Result {
			tm, err := parseRFC3339(d.DateModify)
			if err == nil && tm.After(maxModify) {
				maxModify = tm
			}
		}

		updated += len(page.Result)
		nextVal := -1
		if page.Next != nil {
			nextVal = *page.Next
		}
		log.Printf("delta page=%d got=%d start=%d next=%d updated=%d watermark_now=%s",
			pageNum, len(page.Result), start, nextVal, updated, maxModify.UTC().Format(time.RFC3339))

		if page.Next == nil {
			break
		}
		start = *page.Next
		time.Sleep(s.requestWait)
	}

	if maxModify.After(wm) {
		if err := s.repo.SetWatermark(ctx, s.stateKey, maxModify); err != nil {
			return fmt.Errorf("set watermark: %w", err)
		}
		log.Printf("DELTA SYNC watermark=%s", maxModify.UTC().Format(time.RFC3339))
	} else if !wm.IsZero() {
		age := time.Since(wm)
		if age > s.staleAfter {
			log.Printf("WARN delta sync: no newer deals for %s (watermark=%s)",
				age.Round(time.Minute), wm.UTC().Format(time.RFC3339))
		}
	}

	log.Println("DELTA SYNC END")
	return nil
}

func dealSelectFields() []string {
	return []string{
		"CATEGORY_ID",
		"STAGE_ID",
		"ASSIGNED_BY_ID",
		"SOURCE_ID",
		"DATE_CREATE",
		"DATE_MODIFY",
		"UTM_SOURCE",
		"UTM_CAMPAIGN",
		"UF_CRM_1740477560309",
		"UF_CRM_1647265424537",
		"UF_CRM_1650279712660",
		"UF_CRM_1699841388494",
		"UF_CRM_1699863367472",
		"UF_CRM_1752578793696",
		"UF_CRM_1753169789836",
		"UF_CRM_1771313479555",
		"ID",
	}
}

func parseRFC3339(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	return time.Parse(time.RFC3339, s)
}
