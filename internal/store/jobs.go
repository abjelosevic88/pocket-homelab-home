package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// Units a job can repeat in.
var Units = []string{"day", "week", "month", "year"}

// ValidUnit reports whether u is a repeat unit.
func ValidUnit(u string) bool {
	for _, x := range Units {
		if x == u {
			return true
		}
	}
	return false
}

// AddInterval moves a date on by n units. Months and years keep the day,
// clamped to the end of a shorter month (31 Jan + 1 month = 28/29 Feb).
func AddInterval(date string, n int, unit string) string {
	t, err := ParseDate(date)
	if err != nil || n <= 0 {
		return date
	}
	switch unit {
	case "day":
		t = t.AddDate(0, 0, n)
	case "week":
		t = t.AddDate(0, 0, 7*n)
	case "year":
		t = addMonths(t, 12*n)
	default:
		t = addMonths(t, n)
	}
	return t.Format("2006-01-02")
}

func addMonths(t time.Time, n int) time.Time {
	y, m, d := t.Date()
	first := time.Date(y, m+time.Month(n), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	if d > last {
		d = last
	}
	return time.Date(first.Year(), first.Month(), d, 0, 0, 0, 0, time.UTC)
}

// DaysBetween is b - a in whole days.
func DaysBetween(a, b string) int {
	ta, err1 := ParseDate(a)
	tb, err2 := ParseDate(b)
	if err1 != nil || err2 != nil {
		return 0
	}
	return int(tb.Sub(ta).Hours() / 24)
}

// Statuses, most urgent first.
const (
	StatusOverdue = "overdue"
	StatusDue     = "due"
	StatusSoon    = "soon"
	StatusOK      = "ok"
	StatusPaused  = "paused"
	StatusDone    = "done"
)

// StatusRank orders statuses by urgency (lower = more urgent).
func StatusRank(s string) int {
	switch s {
	case StatusOverdue:
		return 0
	case StatusDue:
		return 1
	case StatusSoon:
		return 2
	case StatusOK:
		return 3
	case StatusPaused:
		return 4
	}
	return 5
}

// SetStatus fills Status and DaysLeft from the due date and today.
func (j *Job) SetStatus(today string) {
	j.DaysLeft = DaysBetween(today, j.DueOn)
	switch {
	case j.FinishedAt != "":
		j.Status = StatusDone
	case j.Paused:
		j.Status = StatusPaused
	case j.DaysLeft < 0:
		j.Status = StatusOverdue
	case j.DaysLeft == 0:
		j.Status = StatusDue
	case j.DaysLeft <= j.SoonDays:
		j.Status = StatusSoon
	default:
		j.Status = StatusOK
	}
}

const jobCols = `j.id, j.user_id, j.home_id, j.thing_id, j.plant_id, j.care, j.pro_id, j.title, j.notes, j.every_n, j.every_unit, j.due_on, j.soon_days, j.est_cost, j.last_done,
	j.done_count, j.paused, j.finished_at, j.notified_level, j.notified_on, j.cal_event_id, j.cal_synced, j.task_id, j.source, j.created_at, j.updated_at,
	h.name, h.currency, COALESCE(t.name, ''), COALESCE(pl.name, ''), COALESCE(p.name, '')`

const jobFrom = ` FROM jobs j JOIN homes h ON h.id = j.home_id LEFT JOIN things t ON t.id = j.thing_id LEFT JOIN plants pl ON pl.id = j.plant_id LEFT JOIN pros p ON p.id = j.pro_id`

func scanJob(row interface{ Scan(...any) error }) (*Job, error) {
	var j Job
	var thing, plant, pro, est, cal, task sql.NullInt64
	if err := row.Scan(&j.ID, &j.UserID, &j.HomeID, &thing, &plant, &j.Care, &pro, &j.Title, &j.Notes, &j.EveryN, &j.EveryUnit, &j.DueOn, &j.SoonDays, &est, &j.LastDone,
		&j.DoneCount, &j.Paused, &j.FinishedAt, &j.NotifiedLevel, &j.NotifiedOn, &cal, &j.CalSynced, &task, &j.Source, &j.CreatedAt, &j.UpdatedAt,
		&j.HomeName, &j.Currency, &j.ThingName, &j.PlantName, &j.ProName); err != nil {
		return nil, err
	}
	j.PlantID = scanID(plant)
	j.ThingID, j.ProID, j.EstCost, j.CalEventID, j.TaskID = scanID(thing), scanID(pro), scanID(est), scanID(cal), scanID(task)
	j.SetStatus(Today())
	return &j, nil
}

// JobFilter narrows the upkeep list.
type JobFilter struct {
	HomeID  int64
	ThingID int64
	PlantID int64
	Scope   string // "" everything | "house" upkeep only | "plants" plant care only
	Q       string
	All     bool // include finished one-offs
	Until   string
}

// ListJobs lists a person's upkeep, soonest first.
func (s *Store) ListJobs(ctx context.Context, uid int64, f JobFilter) ([]Job, error) {
	q := `SELECT ` + jobCols + jobFrom + ` WHERE j.user_id = ?`
	args := []any{uid}
	if f.HomeID > 0 {
		q += ` AND j.home_id = ?`
		args = append(args, f.HomeID)
	}
	if f.ThingID > 0 {
		q += ` AND j.thing_id = ?`
		args = append(args, f.ThingID)
	}
	if f.PlantID > 0 {
		q += ` AND j.plant_id = ?`
		args = append(args, f.PlantID)
	}
	switch f.Scope {
	case "house":
		q += ` AND j.plant_id IS NULL`
	case "plants":
		q += ` AND j.plant_id IS NOT NULL`
	}
	if !f.All {
		q += ` AND j.finished_at = ''`
	}
	if f.Until != "" {
		q += ` AND j.due_on <= ? AND j.paused = 0`
		args = append(args, f.Until)
	}
	if f.Q != "" {
		like := "%" + strings.ToLower(f.Q) + "%"
		q += ` AND (lower(j.title) LIKE ? OR lower(j.notes) LIKE ? OR lower(COALESCE(t.name, '')) LIKE ? OR lower(COALESCE(pl.name, '')) LIKE ?)`
		args = append(args, like, like, like, like)
	}
	q += ` ORDER BY j.finished_at <> '', j.paused, j.due_on, j.id`
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}

// ActiveJobs lists every open job of every person (the daily check).
func (s *Store) ActiveJobs(ctx context.Context, uid int64) ([]Job, error) {
	return s.ListJobs(ctx, uid, JobFilter{})
}

// GetJob reads one job.
func (s *Store) GetJob(ctx context.Context, uid, id int64) (*Job, error) {
	j, err := scanJob(s.DB.QueryRowContext(ctx, `SELECT `+jobCols+jobFrom+` WHERE j.id = ? AND j.user_id = ?`, id, uid))
	return j, notFound(err)
}

func (s *Store) jobRefs(ctx context.Context, uid int64, j *Job) error {
	if err := s.OwnsHome(ctx, uid, j.HomeID); err != nil {
		return err
	}
	if err := s.owns(ctx, "things", uid, j.ThingID); err != nil {
		return err
	}
	if err := s.owns(ctx, "plants", uid, j.PlantID); err != nil {
		return err
	}
	return s.owns(ctx, "pros", uid, j.ProID)
}

// CreateJob adds upkeep.
func (s *Store) CreateJob(ctx context.Context, uid int64, j *Job) error {
	if err := s.jobRefs(ctx, uid, j); err != nil {
		return err
	}
	if j.Source == "" {
		j.Source = "web"
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO jobs (user_id, home_id, thing_id, plant_id, care, pro_id, title, notes, every_n, every_unit, due_on, soon_days, est_cost, last_done, done_count, paused, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, uid, j.HomeID, nullID(j.ThingID), nullID(j.PlantID), j.Care, nullID(j.ProID), j.Title, j.Notes, j.EveryN, j.EveryUnit, j.DueOn,
		j.SoonDays, nullID(j.EstCost), j.LastDone, j.DoneCount, boolInt(j.Paused), j.Source)
	if err != nil {
		return err
	}
	j.ID, _ = res.LastInsertId()
	return nil
}

// UpdateJob saves a job's own fields (not what Done/Calendar/Tasks keep).
func (s *Store) UpdateJob(ctx context.Context, uid int64, j *Job) error {
	if err := s.jobRefs(ctx, uid, j); err != nil {
		return err
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE jobs SET home_id = ?, thing_id = ?, plant_id = ?, care = ?, pro_id = ?, title = ?, notes = ?, every_n = ?, every_unit = ?, due_on = ?, soon_days = ?,
		est_cost = ?, paused = ?, finished_at = ?, last_done = ?, done_count = ?, notified_level = CASE WHEN due_on <> ? THEN '' ELSE notified_level END, updated_at = ?
		WHERE id = ? AND user_id = ?`, j.HomeID, nullID(j.ThingID), nullID(j.PlantID), j.Care, nullID(j.ProID), j.Title, j.Notes, j.EveryN, j.EveryUnit, j.DueOn, j.SoonDays,
		nullID(j.EstCost), boolInt(j.Paused), j.FinishedAt, j.LastDone, j.DoneCount, j.DueOn, Now(), j.ID, uid)
	return affected(res, err)
}

// DeleteJob removes a job; what was logged for it stays.
func (s *Store) DeleteJob(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM jobs WHERE id = ? AND user_id = ?`, id, uid)
	return affected(res, err)
}

// SetJobNotified remembers the last push for a job.
func (s *Store) SetJobNotified(ctx context.Context, uid, id int64, level, on string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE jobs SET notified_level = ?, notified_on = ? WHERE id = ? AND user_id = ?`, level, on, id, uid)
	return err
}

// SetJobCalendar remembers the job's calendar event and what it shows.
func (s *Store) SetJobCalendar(ctx context.Context, uid, id int64, event *int64, synced string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE jobs SET cal_event_id = ?, cal_synced = ? WHERE id = ? AND user_id = ?`, nullID(event), synced, id, uid)
	return err
}

// SetJobTask remembers the task made for a job in Pocket Tasks.
func (s *Store) SetJobTask(ctx context.Context, uid, id int64, task *int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE jobs SET task_id = ? WHERE id = ? AND user_id = ?`, nullID(task), id, uid)
	return err
}

// CompleteJob marks a job done on a date: a recurring job moves to its next
// due date (counted from the day it was done), a one-off is finished. The
// log entry, if any, is written in the same transaction.
func (s *Store) CompleteJob(ctx context.Context, uid int64, j *Job, date string, rec *Record) error {
	// one connection: check before the transaction holds it
	if rec != nil {
		if err := s.recordRefs(ctx, uid, rec); err != nil {
			return err
		}
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if rec != nil {
		if err := s.insertRecord(ctx, tx, uid, rec); err != nil {
			return err
		}
	}
	j.LastDone, j.DoneCount = date, j.DoneCount+1
	if j.Recurring() {
		j.DueOn = AddInterval(date, j.EveryN, j.EveryUnit)
	} else {
		j.FinishedAt = Now()
	}
	if _, err := tx.ExecContext(ctx, `UPDATE jobs SET last_done = ?, done_count = ?, due_on = ?, finished_at = ?, notified_level = '', cal_synced = '', updated_at = ? WHERE id = ? AND user_id = ?`,
		j.LastDone, j.DoneCount, j.DueOn, j.FinishedAt, Now(), j.ID, uid); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	j.SetStatus(Today())
	return nil
}

// ---------- the log ----------

const recordCols = `r.id, r.user_id, r.home_id, r.thing_id, r.plant_id, r.job_id, r.pro_id, r.date, r.kind, r.title, r.notes, r.cost, r.currency, r.money_tx_id, r.money_user_id,
	r.source, r.external_id, r.created_at, r.updated_at, h.name, COALESCE(t.name, ''), COALESCE(pl.name, ''), COALESCE(p.name, ''), COALESCE(j.title, '')`

const recordFrom = ` FROM records r JOIN homes h ON h.id = r.home_id LEFT JOIN things t ON t.id = r.thing_id LEFT JOIN plants pl ON pl.id = r.plant_id LEFT JOIN pros p ON p.id = r.pro_id LEFT JOIN jobs j ON j.id = r.job_id`

func scanRecord(row interface{ Scan(...any) error }) (*Record, error) {
	var r Record
	var thing, plant, job, pro, tx, txu sql.NullInt64
	if err := row.Scan(&r.ID, &r.UserID, &r.HomeID, &thing, &plant, &job, &pro, &r.Date, &r.Kind, &r.Title, &r.Notes, &r.Cost, &r.Currency, &tx, &txu,
		&r.Source, &r.ExternalID, &r.CreatedAt, &r.UpdatedAt, &r.HomeName, &r.ThingName, &r.PlantName, &r.ProName, &r.JobTitle); err != nil {
		return nil, err
	}
	r.PlantID = scanID(plant)
	r.ThingID, r.JobID, r.ProID, r.MoneyTxID, r.MoneyUserID = scanID(thing), scanID(job), scanID(pro), scanID(tx), scanID(txu)
	return &r, nil
}

// RecordFilter narrows the log.
type RecordFilter struct {
	HomeID  int64
	ThingID int64
	PlantID int64
	Scope   string // "" everything | "house" without plant care | "plants" plant care only
	ProID   int64
	JobID   int64
	Kind    string
	From    string
	To      string
	Q       string
	Limit   int
	Offset  int
}

// ListRecords lists the log, newest first, with the total count.
func (s *Store) ListRecords(ctx context.Context, uid int64, f RecordFilter) ([]Record, int, error) {
	where := ` WHERE r.user_id = ?`
	args := []any{uid}
	add := func(cond string, v any) { where += cond; args = append(args, v) }
	if f.HomeID > 0 {
		add(` AND r.home_id = ?`, f.HomeID)
	}
	if f.ThingID > 0 {
		add(` AND r.thing_id = ?`, f.ThingID)
	}
	if f.PlantID > 0 {
		add(` AND r.plant_id = ?`, f.PlantID)
	}
	switch f.Scope {
	case "house":
		where += ` AND r.kind <> 'plant'`
	case "plants":
		where += ` AND (r.kind = 'plant' OR r.plant_id IS NOT NULL)`
	}
	if f.ProID > 0 {
		add(` AND r.pro_id = ?`, f.ProID)
	}
	if f.JobID > 0 {
		add(` AND r.job_id = ?`, f.JobID)
	}
	if f.Kind != "" {
		add(` AND r.kind = ?`, f.Kind)
	}
	if f.From != "" {
		add(` AND r.date >= ?`, f.From)
	}
	if f.To != "" {
		add(` AND r.date <= ?`, f.To)
	}
	if f.Q != "" {
		like := "%" + strings.ToLower(f.Q) + "%"
		where += ` AND (lower(r.title) LIKE ? OR lower(r.notes) LIKE ? OR lower(COALESCE(t.name, '')) LIKE ? OR lower(COALESCE(pl.name, '')) LIKE ? OR lower(COALESCE(p.name, '')) LIKE ?)`
		args = append(args, like, like, like, like, like)
	}
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(1)`+recordFrom+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT ` + recordCols + recordFrom + where + ` ORDER BY r.date DESC, r.id DESC`
	if f.Limit > 0 {
		q += ` LIMIT ? OFFSET ?`
		args = append(args, f.Limit, f.Offset)
	}
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		r, err := scanRecord(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *r)
	}
	return out, total, rows.Err()
}

// GetRecord reads one log entry.
func (s *Store) GetRecord(ctx context.Context, uid, id int64) (*Record, error) {
	r, err := scanRecord(s.DB.QueryRowContext(ctx, `SELECT `+recordCols+recordFrom+` WHERE r.id = ? AND r.user_id = ?`, id, uid))
	return r, notFound(err)
}

// RecordByExternalID finds an imported entry.
func (s *Store) RecordByExternalID(ctx context.Context, uid int64, ext string) (*Record, error) {
	r, err := scanRecord(s.DB.QueryRowContext(ctx, `SELECT `+recordCols+recordFrom+` WHERE r.user_id = ? AND r.external_id = ?`, uid, ext))
	return r, notFound(err)
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (s *Store) recordRefs(ctx context.Context, uid int64, r *Record) error {
	if err := s.OwnsHome(ctx, uid, r.HomeID); err != nil {
		return err
	}
	if err := s.owns(ctx, "things", uid, r.ThingID); err != nil {
		return err
	}
	if err := s.owns(ctx, "plants", uid, r.PlantID); err != nil {
		return err
	}
	if err := s.owns(ctx, "jobs", uid, r.JobID); err != nil {
		return err
	}
	return s.owns(ctx, "pros", uid, r.ProID)
}

// insertRecord writes a checked entry (recordRefs first).
func (s *Store) insertRecord(ctx context.Context, x execer, uid int64, r *Record) error {
	if r.Source == "" {
		r.Source = "web"
	}
	res, err := x.ExecContext(ctx, `INSERT INTO records (user_id, home_id, thing_id, plant_id, job_id, pro_id, date, kind, title, notes, cost, currency, source, external_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, uid, r.HomeID, nullID(r.ThingID), nullID(r.PlantID), nullID(r.JobID), nullID(r.ProID), r.Date, r.Kind, r.Title, r.Notes,
		r.Cost, r.Currency, r.Source, r.ExternalID)
	if err != nil {
		return conflict(err)
	}
	r.ID, _ = res.LastInsertId()
	return nil
}

// CreateRecord adds a log entry.
func (s *Store) CreateRecord(ctx context.Context, uid int64, r *Record) error {
	if err := s.recordRefs(ctx, uid, r); err != nil {
		return err
	}
	return s.insertRecord(ctx, s.DB, uid, r)
}

// UpdateRecord saves a log entry.
func (s *Store) UpdateRecord(ctx context.Context, uid int64, r *Record) error {
	if err := s.recordRefs(ctx, uid, r); err != nil {
		return err
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE records SET home_id = ?, thing_id = ?, plant_id = ?, job_id = ?, pro_id = ?, date = ?, kind = ?, title = ?, notes = ?, cost = ?, currency = ?, updated_at = ?
		WHERE id = ? AND user_id = ?`, r.HomeID, nullID(r.ThingID), nullID(r.PlantID), nullID(r.JobID), nullID(r.ProID), r.Date, r.Kind, r.Title, r.Notes, r.Cost, r.Currency, Now(), r.ID, uid)
	return affected(res, err)
}

// SetRecordMoney remembers (or forgets, with nil) the Pocket Money booking.
func (s *Store) SetRecordMoney(ctx context.Context, uid, id int64, tx *int64) error {
	var who any
	if tx != nil {
		who = uid
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE records SET money_tx_id = ?, money_user_id = ? WHERE id = ? AND user_id = ?`, nullID(tx), who, id, uid)
	return err
}

// DeleteRecord removes a log entry and what was kept with it.
func (s *Store) DeleteRecord(ctx context.Context, uid, id int64) error {
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM attachments WHERE user_id = ? AND owner_kind = 'record' AND owner_id = ?`, uid, id); err != nil {
		return err
	}
	res, err := s.DB.ExecContext(ctx, `DELETE FROM records WHERE id = ? AND user_id = ?`, id, uid)
	return affected(res, err)
}

// SpendRow is the money spent in one bucket.
type SpendRow struct {
	Key      string `json:"key"`
	Currency string `json:"currency"`
	Total    int64  `json:"total"`
	Count    int    `json:"count"`
}

// Spending sums the log by month, kind or thing between two dates.
// by is "month" | "kind" | "year" | "thing"; never user input in SQL.
func (s *Store) Spending(ctx context.Context, uid, homeID int64, by, from, to string) ([]SpendRow, error) {
	key := map[string]string{"month": "substr(r.date, 1, 7)", "year": "substr(r.date, 1, 4)", "kind": "r.kind", "thing": "COALESCE(t.name, '')"}[by]
	if key == "" {
		key = "substr(r.date, 1, 7)"
	}
	q := `SELECT ` + key + `, r.currency, COALESCE(SUM(r.cost), 0), COUNT(1) FROM records r LEFT JOIN things t ON t.id = r.thing_id
		WHERE r.user_id = ? AND r.date >= ? AND r.date <= ?`
	args := []any{uid, from, to}
	if homeID > 0 {
		q += ` AND r.home_id = ?`
		args = append(args, homeID)
	}
	rows, err := s.DB.QueryContext(ctx, q+` GROUP BY 1, 2 ORDER BY 1`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SpendRow{}
	for rows.Next() {
		var r SpendRow
		if err := rows.Scan(&r.Key, &r.Currency, &r.Total, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
