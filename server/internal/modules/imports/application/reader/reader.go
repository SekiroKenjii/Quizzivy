// Package reader reads a scanned or photographed PDF with the AI assistant. It draws the pages, holds the credits for every page before the first one is sent, asks the provider for each page's printed text and settles what was billed. What comes back is untrusted text: the import pipeline recognises, validates and reviews it as it does the text layer of a PDF.
package reader

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	aicommand "quizzivy/internal/modules/ai/application/command"
	aiports "quizzivy/internal/modules/ai/application/ports"
	aidomain "quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/modules/ai/domain/prompts"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/cqrs"
)

const (
	// MaxPages is the most pages one file may have to be read with AI.
	MaxPages = 40

	concurrency   = 4
	budget        = 10 * time.Minute
	settleTimeout = 15 * time.Second
)

var (
	// ErrTooLong is a document of more than MaxPages pages.
	ErrTooLong = errors.New("reader: the document has more pages than AI reads")
	// ErrProtected is a document that needs a password.
	ErrProtected = errors.New("reader: the document is protected")
	// ErrUndrawable is a document whose pages cannot be drawn.
	ErrUndrawable = errors.New("reader: the pages cannot be drawn")
	// ErrPaused is a teacher whose AI use an Admin paused.
	ErrPaused = errors.New("reader: AI is paused for this teacher")
	// ErrUnavailable is a read the provider could not finish: it was down, overloaded, slow or rejected the request. What it billed was settled and the rest released.
	ErrUnavailable = errors.New("reader: the AI service could not finish reading")
)

// CreditsError is a read the credits cannot cover, before any page was sent. Scope says whether the teacher's allowance or the centre's pool fell short.
type CreditsError struct {
	Scope aidomain.Scope
}

func (e *CreditsError) Error() string {
	return fmt.Sprintf("reader: not enough AI credits (%s)", e.Scope)
}

// Page is one page drawn as an image.
type Page struct {
	Number    int
	MediaType string
	Image     []byte
}

// Pages draws the pages of a PDF for the provider. It answers ErrTooLong for a document of more than limit pages, ErrProtected and ErrUndrawable.
type Pages interface {
	Draw(ctx context.Context, pdf []byte, limit int) ([]Page, error)
}

// Line is one line of a page's printed text.
type Line struct {
	Page int
	Text string
}

// Result is a document read: its lines in page order, the pages that could not be read, and the credits the read cost.
type Result struct {
	Pages      int
	Lines      []Line
	Unreadable []int
	Credits    int64
}

// Reader reads documents with AI. A Reader without a Completer is not built: the assistant is off then, and a scan is refused as it was before it.
type Reader struct {
	Pages     Pages
	Reserve   cqrs.CommandHandler[aicommand.Reserve, aidomain.Reservation]
	Finish    cqrs.CommandHandler[aicommand.Finish, aidomain.Settled]
	Completer aiports.Completer
	Logger    *slog.Logger
}

// Read reads the pages of pdf for the teacher by, who pays. Every page is drawn first, then the credits for all of them are reserved, so a file is never half read for want of credits; each page is then read on the capable tier, a few at a time. A page the provider refuses, or answers with nothing usable, is listed in Unreadable and the rest are read. When the provider fails on a page, or the context ends, the read stops and fails: what the finished pages billed is charged and the remainder of the reservation released.
func (r Reader) Read(ctx context.Context, pdf []byte, by string) (Result, error) {
	pages, err := r.Pages.Draw(ctx, pdf, MaxPages)
	if err != nil {
		return Result{}, err
	}
	hold, err := r.Reserve.Handle(ctx, aicommand.Reserve{
		Actor:   actor.Actor{ID: by},
		Feature: aidomain.FeatureImport,
		Ceiling: aidomain.FeatureImport.Ceiling() * int64(len(pages)),
	})
	switch {
	case errors.Is(err, aidomain.ErrPaused):
		return Result{}, ErrPaused
	case errors.Is(err, aidomain.ErrExhausted):
		var exhausted *aidomain.Exhausted
		if errors.As(err, &exhausted) {
			return Result{}, &CreditsError{Scope: exhausted.Scope}
		}
		return Result{}, &CreditsError{Scope: aidomain.ScopeTeacher}
	case err != nil:
		return Result{}, err
	}
	spent := &meter{}
	settled := false
	defer func() {
		if !settled {
			r.settle(ctx, hold, spent)
		}
	}()
	out, err := r.readAll(ctx, pages, spent)
	settled = true
	out.Credits = r.settle(ctx, hold, spent)
	if err != nil {
		return Result{}, err
	}
	return out, nil
}

