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
