package store

import (
	"context"
	"database/sql"
	"strings"
)

// owns reports ErrNotFound unless the row exists and belongs to the person.
// table is one of ours, never user input.
func (s *Store) owns(ctx context.Context, table string, uid int64, id *int64) error {
	if id == nil || *id == 0 {
		return nil
	}
	var n int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM `+table+` WHERE id = ? AND user_id = ?`, *id, uid).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// OwnsHome is owns for a home id.
func (s *Store) OwnsHome(ctx context.Context, uid, id int64) error {
	return s.owns(ctx, "homes", uid, &id)
}

func nullID(p *int64) any {
	if p == nil || *p == 0 {
		return nil
	}
	return *p
}

func scanID(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

// ---------- homes ----------

const homeCols = `id, user_id, name, kind, address, year_built, area_m2, currency, notes, money_tag, archived, sort, created_at, updated_at`

func scanHome(row interface{ Scan(...any) error }) (*Home, error) {
	var h Home
	var yb sql.NullInt64
	var area sql.NullFloat64
	if err := row.Scan(&h.ID, &h.UserID, &h.Name, &h.Kind, &h.Address, &yb, &area, &h.Currency, &h.Notes, &h.MoneyTag, &h.Archived, &h.Sort, &h.CreatedAt, &h.UpdatedAt); err != nil {
		return nil, err
	}
	if yb.Valid {
		v := int(yb.Int64)
		h.YearBuilt = &v
	}
	if area.Valid {
		v := area.Float64
		h.AreaM2 = &v
	}
	return &h, nil
}

// ListHomes lists a person's homes with their counts (archived last).
func (s *Store) ListHomes(ctx context.Context, uid int64) ([]Home, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+homeCols+` FROM homes WHERE user_id = ? ORDER BY archived, sort, id`, uid)
	if err != nil {
		return nil, err
	}
	var out []Home
	for rows.Next() {
		h, err := scanHome(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *h)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	today := Today()
	for i := range out {
		h := &out[i]
		_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM things WHERE user_id = ? AND home_id = ? AND archived = 0`, uid, h.ID).Scan(&h.Things)
		_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(1),
			COALESCE(SUM(CASE WHEN due_on < ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN due_on >= ? AND julianday(due_on) - julianday(?) <= soon_days THEN 1 ELSE 0 END), 0)
			FROM jobs WHERE user_id = ? AND home_id = ? AND paused = 0 AND finished_at = ''`, today, today, today, uid, h.ID).Scan(&h.Open, &h.Overdue, &h.DueSoon)
	}
	return out, nil
}

// GetHome reads one of the person's homes.
func (s *Store) GetHome(ctx context.Context, uid, id int64) (*Home, error) {
	h, err := scanHome(s.DB.QueryRowContext(ctx, `SELECT `+homeCols+` FROM homes WHERE id = ? AND user_id = ?`, id, uid))
	return h, notFound(err)
}

// CreateHome adds a home.
func (s *Store) CreateHome(ctx context.Context, uid int64, h *Home) error {
	if h.Currency == "" {
		h.Currency = "EUR"
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO homes (user_id, name, kind, address, year_built, area_m2, currency, notes, money_tag, archived, sort)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, uid, h.Name, h.Kind, h.Address, h.YearBuilt, h.AreaM2, h.Currency, h.Notes, h.MoneyTag, boolInt(h.Archived), h.Sort)
	if err != nil {
		return err
	}
	h.ID, _ = res.LastInsertId()
	h.UserID = uid
	return nil
}

// UpdateHome saves a home.
func (s *Store) UpdateHome(ctx context.Context, uid int64, h *Home) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE homes SET name = ?, kind = ?, address = ?, year_built = ?, area_m2 = ?, currency = ?, notes = ?, money_tag = ?, archived = ?, sort = ?, updated_at = ?
		WHERE id = ? AND user_id = ?`, h.Name, h.Kind, h.Address, h.YearBuilt, h.AreaM2, h.Currency, h.Notes, h.MoneyTag, boolInt(h.Archived), h.Sort, Now(), h.ID, uid)
	return affected(res, err)
}

