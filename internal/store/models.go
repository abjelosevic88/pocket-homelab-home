package store

import "strings"

// User is an account on this box.
type User struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"` // admin | member
	CreatedAt    string `json:"created_at"`
}

// APIToken is a personal access token (hash stored, plaintext shown once).
type APIToken struct {
	ID         int64   `json:"id"`
	UserID     int64   `json:"user_id"`
	Name       string  `json:"name"`
	Prefix     string  `json:"prefix"`
	LastUsedAt *string `json:"last_used_at"`
	CreatedAt  string  `json:"created_at"`
}

// Home is a place someone looks after.
type Home struct {
	ID        int64    `json:"id"`
	UserID    int64    `json:"-"`
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Address   string   `json:"address"`
	YearBuilt *int     `json:"year_built"`
	AreaM2    *float64 `json:"area_m2"`
	Currency  string   `json:"currency"`
	Notes     string   `json:"notes"`
	MoneyTag  string   `json:"money_tag"`
	Archived  bool     `json:"archived"`
	Sort      int      `json:"sort"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`

	Things  int `json:"things"`
	Open    int `json:"open_jobs"`
	Overdue int `json:"overdue"`
	DueSoon int `json:"due_soon"`
}

// Thing is an appliance or a part of the house.
type Thing struct {
	ID            int64  `json:"id"`
	UserID        int64  `json:"-"`
	HomeID        int64  `json:"home_id"`
	Name          string `json:"name"`
	Category      string `json:"category"`
	Room          string `json:"room"`
	Brand         string `json:"brand"`
	Model         string `json:"model"`
	Serial        string `json:"serial"`
	InstalledOn   string `json:"installed_on"`
	PurchasedOn   string `json:"purchased_on"`
	Price         *int64 `json:"price"`
	Vendor        string `json:"vendor"`
	WarrantyUntil string `json:"warranty_until"`
	ManualURL     string `json:"manual_url"`
	InventoryID   *int64 `json:"inventory_id"`
	InventoryName string `json:"inventory_name"`
	Notes         string `json:"notes"`
	Archived      bool   `json:"archived"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`

	HomeName string `json:"home_name"`
	Currency string `json:"currency"`
	Open     int    `json:"open_jobs"`
	Overdue  int    `json:"overdue"`
	NextDue  string `json:"next_due"`
	LastDone string `json:"last_done"`
	Spent    int64  `json:"spent"`
}

// Pro is someone who works on the house.
type Pro struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"-"`
	Name      string `json:"name"`
	Trade     string `json:"trade"`
	Company   string `json:"company"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Website   string `json:"website"`
	Notes     string `json:"notes"`
	Rating    int    `json:"rating"`
	ContactID *int64 `json:"contact_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`

	Jobs     int    `json:"jobs"`
	LastWork string `json:"last_work"`
	Spent    int64  `json:"spent"`
}

// Job is upkeep: something to do every so often, or once.
type Job struct {
	ID            int64  `json:"id"`
	UserID        int64  `json:"-"`
	HomeID        int64  `json:"home_id"`
	ThingID       *int64 `json:"thing_id"`
	ProID         *int64 `json:"pro_id"`
	Title         string `json:"title"`
	Notes         string `json:"notes"`
	EveryN        int    `json:"every_n"`
	EveryUnit     string `json:"every_unit"`
	DueOn         string `json:"due_on"`
	SoonDays      int    `json:"soon_days"`
	EstCost       *int64 `json:"est_cost"`
	LastDone      string `json:"last_done"`
	DoneCount     int    `json:"done_count"`
	Paused        bool   `json:"paused"`
	FinishedAt    string `json:"finished_at"`
	NotifiedLevel string `json:"-"`
	NotifiedOn    string `json:"-"`
	CalEventID    *int64 `json:"cal_event_id"`
	CalSynced     string `json:"-"`
	TaskID        *int64 `json:"task_id"`
	Source        string `json:"source"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`

	HomeName  string `json:"home_name"`
	ThingName string `json:"thing_name"`
	ProName   string `json:"pro_name"`
	Currency  string `json:"currency"`
	Status    string `json:"status"` // overdue | due | soon | ok | paused | done
	DaysLeft  int    `json:"days_left"`
}

// Recurring reports whether the job comes back after it is done.
func (j Job) Recurring() bool { return j.EveryN > 0 }

// Record is one entry in the log.
type Record struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"-"`
	HomeID      int64  `json:"home_id"`
	ThingID     *int64 `json:"thing_id"`
	JobID       *int64 `json:"job_id"`
	ProID       *int64 `json:"pro_id"`
	Date        string `json:"date"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Notes       string `json:"notes"`
	Cost        int64  `json:"cost"`
	Currency    string `json:"currency"`
	MoneyTxID   *int64 `json:"money_tx_id"`
	MoneyUserID *int64 `json:"money_user_id"`
	Source      string `json:"source"`
	ExternalID  string `json:"external_id"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`

	HomeName  string `json:"home_name"`
	ThingName string `json:"thing_name"`
	ProName   string `json:"pro_name"`
	JobTitle  string `json:"job_title"`
}

