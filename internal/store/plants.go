package store

import (
	"context"
	"strings"
)

// ---------- plants ----------

const plantCols = `p.id, p.user_id, p.home_id, p.name, p.species, p.kind, p.room, p.light, p.acquired_on, p.pot, p.notes, p.archived, p.gone_reason, p.gone_on,
	p.created_at, p.updated_at, h.name`

const plantFrom = ` FROM plants p JOIN homes h ON h.id = p.home_id`

func scanPlant(row interface{ Scan(...any) error }) (*Plant, error) {
	var p Plant
	if err := row.Scan(&p.ID, &p.UserID, &p.HomeID, &p.Name, &p.Species, &p.Kind, &p.Room, &p.Light, &p.AcquiredOn, &p.Pot, &p.Notes, &p.Archived, &p.GoneReason, &p.GoneOn,
		&p.CreatedAt, &p.UpdatedAt, &p.HomeName); err != nil {
		return nil, err
	}
	p.Care = []Job{}
	return &p, nil
}

// PlantFilter narrows the list of plants.
type PlantFilter struct {
	HomeID   int64
	Room     string
	Q        string
	Archived bool // include archived
}

// ListPlants lists a person's plants, each with its open care.
func (s *Store) ListPlants(ctx context.Context, uid int64, f PlantFilter) ([]Plant, error) {
	q := `SELECT ` + plantCols + plantFrom + ` WHERE p.user_id = ?`
	args := []any{uid}
	if f.HomeID > 0 {
		q += ` AND p.home_id = ?`
		args = append(args, f.HomeID)
	}
	if f.Room != "" {
		q += ` AND p.room = ?`
		args = append(args, f.Room)
	}
	if !f.Archived {
		q += ` AND p.archived = 0`
	}
	if f.Q != "" {
		like := "%" + strings.ToLower(f.Q) + "%"
		q += ` AND (lower(p.name) LIKE ? OR lower(p.species) LIKE ? OR lower(p.room) LIKE ? OR lower(p.notes) LIKE ?)`
		args = append(args, like, like, like, like)
	}
	q += ` ORDER BY p.archived, h.sort, h.id, p.room, p.name COLLATE NOCASE`
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	out := []Plant{}
	for rows.Next() {
		p, err := scanPlant(rows)
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
	if len(out) == 0 {
		return out, nil
	}
	// all care in one query, then hand it out
	care, err := s.ListJobs(ctx, uid, JobFilter{HomeID: f.HomeID, Scope: "plants"})
	if err != nil {
		return nil, err
	}
	at := map[int64]int{}
	for i := range out {
		at[out[i].ID] = i
	}
	for _, j := range care {
		if i, ok := at[*j.PlantID]; ok {
			out[i].Care = append(out[i].Care, j)
		}
	}
	last := s.lastWatered(ctx, uid, 0)
	for i := range out {
		out[i].LastWatered = last[out[i].ID]
		out[i].figures()
	}
	return out, nil
}

// lastWatered is the date of the last watering per plant (one plant, or all with 0).
func (s *Store) lastWatered(ctx context.Context, uid, plantID int64) map[int64]string {
	q := `SELECT r.plant_id, MAX(r.date) FROM records r JOIN jobs j ON j.id = r.job_id WHERE r.user_id = ? AND r.plant_id IS NOT NULL AND j.care = 'water'`
	args := []any{uid}
	if plantID > 0 {
		q += ` AND r.plant_id = ?`
		args = append(args, plantID)
	}
	out := map[int64]string{}
	rows, err := s.DB.QueryContext(ctx, q+` GROUP BY r.plant_id`, args...)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var d string
		if rows.Scan(&id, &d) == nil {
			out[id] = d
		}
	}
	return out
}

// figures sorts the care most urgent first and sums it up: the plant's
// status is that of its most urgent care.
func (p *Plant) figures() {
	care := p.Care
	for i := 1; i < len(care); i++ {
		for k := i; k > 0 && careLess(care[k], care[k-1]); k-- {
			care[k], care[k-1] = care[k-1], care[k]
		}
	}
	p.Status, p.NextCare = StatusOK, ""
	if p.Archived {
		p.Status = StatusPaused
	}
	for _, j := range care {
		if j.Status == StatusPaused || j.Status == StatusDone {
			continue
		}
		if p.NextCare == "" {
			p.NextCare = j.DueOn
			if !p.Archived {
				p.Status = j.Status
			}
		}
	}
	if p.LastWatered == "" {
		for _, j := range care {
			if j.Care == "water" {
				p.LastWatered = j.LastDone
			}
		}
	}
}

func careLess(a, b Job) bool {
	if ra, rb := StatusRank(a.Status), StatusRank(b.Status); ra != rb {
		return ra < rb
	}
	if a.DueOn != b.DueOn {
		return a.DueOn < b.DueOn
	}
	return CareRank(a.Care) < CareRank(b.Care)
}

