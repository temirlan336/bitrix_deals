package server

import (
	"context"
	"encoding/json"
	"freedom_bitrix/internal/bitrix"
	"freedom_bitrix/internal/repo"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ufCoopType   = "UF_CRM_1740477560309"
	ufClientType = "UF_CRM_1647265424537"
	ufSource1    = "UF_CRM_1699841388494"
)

type Server struct {
	repo       *repo.DealsRepository
	bitrix     *bitrix.Client
	mappingTTL time.Duration
	sheetsLoc  *time.Location

	mu            sync.RWMutex
	cachedMapping dealMappings
	cacheUpdated  time.Time
}

func New(repository *repo.DealsRepository, bitrixClient *bitrix.Client) *Server {
	loc, err := time.LoadLocation("Asia/Almaty")
	if err != nil {
		loc = time.FixedZone("UTC+5", 5*60*60)
	}

	return &Server{
		repo:          repository,
		bitrix:        bitrixClient,
		mappingTTL:    10 * time.Minute,
		sheetsLoc:     loc,
		cachedMapping: newDealMappings(),
	}
}

func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/deals/sheets", s.handleDealsSheets)

	log.Printf("HTTP server on %s", addr)
	return http.ListenAndServe(addr, mux)
}

type dealsSheetsResponse struct {
	Headers []string `json:"headers"`
	Rows    [][]any  `json:"rows"`
}