// Meter is an electricity, water or gas meter.
type Meter struct {
	ID           int64    `json:"id"`
	UserID       int64    `json:"-"`
	HomeID       int64    `json:"home_id"`
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	Unit         string   `json:"unit"`
	PricePerUnit *float64 `json:"price_per_unit"`
	Number       string   `json:"number"`
	Notes        string   `json:"notes"`
	RemindDay    int      `json:"remind_day"`
	RemindedOn   string   `json:"-"`
	Archived     bool     `json:"archived"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`

	HomeName string   `json:"home_name"`
	Currency string   `json:"currency"`
	Last     *Reading `json:"last"`
	Readings int      `json:"readings"`
}

// Reading is one meter reading. Usage is derived on read: the difference to
// the reading before (none after a reset).
type Reading struct {
	ID        int64    `json:"id"`
	UserID    int64    `json:"-"`
	MeterID   int64    `json:"meter_id"`
	ReadOn    string   `json:"read_on"`
	Value     float64  `json:"value"`
	Reset     bool     `json:"reset"`
	Note      string   `json:"note"`
	Source    string   `json:"source"`
	CreatedAt string   `json:"created_at"`
	Usage     *float64 `json:"usage"`
	Days      int      `json:"days"`
}

// Attachment is a paper or link kept with a home, a thing or a log entry.
type Attachment struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"-"`
	OwnerKind string `json:"owner_kind"`
	OwnerID   int64  `json:"owner_id"`
	Kind      string `json:"kind"` // document | url
	Ref       string `json:"ref"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	CreatedAt string `json:"created_at"`
}

// Settings are the box-wide values plus one person's preferences and their
// own connections. Connection modes: "core" (an app link through Pocket
// Core), "direct" (URL + token) or "off"; "" = not chosen yet.
type Settings struct {
	Title         string `json:"title"`
	SetupComplete bool   `json:"setup_complete"`

	DateFormat string `json:"date_format"`
	TimeFormat string `json:"time_format"`
	WeekStart  int    `json:"week_start"` // 1 = Monday, 7 = Sunday
	Currency   string `json:"currency"`   // for new homes
	AreaUnit   string `json:"area_unit"`  // m2 | ft2

	// Notifications.
	NotifyUpkeep   bool   `json:"notify_upkeep"`   // upkeep due soon, due, overdue
	NotifyWarranty bool   `json:"notify_warranty"` // a warranty ends within 30 days
	NotifyMeters   bool   `json:"notify_meters"`   // a meter's reading day
	NotifyHour     int    `json:"notify_hour"`
	NotifyURL      string `json:"notify_url"`
	NotifyFormat   string `json:"notify_format"` // apprise | ntfy | json

	// Pocket Money: costs in the log become transactions.
	MoneyMode       string `json:"money_mode"`
	MoneyURL        string `json:"money_url"`
	MoneyToken      string `json:"money_token"`
	MoneyPublicURL  string `json:"money_public_url"`
	MoneyBook       bool   `json:"money_book"` // tick "Create a transaction" by default
	MoneyAccountID  int64  `json:"money_account_id"`
	MoneyCategoryID int64  `json:"money_category_id"`

	// Pocket Calendar: upkeep due dates as all-day events.
	CalendarMode      string `json:"calendar_mode"`
	CalendarURL       string `json:"calendar_url"`
	CalendarToken     string `json:"calendar_token"`
	CalendarPublicURL string `json:"calendar_public_url"`
	CalendarSync      bool   `json:"calendar_sync"`
	CalendarID        int64  `json:"calendar_id"` // 0 = the person's default calendar

	// Pocket Tasks: "Add to my tasks" on a job.
	TasksMode      string `json:"tasks_mode"`
	TasksURL       string `json:"tasks_url"`
	TasksToken     string `json:"tasks_token"`
	TasksPublicURL string `json:"tasks_public_url"`

	// Pocket Contacts: pick a pro from your contacts.
	ContactsMode      string `json:"contacts_mode"`
	ContactsURL       string `json:"contacts_url"`
	ContactsToken     string `json:"contacts_token"`
	ContactsPublicURL string `json:"contacts_public_url"`

	// Pocket Documents: manuals, warranties and invoices.
	DocumentsMode      string `json:"documents_mode"`
	DocumentsURL       string `json:"documents_url"`
	DocumentsToken     string `json:"documents_token"`
	DocumentsPublicURL string `json:"documents_public_url"`

	// Pocket Inventory: a thing is also an item on your shelves.
	InventoryMode      string `json:"inventory_mode"`
	InventoryURL       string `json:"inventory_url"`
	InventoryToken     string `json:"inventory_token"`
	InventoryPublicURL string `json:"inventory_public_url"`

	// AI for upkeep suggestions and questions: Pocket AI through Core, or an
	// OpenAI-compatible API with the person's own key.
	AIProvider    string `json:"ai_provider"` // auto | pocket | openai
	AIPocketModel string `json:"ai_pocket_model"`
	OpenAIKey     string `json:"openai_api_key"`
	OpenAIModel   string `json:"openai_model"`
	OpenAIBaseURL string `json:"openai_base_url"`
}

// SplitTags turns "a, b ,c" into clean lowercase tags.
func SplitTags(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range strings.Split(s, ",") {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// JoinTags is the stored form of tags.
func JoinTags(s string) string { return strings.Join(SplitTags(s), ", ") }
