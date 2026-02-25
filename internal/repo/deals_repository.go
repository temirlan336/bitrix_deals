package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"freedom_bitrix/internal/bitrix"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DealsRepository struct {
	pool *pgxpool.Pool
}

type SyncStatus struct {
	Watermark      *time.Time `json:"watermark"`
	LastDealModify *time.Time `json:"last_deal_modify"`
}

type DealRow struct {
	ID                     int64      `json:"id"`
	CategoryID             int        `json:"category_id"`
	StageID                string     `json:"stage_id"`
	AssignedByID           int64      `json:"assigned_by_id"`
	SourceID               string     `json:"source_id"`
	DateCreate             time.Time  `json:"date_create"`
	UTMSource              *string    `json:"utm_source"`
	UTMCampaign            *string    `json:"utm_campaign"`
	CoopType               *string    `json:"coop_type"`
	ClientType             *string    `json:"client_type"`
	UFCRM1650279712660     *string    `json:"UF_CRM_1650279712660"`
	UFCRM1699841388494     *string    `json:"UF_CRM_1699841388494"`
	UFCRM1699863367472     *string    `json:"UF_CRM_1699863367472"`
	UFCRM1752578793696     *string    `json:"UF_CRM_1752578793696"`
	UFCRM1753169789836     *string    `json:"UF_CRM_1753169789836"`
	UFCRM1771313479555     *string    `json:"UF_CRM_1771313479555"`
	UFCRM1650279712660Date *time.Time `json:"-"`
	UFCRM1699863367472Date *time.Time `json:"-"`
	UFCRM1752578793696Date *time.Time `json:"-"`
	UFCRM1753169789836At   *time.Time `json:"-"`
	UFCRM1771313479555Date *time.Time `json:"-"`
}

func NewDealsRepository(pool *pgxpool.Pool) *DealsRepository {
	return &DealsRepository{pool: pool}
}

func (r *DealsRepository) Migrate(ctx context.Context) error {
	ddl := `
CREATE TABLE IF NOT EXISTS bitrix_deals (
  id               bigint PRIMARY KEY,
  category_id      int,
  stage_id         text,
  assigned_by_id   bigint,
  source_id        text,
  date_create      timestamptz,
  date_modify      timestamptz,
  utm_source       text,
  utm_campaign     text,
  uf_coop_type     text,
  uf_client_type   text,
  uf_crm_1650279712660 text,
  uf_crm_1699841388494 text,
  uf_crm_1699863367472 text,
  uf_crm_1752578793696 text,
  uf_crm_1753169789836 text,
  uf_crm_1771313479555 text,
  uf_crm_1650279712660_date date,
  uf_crm_1699863367472_date date,
  uf_crm_1752578793696_date date,
  uf_crm_1753169789836_at timestamptz,
  uf_crm_1771313479555_date date,
  raw              jsonb,
  updated_at       timestamptz DEFAULT now()
);

CREATE INDEX IF NOT EXISTS bitrix_deals_date_modify_idx ON bitrix_deals(date_modify);

CREATE TABLE IF NOT EXISTS sync_state (
  key         text PRIMARY KEY,
  watermark   timestamptz NOT NULL,
  updated_at  timestamptz NOT NULL DEFAULT now()
);
`
	_, err := r.pool.Exec(ctx, ddl)
	return err
}

