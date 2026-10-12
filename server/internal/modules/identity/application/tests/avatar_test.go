package application_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	"strings"
	"sync"
	"testing"
	"time"
)

type photoUsers struct {
	domain.Users
	written  []domain.AvatarRecord
	previous *string
	err      error
}

func (u *photoUsers) SetAvatar(_ context.Context, in domain.AvatarRecord) (domain.AvatarWrite, error) {
	u.written = append(u.written, in)
	if u.err != nil {
		return domain.AvatarWrite{}, u.err
	}
	user := domain.User{ID: in.UserID, AvatarKey: in.Key}
	return domain.AvatarWrite{User: user, PreviousKey: u.previous}, nil
}

type photoStore struct {
	mu        sync.Mutex
	puts      []string
	putType   string
	putBytes  []byte
	deletes   []string
	signed    []string
	signedTTL time.Duration
	putErr    error
	deleteErr error
}

func (s *photoStore) Put(_ context.Context, key, contentType string, body io.Reader, size int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, _ := io.ReadAll(body)
	if int64(len(data)) != size {
		return errors.New("size does not match the body")
	}
	s.puts, s.putType, s.putBytes = append(s.puts, key), contentType, data
	return s.putErr
}

func (s *photoStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	s.deletes = append(s.deletes, key)
	return s.deleteErr
}

func (s *photoStore) SignedURL(_ context.Context, key string, ttl time.Duration) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signed, s.signedTTL = append(s.signed, key), ttl
	return "https://objects.example/" + key, nil
}

type photoMaker struct {
	read         string
	result       []byte
	err          error
	cancelOnRead func()
}

func (p *photoMaker) Square(_ context.Context, body io.Reader) ([]byte, error) {
	data, _ := io.ReadAll(body)
	p.read = string(data)
	if p.cancelOnRead != nil {
		p.cancelOnRead()
	}
	return p.result, p.err
}

func photoApp(users *photoUsers, store *photoStore, maker *photoMaker) *application.Application {
	app := application.New(users, nil, time.Hour, nil, nil)
	if store != nil && maker != nil {
		app.SetAvatars(store, maker, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}
	return app
}

func TestSettingAPhotoStoresTheSquareUnderAFreshKeyAndDeletesTheOneItReplaced(t *testing.T) {
	old := "avatars/u1/old.png"
	users, store, maker := &photoUsers{previous: &old}, &photoStore{}, &photoMaker{result: []byte("square-png")}

	user, err := photoApp(users, store, maker).Commands.SetAvatar.Handle(context.Background(), command.SetAvatar{UserID: "u1", Body: strings.NewReader("upload"), IP: "203.0.113.9", UserAgent: "agent"})
	if err != nil {
		t.Fatal(err)
	}

	if maker.read != "upload" {
		t.Errorf("the processor read %q, want the upload", maker.read)
	}
	if len(store.puts) != 1 || !strings.HasPrefix(store.puts[0], "avatars/u1/") || !strings.HasSuffix(store.puts[0], ".png") {
		t.Fatalf("stored under %v, want one avatars/u1/*.png", store.puts)
	}
	if store.putType != "image/png" || string(store.putBytes) != "square-png" {
		t.Errorf("stored %q as %s, want the processor's bytes as image/png", store.putBytes, store.putType)
	}
	if len(users.written) != 1 || users.written[0].Key == nil || *users.written[0].Key != store.puts[0] {
		t.Fatalf("the user was pointed at %+v, want the stored key %s", users.written, store.puts[0])
	}
	if users.written[0].IP == nil || *users.written[0].IP != "203.0.113.9" || users.written[0].UserAgent == nil {
		t.Errorf("the audit metadata was %+v", users.written[0])
	}
	if user.AvatarKey == nil || *user.AvatarKey != store.puts[0] {
		t.Errorf("answered key %v", user.AvatarKey)
	}
	if len(store.deletes) != 1 || store.deletes[0] != old {
		t.Errorf("deleted %v, want only %s", store.deletes, old)
	}
}

func TestTwoPhotosOfOneUserNeverShareAKey(t *testing.T) {
	users, store, maker := &photoUsers{}, &photoStore{}, &photoMaker{result: []byte("x")}
	app := photoApp(users, store, maker)
	for range 2 {
		if _, err := app.Commands.SetAvatar.Handle(context.Background(), command.SetAvatar{UserID: "u1", Body: strings.NewReader("a")}); err != nil {
			t.Fatal(err)
		}
	}
	if len(store.puts) != 2 || store.puts[0] == store.puts[1] {
		t.Errorf("keys %v", store.puts)
	}
}

func TestARefusedImageStoresAndWritesNothing(t *testing.T) {
	for _, refusal := range []error{domain.ErrAvatarTooLarge, domain.ErrAvatarUnsupported, domain.ErrAvatarUnreadable, domain.ErrAvatarDimensions} {
		users, store, maker := &photoUsers{}, &photoStore{}, &photoMaker{err: refusal}

		_, err := photoApp(users, store, maker).Commands.SetAvatar.Handle(context.Background(), command.SetAvatar{UserID: "u1", Body: strings.NewReader("x")})

		if !errors.Is(err, refusal) {
			t.Errorf("answered %v, want %v", err, refusal)
		}
		if len(store.puts) != 0 || len(store.deletes) != 0 || len(users.written) != 0 {
			t.Errorf("%v: puts %v deletes %v writes %v, want none", refusal, store.puts, store.deletes, users.written)
		}
	}
}

func TestAFailedWriteDeletesTheObjectItJustStored(t *testing.T) {
	boom := errors.New("database down")
	users, store, maker := &photoUsers{err: boom}, &photoStore{}, &photoMaker{result: []byte("x")}

	_, err := photoApp(users, store, maker).Commands.SetAvatar.Handle(context.Background(), command.SetAvatar{UserID: "u1", Body: strings.NewReader("x")})

	if !errors.Is(err, boom) {
		t.Fatalf("answered %v, want the write's error", err)
	}
	if len(store.puts) != 1 || len(store.deletes) != 1 || store.deletes[0] != store.puts[0] {
		t.Errorf("puts %v deletes %v, want the new object deleted", store.puts, store.deletes)
	}
}

func TestAFailedStoreWritesNothingToTheUser(t *testing.T) {
	boom := errors.New("bucket down")
	users, store, maker := &photoUsers{}, &photoStore{putErr: boom}, &photoMaker{result: []byte("x")}

	_, err := photoApp(users, store, maker).Commands.SetAvatar.Handle(context.Background(), command.SetAvatar{UserID: "u1", Body: strings.NewReader("x")})

	if !errors.Is(err, boom) || len(users.written) != 0 {
		t.Errorf("answered %v with %d writes, want the store's error and none", err, len(users.written))
	}
}

func TestAFailedDeleteOfTheOldPhotoIsNotAnError(t *testing.T) {
	old := "avatars/u1/old.png"
	users, store, maker := &photoUsers{previous: &old}, &photoStore{deleteErr: errors.New("gone")}, &photoMaker{result: []byte("x")}

	if _, err := photoApp(users, store, maker).Commands.SetAvatar.Handle(context.Background(), command.SetAvatar{UserID: "u1", Body: strings.NewReader("x")}); err != nil {
		t.Fatalf("answered %v, want success", err)
	}
	if len(store.deletes) != 1 {
		t.Errorf("deletes %v", store.deletes)
	}
}

func TestSettingAPhotoWithNoObjectStoreAnswersUnavailableWithoutReadingTheUpload(t *testing.T) {
	body := &countingReader{Reader: strings.NewReader("x")}

	_, err := photoApp(&photoUsers{}, nil, nil).Commands.SetAvatar.Handle(context.Background(), command.SetAvatar{UserID: "u1", Body: body})

	if !errors.Is(err, domain.ErrAvatarsUnavailable) || body.reads != 0 {
		t.Errorf("answered %v after %d reads", err, body.reads)
	}
}

type countingReader struct {
	io.Reader
	reads int
}

func (c *countingReader) Read(p []byte) (int, error) {
	c.reads++
	return c.Reader.Read(p)
}

func TestRemovingAPhotoDeletesItsObjectAndAUserWithoutOneDeletesNothing(t *testing.T) {
	kept := "avatars/u1/kept.png"
	cases := []struct {
		name     string
		previous *string
		deletes  []string
	}{
		{"a photo", &kept, []string{kept}},
		{"no photo", nil, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			users, store := &photoUsers{previous: c.previous}, &photoStore{}

			user, err := photoApp(users, store, &photoMaker{}).Commands.RemoveAvatar.Handle(context.Background(), command.RemoveAvatar{UserID: "u1"})
			if err != nil {
				t.Fatal(err)
			}
			if len(users.written) != 1 || users.written[0].Key != nil || user.AvatarKey != nil {
				t.Errorf("wrote %+v and answered key %v, want a cleared key", users.written, user.AvatarKey)
			}
			if strings.Join(store.deletes, ",") != strings.Join(c.deletes, ",") {
				t.Errorf("deleted %v, want %v", store.deletes, c.deletes)
			}
		})
	}
}