func (r Reader) readAll(parent context.Context, pages []Page, spent *meter) (Result, error) {
	ctx, cancel := context.WithTimeout(parent, budget)
	defer cancel()
	texts := make([]prompts.PageText, len(pages))
	var (
		wg      sync.WaitGroup
		failure sync.Once
		first   error
		slots   = make(chan struct{}, concurrency)
	)
	for i, page := range pages {
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer func() { <-slots; wg.Done() }()
			text, err := r.readPage(ctx, page, len(pages), spent)
			if err != nil {
				failure.Do(func() { first = err; cancel() })
				return
			}
			texts[i] = text
		}()
	}
	wg.Wait()
	if first != nil {
		if parent.Err() == nil && errors.Is(first, context.DeadlineExceeded) {
			return Result{}, fmt.Errorf("%w: %w", ErrUnavailable, first)
		}
		return Result{}, first
	}
	out := Result{Pages: len(pages)}
	for i, text := range texts {
		if !text.Readable {
			out.Unreadable = append(out.Unreadable, pages[i].Number)
			continue
		}
		for _, line := range text.Lines {
			out.Lines = append(out.Lines, Line{Page: pages[i].Number, Text: line})
		}
	}
	return out, nil
}

func (r Reader) readPage(ctx context.Context, page Page, total int, spent *meter) (prompts.PageText, error) {
	if err := ctx.Err(); err != nil {
		return prompts.PageText{}, err
	}
	prompt := prompts.ReadPage(prompts.ReadPageInput{Page: page.Number, Pages: total})
	response, err := r.Completer.Complete(ctx, aiports.Request{
		Tier:      aiports.TierCapable,
		System:    prompt.System,
		Blocks:    []aiports.Block{aiports.ImageBlock(page.MediaType, page.Image), aiports.TextBlock(prompt.User)},
		Schema:    prompt.Schema,
		MaxTokens: prompt.MaxTokens,
	})
	spent.add(response)
	switch {
	case err == nil:
		text, decodeErr := prompts.DecodePage(response.Object)
		if decodeErr != nil {
			return prompts.PageText{}, nil
		}
		return text, nil
	case errors.Is(err, aiports.ErrRefused), errors.Is(err, aiports.ErrInvalidResponse):
		return prompts.PageText{}, nil
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return prompts.PageText{}, err
	case errors.Is(err, aiports.ErrRejected):
		r.logger().ErrorContext(ctx, "ai provider rejected an import page", "page", page.Number)
	}
	return prompts.PageText{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
}

func (r Reader) settle(ctx context.Context, hold aidomain.Reservation, spent *meter) int64 {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), settleTimeout)
	defer cancel()
	settled, err := r.Finish.Handle(ctx, aicommand.Finish{Reservation: hold, Tier: aiports.TierCapable, Response: spent.response()})
	if err != nil {
		r.logger().ErrorContext(ctx, "ai import reservation was not settled", "reservation", hold.ID, "err", err)
		return 0
	}
	r.logger().InfoContext(ctx, "ai import settled", "reservation", hold.ID, "held", hold.Credits, "charged", settled.Charged, "metered", settled.Metered)
	return settled.Charged
}

func (r Reader) logger() *slog.Logger {
	if r.Logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return r.Logger
}

type meter struct {
	mu        sync.Mutex
	usage     aiports.Usage
	requestID string
}

func (m *meter) add(response aiports.Response) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.usage.InputTokens += response.Usage.InputTokens
	m.usage.OutputTokens += response.Usage.OutputTokens
	m.usage.CacheReadTokens += response.Usage.CacheReadTokens
	m.usage.CacheWriteTokens += response.Usage.CacheWriteTokens
	if m.requestID == "" {
		m.requestID = response.RequestID
	}
}

func (m *meter) response() aiports.Response {
	m.mu.Lock()
	defer m.mu.Unlock()
	return aiports.Response{Usage: m.usage, RequestID: m.requestID}
}