func (s *Server) handleDealsSheets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	result, err := s.repo.ListDeals(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	maps := s.loadMappings(ctx, result)

	headers := []string{
		"Воронка",
		"Стадия сделки",
		"Ответственный",
		"Источник",
		"Дата создания",
		"UTM Source",
		"Тип сотрудничества",
		"UTM Campaign",
		"Тип клиента",
		"Собеседование проведено (дата когда фактически кандидат пришел)",
		"Источник1",
		"Дата КОГДА назначено собеседование",
		"Дата КОГДА назначена встреча",
		"Дата/ время КОГДА прошла встреча",
		"Вторичный собес УЦ",
		"ID",
	}

	rows := make([][]any, 0, len(result))
	for _, d := range result {
		row := []any{
			mapInt(maps.categoryNames, d.CategoryID),
			mapString(maps.stageNames, d.StageID),
			mapInt64(maps.assignedNames, d.AssignedByID),
			mapString(maps.sourceNames, d.SourceID),
			toSheetsDateTimeSerialInLocation(d.DateCreate, s.sheetsLoc),
			strOrEmpty(d.UTMSource),
			mapNullableString(maps.coopTypeNames, d.CoopType),
			strOrEmpty(d.UTMCampaign),
			mapNullableString(maps.clientTypeNames, d.ClientType),
			toSheetsDateSerialPtr(d.UFCRM1650279712660Date),
			mapNullableEnumStrict(maps.source1Names, d.UFCRM1699841388494),
			toSheetsDateSerialPtr(d.UFCRM1699863367472Date),
			toSheetsDateSerialPtr(d.UFCRM1752578793696Date),
			toSheetsDateTimeSerialPtrInLocation(d.UFCRM1753169789836At, s.sheetsLoc),
			toSheetsDateSerialPtr(d.UFCRM1771313479555Date),
			d.ID,
		}
		rows = append(rows, row)
	}

	resp := dealsSheetsResponse{
		Headers: headers,
		Rows:    rows,
	}

	// Force clients (including Sheets bridges) to fetch fresh data every time.
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type dealMappings struct {
	categoryNames   map[int]string
	stageNames      map[string]string
	assignedNames   map[int64]string
	sourceNames     map[string]string
	coopTypeNames   map[string]string
	clientTypeNames map[string]string
	source1Names    map[string]string
}

func (s *Server) loadMappings(ctx context.Context, deals []repo.DealRow) dealMappings {
	if s.bitrix == nil {
		return newDealMappings()
	}

	m, updatedAt := s.getCachedMappings()
	catIDs, userIDs := collectIDs(deals)
	stale := updatedAt.IsZero() || time.Since(updatedAt) > s.mappingTTL

	if stale {
		if cats, err := s.fetchCategoryNames(ctx, catIDs); err != nil {
			log.Printf("mapping category names: %v", err)
		} else {
			m.categoryNames = cats
		}
	} else {
		missingCats := missingCategoryIDs(m.categoryNames, catIDs)
		if len(missingCats) > 0 {
			if cats, err := s.fetchCategoryNames(ctx, missingCats); err != nil {
				log.Printf("mapping missing category names: %v", err)
			} else {
				for id, name := range cats {
					m.categoryNames[id] = name
				}
			}
		}
	}

	if stale || len(m.stageNames) == 0 || len(m.sourceNames) == 0 {
		if stages, sources, source1FromStatus, err := s.fetchStatusMaps(ctx); err != nil {
			log.Printf("mapping status/source names: %v", err)
		} else {
			m.stageNames = stages
			m.sourceNames = sources
			m.source1Names = source1FromStatus
		}
	}

	if stale {
		if users, err := s.fetchAssignedNames(ctx, userIDs); err != nil {
			log.Printf("mapping assigned names: %v", err)
		} else {
			m.assignedNames = users
		}
	} else {
		missingUsers := missingUserIDs(m.assignedNames, userIDs)
		if len(missingUsers) > 0 {
			if users, err := s.fetchAssignedNames(ctx, missingUsers); err != nil {
				log.Printf("mapping missing assigned names: %v", err)
			} else {
				for id, name := range users {
					m.assignedNames[id] = name
				}
			}
		}
	}

	if stale || len(m.coopTypeNames) == 0 {
		if coop, err := s.fetchDealUserFieldEnum(ctx, ufCoopType); err != nil {
			log.Printf("mapping coop type names: %v", err)
		} else {
			m.coopTypeNames = coop
		}
	}

	if stale || len(m.clientTypeNames) == 0 {
		if clients, err := s.fetchDealUserFieldEnum(ctx, ufClientType); err != nil {
			log.Printf("mapping client type names: %v", err)
		} else {
			m.clientTypeNames = clients
		}
	}

	if stale || len(m.source1Names) == 0 {
		if source1, err := s.fetchDealUserFieldEnum(ctx, ufSource1); err != nil {
			log.Printf("mapping source1 names: %v", err)
		} else if len(source1) > 0 {
			m.source1Names = source1
		}
	}

	s.setCachedMappings(m)
	return m
}

func newDealMappings() dealMappings {
	return dealMappings{
		categoryNames:   map[int]string{},
		stageNames:      map[string]string{},
		assignedNames:   map[int64]string{},
		sourceNames:     map[string]string{},
		coopTypeNames:   map[string]string{},
		clientTypeNames: map[string]string{},
		source1Names:    map[string]string{},
	}
}

func (s *Server) getCachedMappings() (dealMappings, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneDealMappings(s.cachedMapping), s.cacheUpdated
}

func (s *Server) setCachedMappings(m dealMappings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cachedMapping = cloneDealMappings(m)
	s.cacheUpdated = time.Now()
}

func cloneDealMappings(src dealMappings) dealMappings {
	dst := newDealMappings()
	for k, v := range src.categoryNames {
		dst.categoryNames[k] = v
	}
	for k, v := range src.stageNames {
		dst.stageNames[k] = v
	}
	for k, v := range src.assignedNames {
		dst.assignedNames[k] = v
	}
	for k, v := range src.sourceNames {
		dst.sourceNames[k] = v
	}
	for k, v := range src.coopTypeNames {
		dst.coopTypeNames[k] = v
	}
	for k, v := range src.clientTypeNames {
		dst.clientTypeNames[k] = v
	}
	for k, v := range src.source1Names {
		dst.source1Names[k] = v
	}
	return dst
}

func missingCategoryIDs(existing map[int]string, ids []string) []string {
	missing := make([]string, 0)
	for _, idStr := range ids {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		if _, ok := existing[id]; !ok {
			missing = append(missing, idStr)
		}
	}
	return missing
}

func missingUserIDs(existing map[int64]string, ids []string) []string {
	missing := make([]string, 0)
	for _, idStr := range ids {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			continue
		}
		if _, ok := existing[id]; !ok {
			missing = append(missing, idStr)
		}
	}
	return missing
}

func collectIDs(deals []repo.DealRow) (categoryIDs []string, userIDs []string) {
	cats := make(map[string]struct{})
	users := make(map[string]struct{})

	for _, d := range deals {
		cats[strconv.Itoa(d.CategoryID)] = struct{}{}
		users[strconv.FormatInt(d.AssignedByID, 10)] = struct{}{}
	}

	categoryIDs = make([]string, 0, len(cats))
	for id := range cats {
		categoryIDs = append(categoryIDs, id)
	}

	userIDs = make([]string, 0, len(users))
	for id := range users {
		userIDs = append(userIDs, id)
	}

	return categoryIDs, userIDs
}

func (s *Server) fetchCategoryNames(ctx context.Context, ids []string) (map[int]string, error) {
	payload := map[string]any{}
	if len(ids) > 0 {
		payload["filter"] = map[string]any{"ID": ids}
	}

	var resp bitrix.ListResponse[bitrix.DealCategory]
	if err := s.bitrix.Call(ctx, "crm.dealcategory.list", payload, &resp); err != nil {
		return nil, err
	}

	out := make(map[int]string)
	for _, c := range resp.Result {
		id, err := strconv.Atoi(c.ID)
		if err != nil {
			continue
		}
		out[id] = c.Name
	}
	return out, nil
}