func (r *DealsRepository) UpsertDeals(ctx context.Context, deals []bitrix.Deal) error {
	if len(deals) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	sql := `
INSERT INTO bitrix_deals (
  id, category_id, stage_id, assigned_by_id, source_id,
  date_create, date_modify, utm_source, utm_campaign,
  uf_coop_type, uf_client_type,
  uf_crm_1650279712660, uf_crm_1699841388494, uf_crm_1699863367472, uf_crm_1752578793696, uf_crm_1753169789836, uf_crm_1771313479555,
  uf_crm_1650279712660_date, uf_crm_1699863367472_date, uf_crm_1752578793696_date, uf_crm_1753169789836_at, uf_crm_1771313479555_date,
  raw, updated_at
) VALUES (
  $1,$2,$3,$4,$5,
  $6,$7,$8,$9,
  $10,$11,
  $12,$13,$14,$15,$16,$17,
  $18,$19,$20,$21,$22,
  $23, now()
)
ON CONFLICT (id) DO UPDATE SET
  category_id = EXCLUDED.category_id,
  stage_id = EXCLUDED.stage_id,
  assigned_by_id = EXCLUDED.assigned_by_id,
  source_id = EXCLUDED.source_id,
  date_create = EXCLUDED.date_create,
  date_modify = EXCLUDED.date_modify,
  utm_source = EXCLUDED.utm_source,
  utm_campaign = EXCLUDED.utm_campaign,
  uf_coop_type = EXCLUDED.uf_coop_type,
  uf_client_type = EXCLUDED.uf_client_type,
  uf_crm_1650279712660 = EXCLUDED.uf_crm_1650279712660,
  uf_crm_1699841388494 = EXCLUDED.uf_crm_1699841388494,
  uf_crm_1699863367472 = EXCLUDED.uf_crm_1699863367472,
  uf_crm_1752578793696 = EXCLUDED.uf_crm_1752578793696,
  uf_crm_1753169789836 = EXCLUDED.uf_crm_1753169789836,
  uf_crm_1771313479555 = EXCLUDED.uf_crm_1771313479555,
  uf_crm_1650279712660_date = EXCLUDED.uf_crm_1650279712660_date,
  uf_crm_1699863367472_date = EXCLUDED.uf_crm_1699863367472_date,
  uf_crm_1752578793696_date = EXCLUDED.uf_crm_1752578793696_date,
  uf_crm_1753169789836_at = EXCLUDED.uf_crm_1753169789836_at,
  uf_crm_1771313479555_date = EXCLUDED.uf_crm_1771313479555_date,
  raw = EXCLUDED.raw,
  updated_at = now();
`

	for _, d := range deals {
		id := toInt64(d.ID)
		cat := toInt(d.CategoryID)
		ass := toInt64(d.AssignedByID)

		dc, _ := parseRFC3339(d.DateCreate)
		dm, _ := parseRFC3339(d.DateModify)
		uf165Date, _ := parseBitrixDateOnly(d.UFCRM1650279712660)
		uf169Date, _ := parseBitrixDateOnly(d.UFCRM1699863367472)
		uf171Date, _ := parseBitrixDateOnly(d.UFCRM1752578793696)
		uf175At, _ := parseBitrixDateTime(d.UFCRM1753169789836)
		uf177Date, _ := parseBitrixDateOnly(d.UFCRM1771313479555)

		raw, _ := json.Marshal(d)

		_, err := tx.Exec(ctx, sql,
			id, cat, d.StageID, ass, d.SourceID,
			nullTime(dc), nullTime(dm), d.UTMSource, d.UTMCampaign,
			emptyToNull(d.UFCoopType), emptyToNull(d.UFClientType),
			emptyToNull(d.UFCRM1650279712660),
			emptyToNull(d.UFCRM1699841388494),
			emptyToNull(d.UFCRM1699863367472),
			emptyToNull(d.UFCRM1752578793696),
			emptyToNull(d.UFCRM1753169789836),
			emptyToNull(d.UFCRM1771313479555),
			nullTime(uf165Date),
			nullTime(uf169Date),
			nullTime(uf171Date),
			nullTime(uf175At),
			nullTime(uf177Date),
			raw,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *DealsRepository) GetWatermark(ctx context.Context, key string) (time.Time, error) {
	var wm time.Time
	err := r.pool.QueryRow(ctx, `SELECT watermark FROM sync_state WHERE key=$1`, key).Scan(&wm)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return wm, nil
}

func (r *DealsRepository) SetWatermark(ctx context.Context, key string, wm time.Time) error {
	_, err := r.pool.Exec(ctx, `
INSERT INTO sync_state(key, watermark) VALUES($1, $2)
ON CONFLICT (key) DO UPDATE SET watermark=EXCLUDED.watermark, updated_at=now()
`, key, wm)
	return err
}

func (r *DealsRepository) GetSyncStatus(ctx context.Context, key string) (SyncStatus, error) {
	var st SyncStatus
	err := r.pool.QueryRow(ctx, `
SELECT
  (SELECT watermark FROM sync_state WHERE key=$1),
  (SELECT max(date_modify) FROM bitrix_deals)
`, key).Scan(&st.Watermark, &st.LastDealModify)
	if err != nil {
		return SyncStatus{}, err
	}
	return st, nil
}

func (r *DealsRepository) ListDeals(ctx context.Context) ([]DealRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
		  id,
		  category_id,
		  stage_id,
		  assigned_by_id,
		  source_id,
		  date_create,
		  utm_source,
		  utm_campaign,
		  uf_coop_type,
		  uf_client_type,
		  uf_crm_1650279712660,
		  uf_crm_1699841388494,
		  uf_crm_1699863367472,
		  uf_crm_1752578793696,
		  uf_crm_1753169789836,
		  uf_crm_1771313479555,
		  uf_crm_1650279712660_date,
		  uf_crm_1699863367472_date,
		  uf_crm_1752578793696_date,
		  uf_crm_1753169789836_at,
		  uf_crm_1771313479555_date
		FROM bitrix_deals
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]DealRow, 0)
	for rows.Next() {
		var r DealRow
		if err := rows.Scan(
			&r.ID,
			&r.CategoryID,
			&r.StageID,
			&r.AssignedByID,
			&r.SourceID,
			&r.DateCreate,
			&r.UTMSource,
			&r.UTMCampaign,
			&r.CoopType,
			&r.ClientType,
			&r.UFCRM1650279712660,
			&r.UFCRM1699841388494,
			&r.UFCRM1699863367472,
			&r.UFCRM1752578793696,
			&r.UFCRM1753169789836,
			&r.UFCRM1771313479555,
			&r.UFCRM1650279712660Date,
			&r.UFCRM1699863367472Date,
			&r.UFCRM1752578793696Date,
			&r.UFCRM1753169789836At,
			&r.UFCRM1771313479555Date,
		); err != nil {
			return nil, err
		}
		result = append(result, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func parseRFC3339(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	return time.Parse(time.RFC3339, s)
}

func parseBitrixDateOnly(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	if len(s) >= 10 {
		s = s[:10]
	}
	return time.Parse("2006-01-02", s)
}

func parseBitrixDateTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty datetime")
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported datetime: %s", s)
}

func toInt(s string) int {
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}

func toInt64(s string) int64 {
	var n int64
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}

func emptyToNull(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
