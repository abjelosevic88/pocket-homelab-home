package store

import (
	"context"
	"database/sql"
	"sort"
)

const meterCols = `m.id, m.user_id, m.home_id, m.name, m.kind, m.unit, m.price_per_unit, m.number, m.notes, m.remind_day, m.reminded_on, m.archived, m.created_at, m.updated_at, h.name, h.currency`

func scanMeter(row interface{ Scan(...any) error }) (*Meter, error) {
	var m Meter
	var price sql.NullFloat64
	if err := row.Scan(&m.ID, &m.UserID, &m.HomeID, &m.Name, &m.Kind, &m.Unit, &price, &m.Number, &m.Notes, &m.RemindDay, &m.RemindedOn, &m.Archived, &m.CreatedAt, &m.UpdatedAt,
		&m.HomeName, &m.Currency); err != nil {
		return nil, err
	}
	if price.Valid {
		v := price.Float64
		m.PricePerUnit = &v
	}
	return &m, nil
}

// ListMeters lists a person's meters with their last reading.
func (s *Store) ListMeters(ctx context.Context, uid, homeID int64) ([]Meter, error) {
	q := `SELECT ` + meterCols + ` FROM meters m JOIN homes h ON h.id = m.home_id WHERE m.user_id = ?`
	args := []any{uid}
	if homeID > 0 {
		q += ` AND m.home_id = ?`
		args = append(args, homeID)
	}
	rows, err := s.DB.QueryContext(ctx, q+` ORDER BY m.archived, h.sort, h.id, m.kind, m.name`, args...)
	if err != nil {
		return nil, err
	}
	out := []Meter{}
	for rows.Next() {
		m, err := scanMeter(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		s.meterLast(ctx, uid, &out[i])
	}
	return out, nil
}

func (s *Store) meterLast(ctx context.Context, uid int64, m *Meter) {
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM readings WHERE user_id = ? AND meter_id = ?`, uid, m.ID).Scan(&m.Readings)
	rs, err := s.ListReadings(ctx, uid, m.ID, 2)
	if err == nil && len(rs) > 0 {
		last := rs[len(rs)-1]
		m.Last = &last
	}
}

// GetMeter reads one meter.
func (s *Store) GetMeter(ctx context.Context, uid, id int64) (*Meter, error) {
	m, err := scanMeter(s.DB.QueryRowContext(ctx, `SELECT `+meterCols+` FROM meters m JOIN homes h ON h.id = m.home_id WHERE m.id = ? AND m.user_id = ?`, id, uid))
	if err != nil {
		return nil, notFound(err)
	}
	s.meterLast(ctx, uid, m)
	return m, nil
}

// CreateMeter adds a meter.
func (s *Store) CreateMeter(ctx context.Context, uid int64, m *Meter) error {
	if err := s.OwnsHome(ctx, uid, m.HomeID); err != nil {
		return err
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO meters (user_id, home_id, name, kind, unit, price_per_unit, number, notes, remind_day, archived) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uid, m.HomeID, m.Name, m.Kind, m.Unit, m.PricePerUnit, m.Number, m.Notes, m.RemindDay, boolInt(m.Archived))
	if err != nil {
		return err
	}
	m.ID, _ = res.LastInsertId()
	return nil
}

// UpdateMeter saves a meter.
func (s *Store) UpdateMeter(ctx context.Context, uid int64, m *Meter) error {
	if err := s.OwnsHome(ctx, uid, m.HomeID); err != nil {
		return err
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE meters SET home_id = ?, name = ?, kind = ?, unit = ?, price_per_unit = ?, number = ?, notes = ?, remind_day = ?, archived = ?, updated_at = ?
		WHERE id = ? AND user_id = ?`, m.HomeID, m.Name, m.Kind, m.Unit, m.PricePerUnit, m.Number, m.Notes, m.RemindDay, boolInt(m.Archived), Now(), m.ID, uid)
	return affected(res, err)
}

// DeleteMeter removes a meter and its readings.
func (s *Store) DeleteMeter(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM meters WHERE id = ? AND user_id = ?`, id, uid)
	return affected(res, err)
}

// SetMeterReminded remembers the reading reminder went out.
func (s *Store) SetMeterReminded(ctx context.Context, uid, id int64, on string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE meters SET reminded_on = ? WHERE id = ? AND user_id = ?`, on, id, uid)
	return err
}