// CareKinds are the kinds of plant care, in the order they are shown.
var CareKinds = []string{"water", "fertilise", "mist", "rotate", "prune", "pests", "repot"}

// CareRank orders care kinds (unknown last).
func CareRank(c string) int {
	for i, k := range CareKinds {
		if k == c {
			return i
		}
	}
	return len(CareKinds)
}

// GetPlant reads one of the person's plants with its care (paused too).
func (s *Store) GetPlant(ctx context.Context, uid, id int64) (*Plant, error) {
	p, err := scanPlant(s.DB.QueryRowContext(ctx, `SELECT `+plantCols+plantFrom+` WHERE p.id = ? AND p.user_id = ?`, id, uid))
	if err != nil {
		return nil, notFound(err)
	}
	care, err := s.ListJobs(ctx, uid, JobFilter{PlantID: id})
	if err != nil {
		return nil, err
	}
	p.Care = care
	p.LastWatered = s.lastWatered(ctx, uid, id)[id]
	p.figures()
	return p, nil
}

// CreatePlant adds a plant to one of the person's homes.
func (s *Store) CreatePlant(ctx context.Context, uid int64, p *Plant) error {
	if err := s.OwnsHome(ctx, uid, p.HomeID); err != nil {
		return err
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO plants (user_id, home_id, name, species, kind, room, light, acquired_on, pot, notes, archived, gone_reason, gone_on)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, uid, p.HomeID, p.Name, p.Species, p.Kind, p.Room, p.Light, p.AcquiredOn, p.Pot, p.Notes,
		boolInt(p.Archived), p.GoneReason, p.GoneOn)
	if err != nil {
		return err
	}
	p.ID, _ = res.LastInsertId()
	p.UserID = uid
	return nil
}

// UpdatePlant saves a plant. Its care moves with it to another home, and is
// paused while the plant is archived (it died or was given away).
func (s *Store) UpdatePlant(ctx context.Context, uid int64, p *Plant) error {
	if err := s.OwnsHome(ctx, uid, p.HomeID); err != nil {
		return err
	}
	var was bool
	if err := s.DB.QueryRowContext(ctx, `SELECT archived FROM plants WHERE id = ? AND user_id = ?`, p.ID, uid).Scan(&was); err != nil {
		return notFound(err)
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE plants SET home_id = ?, name = ?, species = ?, kind = ?, room = ?, light = ?, acquired_on = ?, pot = ?, notes = ?, archived = ?,
		gone_reason = ?, gone_on = ?, updated_at = ? WHERE id = ? AND user_id = ?`, p.HomeID, p.Name, p.Species, p.Kind, p.Room, p.Light, p.AcquiredOn, p.Pot, p.Notes,
		boolInt(p.Archived), p.GoneReason, p.GoneOn, Now(), p.ID, uid)
	if err := affected(res, err); err != nil {
		return err
	}
	if _, err := s.DB.ExecContext(ctx, `UPDATE jobs SET home_id = ?, cal_synced = '' WHERE user_id = ? AND plant_id = ? AND home_id <> ?`, p.HomeID, uid, p.ID, p.HomeID); err != nil {
		return err
	}
	if was != p.Archived {
		// archived: its care stops (reminders, calendar); back again: it starts
		_, err = s.DB.ExecContext(ctx, `UPDATE jobs SET paused = ?, cal_synced = '', notified_level = '', updated_at = ? WHERE user_id = ? AND plant_id = ?`,
			boolInt(p.Archived), Now(), uid, p.ID)
		return err
	}
	_, err = s.DB.ExecContext(ctx, `UPDATE jobs SET cal_synced = '' WHERE user_id = ? AND plant_id = ?`, uid, p.ID)
	return err
}

// DeletePlant removes a plant with its care and its care log; other log
// entries about it (a new pot that cost money) stay, without the plant.
// It returns the care that was removed (for the calendar).
func (s *Store) DeletePlant(ctx context.Context, uid, id int64) ([]Job, error) {
	jobs, err := s.ListJobs(ctx, uid, JobFilter{PlantID: id, All: true})
	if err != nil {
		return nil, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, q := range []string{
		`DELETE FROM records WHERE user_id = ? AND plant_id = ? AND kind = 'plant' AND money_tx_id IS NULL`,
		`DELETE FROM jobs WHERE user_id = ? AND plant_id = ?`,
	} {
		if _, err := tx.ExecContext(ctx, q, uid, id); err != nil {
			return nil, err
		}
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM plants WHERE id = ? AND user_id = ?`, id, uid)
	if err := affected(res, err); err != nil {
		return nil, err
	}
	return jobs, tx.Commit()
}