// DeleteHome removes a home with everything in it.
func (s *Store) DeleteHome(ctx context.Context, uid, id int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, q := range []string{
		`DELETE FROM attachments WHERE user_id = ? AND owner_kind = 'thing' AND owner_id IN (SELECT id FROM things WHERE home_id = ?)`,
		`DELETE FROM attachments WHERE user_id = ? AND owner_kind = 'record' AND owner_id IN (SELECT id FROM records WHERE home_id = ?)`,
		`DELETE FROM attachments WHERE user_id = ? AND owner_kind = 'home' AND owner_id = ?`,
	} {
		if _, err := tx.ExecContext(ctx, q, uid, id); err != nil {
			return err
		}
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM homes WHERE id = ? AND user_id = ?`, id, uid)
	if err := affected(res, err); err != nil {
		return err
	}
	return tx.Commit()
}

func affected(res sql.Result, err error) error {
	if err != nil {
		return conflict(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- things ----------

const thingCols = `t.id, t.user_id, t.home_id, t.name, t.category, t.room, t.brand, t.model, t.serial, t.installed_on, t.purchased_on, t.price, t.vendor,
	t.warranty_until, t.manual_url, t.inventory_id, t.inventory_name, t.notes, t.archived, t.created_at, t.updated_at, h.name, h.currency`

func scanThing(row interface{ Scan(...any) error }) (*Thing, error) {
	var t Thing
	var price, inv sql.NullInt64
	if err := row.Scan(&t.ID, &t.UserID, &t.HomeID, &t.Name, &t.Category, &t.Room, &t.Brand, &t.Model, &t.Serial, &t.InstalledOn, &t.PurchasedOn, &price, &t.Vendor,
		&t.WarrantyUntil, &t.ManualURL, &inv, &t.InventoryName, &t.Notes, &t.Archived, &t.CreatedAt, &t.UpdatedAt, &t.HomeName, &t.Currency); err != nil {
		return nil, err
	}
	t.Price, t.InventoryID = scanID(price), scanID(inv)
	return &t, nil
}

// ThingFilter narrows the list of things.
type ThingFilter struct {
	HomeID   int64
	Category string
	Room     string
	Q        string
	Archived bool // include archived
}

// ListThings lists a person's things with their upkeep figures.
func (s *Store) ListThings(ctx context.Context, uid int64, f ThingFilter) ([]Thing, error) {
	q := `SELECT ` + thingCols + ` FROM things t JOIN homes h ON h.id = t.home_id WHERE t.user_id = ?`
	args := []any{uid}
	if f.HomeID > 0 {
		q += ` AND t.home_id = ?`
		args = append(args, f.HomeID)
	}
	if f.Category != "" {
		q += ` AND t.category = ?`
		args = append(args, f.Category)
	}
	if f.Room != "" {
		q += ` AND t.room = ?`
		args = append(args, f.Room)
	}
	if !f.Archived {
		q += ` AND t.archived = 0`
	}
	if f.Q != "" {
		like := "%" + strings.ToLower(f.Q) + "%"
		q += ` AND (lower(t.name) LIKE ? OR lower(t.brand) LIKE ? OR lower(t.model) LIKE ? OR lower(t.room) LIKE ? OR lower(t.notes) LIKE ? OR lower(t.serial) LIKE ?)`
		args = append(args, like, like, like, like, like, like)
	}
	q += ` ORDER BY t.archived, h.sort, h.id, t.room, t.name COLLATE NOCASE`
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	var out []Thing
	for rows.Next() {
		t, err := scanThing(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		s.thingFigures(ctx, uid, &out[i])
	}
	return out, nil
}

func (s *Store) thingFigures(ctx context.Context, uid int64, t *Thing) {
	today := Today()
	var next sql.NullString
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(1), COALESCE(SUM(CASE WHEN due_on < ? THEN 1 ELSE 0 END), 0), MIN(due_on)
		FROM jobs WHERE user_id = ? AND thing_id = ? AND paused = 0 AND finished_at = ''`, today, uid, t.ID).Scan(&t.Open, &t.Overdue, &next)
	t.NextDue = next.String
	var last sql.NullString
	_ = s.DB.QueryRowContext(ctx, `SELECT MAX(date), COALESCE(SUM(cost), 0) FROM records WHERE user_id = ? AND thing_id = ?`, uid, t.ID).Scan(&last, &t.Spent)
	t.LastDone = last.String
}