// ListReadings returns a meter's readings oldest first with the usage since
// the reading before; last > 0 keeps only the last n (usage still counted).
func (s *Store) ListReadings(ctx context.Context, uid, meterID int64, last int) ([]Reading, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, user_id, meter_id, read_on, value, reset, note, source, created_at FROM readings
		WHERE user_id = ? AND meter_id = ? ORDER BY read_on, id`, uid, meterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Reading{}
	for rows.Next() {
		var r Reading
		if err := rows.Scan(&r.ID, &r.UserID, &r.MeterID, &r.ReadOn, &r.Value, &r.Reset, &r.Note, &r.Source, &r.CreatedAt); err != nil {
			return nil, err
		}
		if n := len(out); n > 0 && !r.Reset {
			u := r.Value - out[n-1].Value
			r.Usage = &u
			r.Days = DaysBetween(out[n-1].ReadOn, r.ReadOn)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if last > 0 && len(out) > last {
		out = out[len(out)-last:]
	}
	return out, nil
}

// GetReading reads one reading.
func (s *Store) GetReading(ctx context.Context, uid, id int64) (*Reading, error) {
	var r Reading
	err := s.DB.QueryRowContext(ctx, `SELECT id, user_id, meter_id, read_on, value, reset, note, source, created_at FROM readings WHERE id = ? AND user_id = ?`, id, uid).
		Scan(&r.ID, &r.UserID, &r.MeterID, &r.ReadOn, &r.Value, &r.Reset, &r.Note, &r.Source, &r.CreatedAt)
	return &r, notFound(err)
}

// CreateReading adds a reading to one of the person's meters.
func (s *Store) CreateReading(ctx context.Context, uid int64, r *Reading) error {
	if err := s.owns(ctx, "meters", uid, &r.MeterID); err != nil {
		return err
	}
	if r.Source == "" {
		r.Source = "web"
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO readings (user_id, meter_id, read_on, value, reset, note, source) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uid, r.MeterID, r.ReadOn, r.Value, boolInt(r.Reset), r.Note, r.Source)
	if err != nil {
		return err
	}
	r.ID, _ = res.LastInsertId()
	return nil
}

// UpdateReading saves a reading.
func (s *Store) UpdateReading(ctx context.Context, uid int64, r *Reading) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE readings SET read_on = ?, value = ?, reset = ?, note = ? WHERE id = ? AND user_id = ?`, r.ReadOn, r.Value, boolInt(r.Reset), r.Note, r.ID, uid)
	return affected(res, err)
}

// DeleteReading removes a reading.
func (s *Store) DeleteReading(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM readings WHERE id = ? AND user_id = ?`, id, uid)
	return affected(res, err)
}

// UsageMonth is a meter's usage in one calendar month.
type UsageMonth struct {
	Month     string   `json:"month"` // YYYY-MM
	Usage     float64  `json:"usage"`
	Cost      *float64 `json:"cost"`      // usage × price, major units
	Estimated bool     `json:"estimated"` // spread from readings that straddle months
}

// MonthlyUsage spreads the usage between readings evenly over the days in
// between and sums it per calendar month, for the last n months.
func MonthlyUsage(rs []Reading, price *float64, months int, today string) []UsageMonth {
	per := map[string]float64{}
	exact := map[string]bool{}
	for _, r := range rs {
		if r.Usage == nil || r.Days <= 0 {
			continue
		}
		daily := *r.Usage / float64(r.Days)
		start := AddDays(r.ReadOn, -r.Days)
		for d := 0; d < r.Days; d++ {
			day := AddDays(start, d)
			per[day[:7]] += daily
		}
		if r.Days <= 31 {
			exact[r.ReadOn[:7]] = true
		}
	}
	keys := make([]string, 0, months)
	t, err := ParseDate(today)
	if err != nil {
		return nil
	}
	first := addMonths(t, -(months - 1)).Format("2006-01")
	for m := first; len(keys) < months; {
		keys = append(keys, m)
		mt, _ := ParseDate(m + "-01")
		m = addMonths(mt, 1).Format("2006-01")
	}
	sort.Strings(keys)
	out := make([]UsageMonth, 0, len(keys))
	for _, k := range keys {
		u := UsageMonth{Month: k, Usage: round(per[k], 2), Estimated: !exact[k] && per[k] > 0}
		if price != nil && per[k] > 0 {
			c := round(per[k]**price, 2)
			u.Cost = &c
		}
		out = append(out, u)
	}
	return out
}

func round(v float64, d int) float64 {
	p := 1.0
	for i := 0; i < d; i++ {
		p *= 10
	}
	if v < 0 {
		return -float64(int64(-v*p+0.5)) / p
	}
	return float64(int64(v*p+0.5)) / p
}
