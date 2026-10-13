// Package domain is the AI assistant's credit model. A credit is $0.001 of the provider's list price. The centre has a pool of credits each month (D21), every teacher an allowance under it, and a call reserves its ceiling before it reaches the provider and settles what the provider metered after. The database holds the pool and each allowance as constraints, so no code path can spend past either.
package domain

import (
	"fmt"
	"time"

	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
)

// The bounds the commands hold a person's input to, in credits.
const (
	// MinAllowance is the least a teacher's monthly allowance may be.
	MinAllowance int64 = 1_000
	// MaxAllowance is the most a teacher's monthly allowance may be before requests are added.
	MaxAllowance int64 = 10_000_000
	// MinRequest is the least credits one request asks for.
	MinRequest int64 = 1_000
	// MaxRequest is the most credits one request asks for.
	MaxRequest int64 = 100_000
	// MaxAddition is the most credits one addition may put in the pool.
	MaxAddition int64 = 10_000_000
	// MaxCeiling is the most credits one reservation may hold.
	MaxCeiling int64 = 100_000
	// MinWarnPercent is the lowest share of an allowance at which a teacher may be warned.
	MinWarnPercent = 50
	// MaxWarnPercent is the highest share of an allowance at which a teacher may be warned.
	MaxWarnPercent = 99
	// MaxReasonLength is the longest reason a request may give, in characters.
	MaxReasonLength = 500
	// StaleAfter is how long a reservation may be held before the sweep returns it.
	StaleAfter = 10 * time.Minute
)

// DefaultZone is the calendar zone a month is counted in when APP_TIMEZONE names none.
const DefaultZone = "Asia/Ho_Chi_Minh"

const microUSDPerCredit = 1_000

const microUSDPerMillionTokens = 1_000_000

const maxTokens = 1 << 31

// Tier names a class of model, as the ledger records it.
type Tier string

// The two tiers.
const (
	TierFast    Tier = "fast"
	TierCapable Tier = "capable"
)

// Tokens is what the provider reported for one call. Input counts only tokens that were neither read from nor written to the prompt cache, so the four never overlap.
type Tokens struct {
	Input      int64
	Output     int64
	CacheRead  int64
	CacheWrite int64
}

// Zero reports whether nothing was metered.
func (t Tokens) Zero() bool { return t == Tokens{} }

// Price is a tier's list price in millionths of a US dollar per million tokens. Cache writes are not priced separately by the deployment, so they are charged at the input price, which never undercounts the pool by more than the provider's write premium.
type Price struct {
	Input     int64
	Output    int64
	CacheRead int64
}

// Prices holds the price of each tier.
type Prices struct {
	Fast    Price
	Capable Price
}

// For returns the price of tier, or an error for a tier it does not know.
func (p Prices) For(tier Tier) (Price, error) {
	switch tier {
	case TierFast:
		return p.Fast, nil
	case TierCapable:
		return p.Capable, nil
	}
	return Price{}, fmt.Errorf("ai: no price for tier %q", tier)
}

// Credits is what tokens cost at p, in whole credits, rounded up so a call never costs less than it did. It is integer arithmetic throughout, so no call is rounded across a credit boundary by floating point. It refuses a negative count and one too large to multiply safely.
func (p Price) Credits(t Tokens) (int64, error) {
	for _, n := range []int64{t.Input, t.Output, t.CacheRead, t.CacheWrite} {
		if n < 0 || n > maxTokens {
			return 0, fmt.Errorf("ai: a token count of %d is outside what the provider reports", n)
		}
	}
	micro := t.Input*p.Input + t.Output*p.Output + t.CacheRead*p.CacheRead + t.CacheWrite*p.Input
	unit := int64(microUSDPerCredit) * microUSDPerMillionTokens
	return (micro + unit - 1) / unit, nil
}

// Limit is what happens to a teacher who has used the allowance.
type Limit string

// The two rules a centre may choose.
const (
	// LimitPause stops the teacher until next month.
	LimitPause Limit = "pause"
	// LimitRequest lets the teacher ask an Admin for more.
	LimitRequest Limit = "request"
)

// Valid reports whether l is one of the two rules.
func (l Limit) Valid() bool { return l == LimitPause || l == LimitRequest }

// Rules are the centre's settings. PoolCredits is the plan for a month that has not begun; the default allowance, the warning share and the limit rule apply at once.
type Rules struct {
	PoolCredits      int64
	DefaultAllowance int64
	WarnPercent      int
	OnTeacherLimit   Limit
}

