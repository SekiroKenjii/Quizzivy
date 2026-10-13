package maintenance

import (
	"time"

	"quizzivy/internal/modules/ai/domain"
)

type aiPoolReport struct {
	Plan     int64 `json:"plan"`
	Added    int64 `json:"added"`
	Total    int64 `json:"total"`
	Used     int64 `json:"used"`
	Reserved int64 `json:"reserved"`
	Left     int64 `json:"left"`
}

type aiTeacherReport struct {
	UserID    string `json:"userId"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Allowance int64  `json:"allowance"`
	Granted   int64  `json:"granted"`
	Used      int64  `json:"used"`
	Reserved  int64  `json:"reserved"`
	Custom    bool   `json:"custom"`
	Paused    bool   `json:"paused"`
	Status    string `json:"status"`
}

type aiRequestReport struct {
	ID          string `json:"id"`
	UserID      string `json:"userId"`
	TeacherName string `json:"teacherName,omitempty"`
	Month       string `json:"month"`
	Amount      int64  `json:"amount"`
	Reason      string `json:"reason,omitempty"`
	Status      string `json:"status"`
	Used        int64  `json:"used,omitempty"`
	Allowance   int64  `json:"allowance,omitempty"`
}

type aiRulesReport struct {
	PoolCredits      int64  `json:"poolCredits"`
	DefaultAllowance int64  `json:"defaultAllowance"`
	WarnPercent      int    `json:"warnPercent"`
	OnTeacherLimit   string `json:"onTeacherLimit"`
}

type aiStatusReport struct {
	Month      string            `json:"month"`
	Pool       aiPoolReport      `json:"pool"`
	DaysToGo   int               `json:"daysToGo"`
	OnPace     int64             `json:"onPace"`
	WithinPool bool              `json:"withinPool"`
	Allowances int64             `json:"allowances"`
	Active     int               `json:"teachersActive"`
	Idle       int               `json:"teachersIdle"`
	Teachers   []aiTeacherReport `json:"teachers"`
	Requests   []aiRequestReport `json:"requests"`
	Rules      aiRulesReport     `json:"rules"`
}

type aiRulesChangeReport struct {
	Rules     aiRulesReport `json:"rules"`
	Rewritten int64         `json:"allowancesRewritten"`
}

type aiDecisionReport struct {
	Request   aiRequestReport `json:"request"`
	Allowance int64           `json:"allowance"`
}

func monthOf(t time.Time) string { return t.Format(time.DateOnly) }

func poolOf(p domain.Pool) aiPoolReport {
	return aiPoolReport{Plan: p.Plan, Added: p.Added, Total: p.Total(), Used: p.Used, Reserved: p.Reserved, Left: p.Left()}
}

func teacherOf(t domain.TeacherMonth) aiTeacherReport {
	return aiTeacherReport{
		UserID: t.UserID, Name: t.Name, Email: t.Email, Allowance: t.Allowance, Granted: t.Granted,
		Used: t.Used, Reserved: t.Reserved, Custom: t.Custom, Paused: t.Paused, Status: string(t.Status),
	}
}

func rulesOf(r domain.Rules) aiRulesReport {
	return aiRulesReport{
		PoolCredits: r.PoolCredits, DefaultAllowance: r.DefaultAllowance, WarnPercent: r.WarnPercent, OnTeacherLimit: string(r.OnTeacherLimit),
	}
}

func requestOf(r domain.Request) aiRequestReport {
	return aiRequestReport{
		ID: r.ID, UserID: r.UserID, Month: monthOf(r.Month), Amount: r.Amount, Reason: r.Reason, Status: string(r.Status),
	}
}

func rulesChangeOf(c domain.RulesChange) aiRulesChangeReport {
	return aiRulesChangeReport{Rules: rulesOf(c.Rules), Rewritten: c.Rewritten}
}

func decidedOf(d domain.Decided) aiDecisionReport {
	report := requestOf(d.Request)
	report.TeacherName = d.TeacherName
	return aiDecisionReport{Request: report, Allowance: d.Allowance}
}

func statusOf(v domain.AdminCredits) aiStatusReport {
	out := aiStatusReport{
		Month: monthOf(v.Month), Pool: poolOf(v.Pool), DaysToGo: v.DaysToGo, OnPace: v.OnPace, WithinPool: v.WithinPool,
		Allowances: v.Allowances, Active: v.Active, Idle: v.Idle, Rules: rulesOf(v.Rules),
		Teachers: make([]aiTeacherReport, len(v.Teachers)), Requests: make([]aiRequestReport, len(v.Requests)),
	}
	for i, t := range v.Teachers {
		out.Teachers[i] = teacherOf(t)
	}
	for i, r := range v.Requests {
		out.Requests[i] = requestOf(r.Request)
		out.Requests[i].TeacherName, out.Requests[i].Used, out.Requests[i].Allowance = r.TeacherName, r.Used, r.Allowance
	}
	return out
}