// GetThing reads one of the person's things.
func (s *Store) GetThing(ctx context.Context, uid, id int64) (*Thing, error) {
	t, err := scanThing(s.DB.QueryRowContext(ctx, `SELECT `+thingCols+` FROM things t JOIN homes h ON h.id = t.home_id WHERE t.id = ? AND t.user_id = ?`, id, uid))
	if err != nil {
		return nil, notFound(err)
	}
	s.thingFigures(ctx, uid, t)
	return t, nil
}

// CreateThing adds a thing to one of the person's homes.
func (s *Store) CreateThing(ctx context.Context, uid int64, t *Thing) error {
	if err := s.OwnsHome(ctx, uid, t.HomeID); err != nil {
		return err
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO things (user_id, home_id, name, category, room, brand, model, serial, installed_on, purchased_on, price, vendor,
		warranty_until, manual_url, inventory_id, inventory_name, notes, archived) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uid, t.HomeID, t.Name, t.Category, t.Room, t.Brand, t.Model, t.Serial, t.InstalledOn, t.PurchasedOn, nullID(t.Price), t.Vendor,
		t.WarrantyUntil, t.ManualURL, nullID(t.InventoryID), t.InventoryName, t.Notes, boolInt(t.Archived))
	if err != nil {
		return err
	}
	t.ID, _ = res.LastInsertId()
	return nil
}

// UpdateThing saves a thing.
func (s *Store) UpdateThing(ctx context.Context, uid int64, t *Thing) error {
	if err := s.OwnsHome(ctx, uid, t.HomeID); err != nil {
		return err
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE things SET home_id = ?, name = ?, category = ?, room = ?, brand = ?, model = ?, serial = ?, installed_on = ?, purchased_on = ?,
		price = ?, vendor = ?, warranty_until = ?, manual_url = ?, inventory_id = ?, inventory_name = ?, notes = ?, archived = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		t.HomeID, t.Name, t.Category, t.Room, t.Brand, t.Model, t.Serial, t.InstalledOn, t.PurchasedOn, nullID(t.Price), t.Vendor, t.WarrantyUntil, t.ManualURL,
		nullID(t.InventoryID), t.InventoryName, t.Notes, boolInt(t.Archived), Now(), t.ID, uid)
	if err := affected(res, err); err != nil {
		return err
	}
	// its upkeep moves with it
	_, err = s.DB.ExecContext(ctx, `UPDATE jobs SET home_id = ?, cal_synced = '' WHERE user_id = ? AND thing_id = ? AND home_id <> ?`, t.HomeID, uid, t.ID, t.HomeID)
	return err
}

// DeleteThing removes a thing; its log stays (without the thing), its upkeep goes.
func (s *Store) DeleteThing(ctx context.Context, uid, id int64) ([]Job, error) {
	jobs, err := s.ListJobs(ctx, uid, JobFilter{ThingID: id, All: true})
	if err != nil {
		return nil, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx, `DELETE FROM jobs WHERE user_id = ? AND thing_id = ?`, uid, id); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM attachments WHERE user_id = ? AND owner_kind = 'thing' AND owner_id = ?`, uid, id); err != nil {
		return nil, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM things WHERE id = ? AND user_id = ?`, id, uid)
	if err := affected(res, err); err != nil {
		return nil, err
	}
	return jobs, tx.Commit()
}

// Rooms lists the rooms the person has used, most used first.
func (s *Store) Rooms(ctx context.Context, uid int64) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT room FROM things WHERE user_id = ? AND room <> '' GROUP BY room ORDER BY COUNT(1) DESC, room`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---------- pros ----------

const proCols = `id, user_id, name, trade, company, phone, email, website, notes, rating, contact_id, created_at, updated_at`

func scanPro(row interface{ Scan(...any) error }) (*Pro, error) {
	var p Pro
	var c sql.NullInt64
	if err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.Trade, &p.Company, &p.Phone, &p.Email, &p.Website, &p.Notes, &p.Rating, &c, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	p.ContactID = scanID(c)
	return &p, nil
}

// ListPros lists the person's pros with how much work they did.
func (s *Store) ListPros(ctx context.Context, uid int64, q string) ([]Pro, error) {
	query := `SELECT ` + proCols + ` FROM pros WHERE user_id = ?`
	args := []any{uid}
	if q != "" {
		like := "%" + strings.ToLower(q) + "%"
		query += ` AND (lower(name) LIKE ? OR lower(trade) LIKE ? OR lower(company) LIKE ? OR lower(notes) LIKE ?)`
		args = append(args, like, like, like, like)
	}
	rows, err := s.DB.QueryContext(ctx, query+` ORDER BY name COLLATE NOCASE`, args...)
	if err != nil {
		return nil, err
	}
	var out []Pro
	for rows.Next() {
		p, err := scanPro(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		s.proFigures(ctx, uid, &out[i])
	}
	return out, nil
}

func (s *Store) proFigures(ctx context.Context, uid int64, p *Pro) {
	var last sql.NullString
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(1), MAX(date), COALESCE(SUM(cost), 0) FROM records WHERE user_id = ? AND pro_id = ?`, uid, p.ID).Scan(&p.Jobs, &last, &p.Spent)
	p.LastWork = last.String
}