func TestRemovingAPhotoNeedsNoObjectStore(t *testing.T) {
	kept := "avatars/u1/kept.png"
	users := &photoUsers{previous: &kept}

	user, err := photoApp(users, nil, nil).Commands.RemoveAvatar.Handle(context.Background(), command.RemoveAvatar{UserID: "u1"})

	if err != nil || user.AvatarKey != nil || len(users.written) != 1 {
		t.Errorf("answered %+v, %v after %d writes", user, err, len(users.written))
	}
}

func TestAPhotoIsSignedForTwentyFourHoursAndLeftOutWhenThereIsNoneToSign(t *testing.T) {
	key := "avatars/u1/now.png"
	store := &photoStore{}
	app := photoApp(&photoUsers{}, store, &photoMaker{})

	url, err := app.Queries.AvatarURL.Handle(context.Background(), query.AvatarURL{Key: &key})
	if err != nil || url != "https://objects.example/"+key || store.signedTTL != 24*time.Hour {
		t.Errorf("signed %q for %v, %v", url, store.signedTTL, err)
	}
	if url, err := app.Queries.AvatarURL.Handle(context.Background(), query.AvatarURL{}); url != "" || err != nil || len(store.signed) != 1 {
		t.Errorf("no key signed %q, %v (%d signings)", url, err, len(store.signed))
	}
	if url, err := photoApp(&photoUsers{}, nil, nil).Queries.AvatarURL.Handle(context.Background(), query.AvatarURL{Key: &key}); url != "" || err != nil {
		t.Errorf("no store signed %q, %v", url, err)
	}
}

func TestACancelledRequestStillDeletesTheReplacedObject(t *testing.T) {
	old := "avatars/u1/old.png"
	users, store, maker := &photoUsers{previous: &old}, &photoStore{}, &photoMaker{result: []byte("x")}
	ctx, cancel := context.WithCancel(context.Background())
	maker.cancelOnRead = cancel

	if _, err := photoApp(users, store, maker).Commands.SetAvatar.Handle(ctx, command.SetAvatar{UserID: "u1", Body: bytes.NewReader([]byte("x"))}); err != nil {
		t.Fatal(err)
	}
	if len(store.deletes) != 1 {
		t.Errorf("deletes %v, want the old photo deleted although the request ended", store.deletes)
	}
}