func (s *Server) fetchStatusMaps(ctx context.Context) (map[string]string, map[string]string, map[string]string, error) {
	var resp bitrix.ListResponse[bitrix.Status]
	if err := s.bitrix.Call(ctx, "crm.status.list", map[string]any{}, &resp); err != nil {
		return nil, nil, nil, err
	}

	stages := make(map[string]string)
	sources := make(map[string]string)
	source1 := make(map[string]string)

	for _, st := range resp.Result {
		e := strings.ToUpper(strings.TrimSpace(st.EntityID))
		sid := strings.TrimSpace(st.StatusID)
		if sid == "" {
			continue
		}

		switch {
		case strings.HasPrefix(e, "DEAL_STAGE"):
			stages[sid] = st.Name
		case e == "SOURCE":
			sources[sid] = st.Name
		case e == "SOURCE1", e == "SOURCE_1":
			source1[sid] = st.Name
		}
	}

	return stages, sources, source1, nil
}

func (s *Server) fetchAssignedNames(ctx context.Context, ids []string) (map[int64]string, error) {
	payload := map[string]any{}
	if len(ids) > 0 {
		payload["FILTER"] = map[string]any{"ID": ids}
	}

	var resp bitrix.UsersResponse
	if err := s.bitrix.Call(ctx, "user.get", payload, &resp); err != nil {
		return nil, err
	}

	out := make(map[int64]string)
	for _, u := range resp.Result {
		id, err := strconv.ParseInt(strings.TrimSpace(u.ID), 10, 64)
		if err != nil {
			continue
		}
		first := strings.TrimSpace(u.Name)
		last := strings.TrimSpace(u.LastName)
		second := strings.TrimSpace(u.SecondName)
		full := strings.TrimSpace(strings.Join([]string{last, first, second}, " "))
		if full == "" {
			full = u.ID
		}
		out[id] = full
	}
	return out, nil
}

func (s *Server) fetchDealUserFieldEnum(ctx context.Context, fieldName string) (map[string]string, error) {
	payload := map[string]any{
		"filter": map[string]any{"FIELD_NAME": fieldName},
	}

	var resp bitrix.ListResponse[bitrix.DealUserField]
	if err := s.bitrix.Call(ctx, "crm.deal.userfield.list", payload, &resp); err != nil {
		return nil, err
	}

	out := make(map[string]string)
	for _, f := range resp.Result {
		for _, item := range f.List {
			id := strings.TrimSpace(item.ID)
			if id == "" {
				continue
			}
			out[id] = item.Value
		}
	}
	return out, nil
}

func mapInt(m map[int]string, v int) string {
	if s, ok := m[v]; ok {
		return s
	}
	return strconv.Itoa(v)
}

func mapInt64(m map[int64]string, v int64) string {
	if s, ok := m[v]; ok {
		return s
	}
	return strconv.FormatInt(v, 10)
}

func mapString(m map[string]string, v string) string {
	if s, ok := m[v]; ok {
		return s
	}
	return v
}

func mapNullableString(m map[string]string, v *string) string {
	if v == nil {
		return ""
	}
	normalized := strings.TrimSpace(*v)
	return mapString(m, normalized)
}

func mapNullableEnumStrict(m map[string]string, v *string) string {
	if v == nil {
		return ""
	}
	normalized := strings.TrimSpace(*v)
	if normalized == "" {
		return ""
	}
	if s, ok := m[normalized]; ok {
		return s
	}
	return ""
}

func strOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func toSheetsDateSerialPtr(v *time.Time) any {
	if v == nil || v.IsZero() {
		return ""
	}
	y, m, d := v.Date()
	dateOnly := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return toSheetsSerial(dateOnly)
}

func toSheetsDateTimeSerial(v time.Time) any {
	if v.IsZero() {
		return ""
	}
	return toSheetsSerial(v.UTC())
}

func toSheetsDateTimeSerialInLocation(v time.Time, loc *time.Location) any {
	if v.IsZero() {
		return ""
	}
	if loc == nil {
		return toSheetsDateTimeSerial(v)
	}
	t := v.In(loc)
	naive := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
	return toSheetsSerial(naive)
}

func toSheetsDateTimeSerialPtrInLocation(v *time.Time, loc *time.Location) any {
	if v == nil || v.IsZero() {
		return ""
	}
	return toSheetsDateTimeSerialInLocation(*v, loc)
}

func toSheetsSerial(t time.Time) float64 {
	epoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	return t.Sub(epoch).Hours() / 24
}