// GetPro reads one pro.
func (s *Store) GetPro(ctx context.Context, uid, id int64) (*Pro, error) {
	p, err := scanPro(s.DB.QueryRowContext(ctx, `SELECT `+proCols+` FROM pros WHERE id = ? AND user_id = ?`, id, uid))
	if err != nil {
		return nil, notFound(err)
	}
	s.proFigures(ctx, uid, p)
	return p, nil
}

// CreatePro adds a pro.
func (s *Store) CreatePro(ctx context.Context, uid int64, p *Pro) error {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO pros (user_id, name, trade, company, phone, email, website, notes, rating, contact_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uid, p.Name, p.Trade, p.Company, p.Phone, p.Email, p.Website, p.Notes, p.Rating, nullID(p.ContactID))
	if err != nil {
		return err
	}
	p.ID, _ = res.LastInsertId()
	return nil
}

// UpdatePro saves a pro.
func (s *Store) UpdatePro(ctx context.Context, uid int64, p *Pro) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE pros SET name = ?, trade = ?, company = ?, phone = ?, email = ?, website = ?, notes = ?, rating = ?, contact_id = ?, updated_at = ?
		WHERE id = ? AND user_id = ?`, p.Name, p.Trade, p.Company, p.Phone, p.Email, p.Website, p.Notes, p.Rating, nullID(p.ContactID), Now(), p.ID, uid)
	return affected(res, err)
}

// DeletePro removes a pro; the work they did stays in the log.
func (s *Store) DeletePro(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM pros WHERE id = ? AND user_id = ?`, id, uid)
	return affected(res, err)
}

// ---------- attachments ----------

// ownerTable maps an attachment owner kind to its table.
var ownerTable = map[string]string{"home": "homes", "thing": "things", "record": "records"}

// ValidOwnerKind reports whether attachments can hang off this kind.
func ValidOwnerKind(k string) bool { _, ok := ownerTable[k]; return ok }

// ListAttachments lists what is kept with one owner.
func (s *Store) ListAttachments(ctx context.Context, uid int64, kind string, id int64) ([]Attachment, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, user_id, owner_kind, owner_id, kind, ref, title, url, created_at FROM attachments
		WHERE user_id = ? AND owner_kind = ? AND owner_id = ? ORDER BY id`, uid, kind, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Attachment{}
	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.ID, &a.UserID, &a.OwnerKind, &a.OwnerID, &a.Kind, &a.Ref, &a.Title, &a.URL, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AllAttachments lists every attachment of a person (the export).
func (s *Store) AllAttachments(ctx context.Context, uid int64) ([]Attachment, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, user_id, owner_kind, owner_id, kind, ref, title, url, created_at FROM attachments WHERE user_id = ? ORDER BY id`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Attachment{}
	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.ID, &a.UserID, &a.OwnerKind, &a.OwnerID, &a.Kind, &a.Ref, &a.Title, &a.URL, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// CreateAttachment keeps a paper or link with something of the person's.
func (s *Store) CreateAttachment(ctx context.Context, uid int64, a *Attachment) error {
	t, ok := ownerTable[a.OwnerKind]
	if !ok {
		return ErrNotFound
	}
	if err := s.owns(ctx, t, uid, &a.OwnerID); err != nil {
		return err
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO attachments (user_id, owner_kind, owner_id, kind, ref, title, url) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uid, a.OwnerKind, a.OwnerID, a.Kind, a.Ref, a.Title, a.URL)
	if err != nil {
		return err
	}
	a.ID, _ = res.LastInsertId()
	a.CreatedAt = Now()
	return nil
}

// DeleteAttachment removes one.
func (s *Store) DeleteAttachment(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM attachments WHERE id = ? AND user_id = ?`, id, uid)
	return affected(res, err)
}