// Author is who a credits command acts for: a signed-in user, or System when UserID is empty, in which case Operator names the person who ran cmd/maintenance. Grants are the signed-in user's effective permissions, which a command that acts on one teacher compares with that teacher's under the subset rule; System holds none and is not asked.
type Author struct {
	UserID    string
	IP        string
	UserAgent string
	Operator  string
	Grants    access.Set
}

// AuthorOf is the Author for a signed-in user holding grants.
func AuthorOf(a actor.Actor, grants access.Set) Author {
	return Author{UserID: a.ID, IP: a.IP, UserAgent: a.UserAgent, Grants: grants}
}

// System reports whether the command was run from the console and not by a signed-in user.
func (a Author) System() bool { return a.UserID == "" }

// MonthOf is the first day of the month t falls in, counted in zone, as a date at UTC midnight. It is the key of a month's rows.
func MonthOf(t time.Time, zone *time.Location) time.Time {
	local := t.In(zone)
	return time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// DaysIn is how many days the month of t has.
func DaysIn(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// DaysToGo is how many days of the month containing t, counted in zone, follow t's own.
func DaysToGo(t time.Time, zone *time.Location) int {
	local := t.In(zone)
	return DaysIn(local) - local.Day()
}

// OnPace is the credits the month will have used if the rest of it goes as the days so far did: used divided by the days elapsed, today included, times the days in the month, rounded up.
func OnPace(used int64, t time.Time, zone *time.Location) int64 {
	local := t.In(zone)
	elapsed, days := int64(local.Day()), int64(DaysIn(local))
	return (used*days + elapsed - 1) / elapsed
}

// Pool is the centre's month.
type Pool struct {
	Plan     int64
	Added    int64
	Used     int64
	Reserved int64
}

// Total is what the month may spend: the plan and what an Admin added.
func (p Pool) Total() int64 { return p.Plan + p.Added }

// Left is what the month may still spend, counting calls in flight as spent.
func (p Pool) Left() int64 { return max(p.Total()-p.Used-p.Reserved, 0) }

// TeacherStatus is where a teacher stands against the allowance.
type TeacherStatus string

// The four standings the Admin page draws.
const (
	StatusOnTrack   TeacherStatus = "on_track"
	StatusNearLimit TeacherStatus = "near_limit"
	StatusAtLimit   TeacherStatus = "at_limit"
	StatusPaused    TeacherStatus = "paused"
)

// StatusOf is the standing of a teacher who has used used of allowance, warned at warnPercent: paused beats at the limit, which beats near it.
func StatusOf(used, allowance int64, warnPercent int, paused bool) TeacherStatus {
	switch {
	case paused:
		return StatusPaused
	case used >= allowance:
		return StatusAtLimit
	case used*100 >= allowance*int64(warnPercent):
		return StatusNearLimit
	}
	return StatusOnTrack
}

// TeacherMonth is one teacher's month. Allowance includes Granted, the credits approved requests added; Custom says the allowance was set for this teacher and a change of the default leaves it.
type TeacherMonth struct {
	UserID    string
	Name      string
	Email     string
	Allowance int64
	Granted   int64
	Used      int64
	Reserved  int64
	Custom    bool
	Paused    bool
	Status    TeacherStatus
}

// Reservation is the credits held for one call in flight.
type Reservation struct {
	ID      string
	Month   time.Time
	UserID  string
	Feature Feature
	Credits int64
}

// ReserveInput asks to hold Credits for UserID in Month.
type ReserveInput struct {
	Month   time.Time
	UserID  string
	Feature Feature
	Credits int64
}

// Settlement is a finished call: the reservation it held, what the provider metered and the credits that costs. Metered is what Tokens cost at the tier's price.
type Settlement struct {
	Reservation Reservation
	Tier        Tier
	Tokens      Tokens
	Metered     int64
	RequestID   string
}

// Warning says a teacher crossed the share of the allowance the centre warns at, for the first time that month.
type Warning struct {
	UserID    string
	Month     time.Time
	Used      int64
	Allowance int64
}

// Settled is the outcome of a settlement. Charged is what moved from reserved to used, which never exceeds the reservation, so settling cannot fail on the pool's constraint after the provider has been paid; Metered is what the call cost, and the two differ only when it cost more than its ceiling. Warning is set once a month.
type Settled struct {
	Charged int64
	Metered int64
	Warning *Warning
}

// Released says what the sweep returned.
type Released struct {
	Reservations int64
	Credits      int64
}
