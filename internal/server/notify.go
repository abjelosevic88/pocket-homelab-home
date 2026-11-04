package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-home/internal/notify"
	"github.com/abjelosevic88/pocket-homelab-home/internal/store"
)

// pushes runs every minute; once a day, at each person's own hour, it sends
// one message with the upkeep that became due (soon → due → overdue, plus
// a Monday reminder for what is still overdue), warranties ending within 30
// days and the meters to read today.
func (s *Server) pushes(ctx context.Context) {
	ids, err := s.store.SettingsUsers(ctx)
	if err != nil {
		return
	}
	now := time.Now()
	for _, uid := range ids {
		st, err := s.store.GetSettings(ctx, uid)
		if err != nil || now.Hour() != st.NotifyHour || now.Minute() > 10 {
			continue
		}
		key := fmt.Sprintf("push:%d:%s", uid, store.Today())
		if _, _, done := s.store.CacheGet(ctx, key); done {
			continue
		}
		_ = s.store.CachePut(ctx, key, "1")
		s.dailyCheck(ctx, uid, st, now)
	}
}

var levelRank = map[string]int{"": 0, store.StatusOK: 0, store.StatusSoon: 1, store.StatusDue: 2, store.StatusOverdue: 3}

// dailyCheck builds and sends one person's daily message; it also publishes
// home.job_due and home.warranty_ending, notifications on or off.
func (s *Server) dailyCheck(ctx context.Context, uid int64, st store.Settings, now time.Time) (string, bool) {
	today := store.Today()
	var lines []string
	worst := "info"
	{
		jobs, _ := s.store.ActiveJobs(ctx, uid)
		monday := now.Weekday() == time.Monday
		for _, j := range jobs {
			if j.Paused || levelRank[j.Status] == 0 {
				continue
			}
			rose := levelRank[j.Status] > levelRank[j.NotifiedLevel]
			again := j.Status == store.StatusOverdue && monday && j.NotifiedOn != today
			if !rose && !again {
				continue
			}
			_ = s.store.SetJobNotified(ctx, uid, j.ID, j.Status, today)
			if rose && j.Status != store.StatusSoon {
				s.publish(ctx, uid, "home.job_due", "job:"+itoa(j.ID), map[string]any{"job_id": j.ID, "title": j.Title, "thing": j.ThingName, "home": j.HomeName,
					"due_on": j.DueOn, "status": j.Status, "days_left": j.DaysLeft})
			}
			if !st.NotifyUpkeep {
				continue
			}
			name := j.Title
			if j.ThingName != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(j.ThingName)) {
				name = j.ThingName + ": " + name
			}
			switch j.Status {
			case store.StatusOverdue:
				lines = append(lines, fmt.Sprintf("• %s — overdue by %d %s", name, -j.DaysLeft, dayWord(-j.DaysLeft)))
				worst = "warning"
			case store.StatusDue:
				lines = append(lines, "• "+name+" — due today")
			default:
				lines = append(lines, fmt.Sprintf("• %s — due in %d %s", name, j.DaysLeft, dayWord(j.DaysLeft)))
			}
		}
	}
	things, _ := s.store.ListThings(ctx, uid, store.ThingFilter{})
	for _, t := range things {
		if t.WarrantyUntil == "" || t.WarrantyUntil < today || store.DaysBetween(today, t.WarrantyUntil) > 30 {
			continue
		}
		k := fmt.Sprintf("warranty:%d:%d:%s", uid, t.ID, t.WarrantyUntil)
		if _, _, done := s.store.CacheGet(ctx, k); done {
			continue
		}
		_ = s.store.CachePut(ctx, k, "1")
		s.publish(ctx, uid, "home.warranty_ending", "thing:"+itoa(t.ID), map[string]any{"thing_id": t.ID, "thing": t.Name, "home": t.HomeName, "until": t.WarrantyUntil})
		if st.NotifyWarranty {
			lines = append(lines, fmt.Sprintf("• The warranty on %s ends on %s", t.Name, t.WarrantyUntil))
		}
	}
	if st.NotifyMeters {
		meters, _ := s.store.ListMeters(ctx, uid, 0)
		for _, m := range meters {
			if m.Archived || m.RemindDay == 0 || m.RemindedOn == today || !readingDay(m.RemindDay, now) {
				continue
			}
			if m.Last != nil && store.DaysBetween(m.Last.ReadOn, today) < 5 {
				continue
			}
			_ = s.store.SetMeterReminded(ctx, uid, m.ID, today)
			lines = append(lines, "• Read the "+strings.ToLower(m.Name)+" meter")
		}
	}
	if len(lines) == 0 || st.NotifyURL == "" {
		return "", false
	}
	body := strings.Join(lines, "\n")
	n := notify.New(st.NotifyURL, st.NotifyFormat)
	if err := n.Send(ctx, notify.Message{Title: "Your home today", Body: body, Priority: worst, Tags: []string{"house"}, Link: s.cfg.BaseURL + "/"}); err != nil {
		s.log.Warn("notify", "user", uid, "err", err)
		return body, false
	}
	return body, true
}

func dayWord(n int) string {
	if n == 1 {
		return "day"
	}
	return "days"
}

// readingDay: the chosen day of the month, or the last day when the month is shorter.
func readingDay(day int, now time.Time) bool {
	last := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()
	if day > last {
		day = last
	}
	return now.Day() == day
}

func (s *Server) handleNotifyTest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st, _ := s.store.GetSettings(ctx, actorID(ctx))
	n := notify.New(st.NotifyURL, st.NotifyFormat)
	if err := n.Send(ctx, notify.Message{Title: "Pocket Home", Body: "Notifications work. Upkeep that falls due, warranties that end and meter readings will arrive here.", Priority: "info", Tags: []string{"house"}}); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
