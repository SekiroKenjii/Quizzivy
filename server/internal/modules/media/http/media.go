package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"quizzivy/gen/openapi"
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/application/query"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/platform/httpapi"
	"quizzivy/internal/platform/httpx"
	"time"
)

// UploadMedia implements POST /teacher/media (§11.1).
func (h Media) UploadMedia(ctx context.Context, request openapi.UploadMediaRequestObject) (openapi.UploadMediaResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	part, err := nextFilePart(request.Body)
	if overBodyLimit(err) {
		return fileTooLarge(ctx), nil
	}
	if err != nil {
		return openapi.UploadMedia415JSONResponse(httpapi.Error(ctx, openapi.VALIDATIONFAILED,
			httpx.Text(ctx, "Không tìm thấy tệp trong yêu cầu tải lên.", "The upload request holds no file."))), nil
	}
	defer func() { _ = part.Close() }()

	meta := httpx.RequestMetaFromContext(ctx)
	asset, err := h.app.Commands.Upload.Handle(ctx, command.Upload{Filename: part.FileName(),
		Body:            part,
		UploaderID:      principal.UserID,
		DefaultMaxPlays: request.Params.DefaultMaxPlays,
		IP:              meta.IP,
		UserAgent:       meta.UserAgent,
	})
	switch {
	case err == nil:

	case errors.Is(err, domain.ErrImageTooLarge):
		return openapi.UploadMedia413JSONResponse(httpapi.Error(ctx, openapi.MEDIATOOLARGE,
			httpx.Text(ctx, "Ảnh vượt quá 10 MB. Vui lòng dùng ảnh nhỏ hơn.",
				"The image is larger than 10 MB. Please use a smaller one."))), nil

	case errors.Is(err, domain.ErrTooLarge), overBodyLimit(err):
		return fileTooLarge(ctx), nil

	case errors.Is(err, domain.ErrQuotaExceeded):
		return openapi.UploadMedia409JSONResponse(httpapi.Error(ctx, openapi.MEDIAQUOTAEXCEEDED,
			httpx.Text(ctx, "Thư viện đã hết dung lượng. Hãy xoá bớt tệp rồi tải lại.",
				"Your media library is full. Delete some files, then upload again."))), nil

	case errors.Is(err, domain.ErrPlayLimitOnImage), errors.Is(err, domain.ErrInvalidPlayLimit):
		return openapi.UploadMedia400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(playLimitRefused(ctx, err))}, nil

	case errors.Is(err, domain.ErrTooLong):
		return openapi.UploadMedia415JSONResponse(httpapi.Error(ctx, openapi.MEDIATOOLONG,
			httpx.Text(ctx, "Tệp âm thanh dài hơn 5 phút. Vui lòng cắt ngắn.",
				"The audio file is longer than 5 minutes. Please shorten it."))), nil
	case errors.Is(err, domain.ErrUnmeasurable):
		return openapi.UploadMedia415JSONResponse(httpapi.Error(ctx, openapi.MEDIAUNREADABLE,
			httpx.Text(ctx, "Không đọc được tệp âm thanh này. Tệp có thể bị lỗi hoặc chưa tải lên hết.",
				"This audio file cannot be read. It may be damaged or not fully uploaded."))), nil

	case errors.Is(err, domain.ErrUnsupportedType):
		return openapi.UploadMedia415JSONResponse(httpapi.Error(ctx, openapi.MEDIATYPEUNSUPPORTED,
			httpx.Text(ctx, "Chỉ hỗ trợ mp3, m4a và ảnh png/jpg/webp.",
				"Only mp3, m4a and png, jpg or webp images are supported."))), nil

	default:
		return nil, err
	}

	url, err := h.app.Queries.SignedURL.Handle(ctx, query.SignedURL{Asset: asset})
	if err != nil {
		return nil, err
	}
	return openapi.UploadMedia201JSONResponse(ToAPIMediaAsset(asset, url)), nil
}

func overBodyLimit(err error) bool {
	var tooLarge *http.MaxBytesError
	return errors.As(err, &tooLarge)
}

func fileTooLarge(ctx context.Context) openapi.UploadMedia413JSONResponse {
	return openapi.UploadMedia413JSONResponse(httpapi.Error(ctx, openapi.MEDIATOOLARGE,
		httpx.Text(ctx, "Tệp vượt quá 50 MB. Vui lòng nén hoặc cắt ngắn tệp.",
			"The file is larger than 50 MB. Please compress or shorten it.")))
}

const playLimitField = "defaultMaxPlays"

func playLimitRefused(ctx context.Context, err error) openapi.ErrorResponse {
	if errors.Is(err, domain.ErrPlayLimitOnImage) {
		return httpapi.FieldError(ctx, playLimitField,
			httpx.Text(ctx, "Chỉ tệp âm thanh mới có giới hạn số lần nghe.", "Only an audio file takes a play limit."))
	}
	return httpapi.FieldError(ctx, playLimitField,
		httpx.Text(ctx, "Giới hạn số lần nghe phải từ 0 đến 3.", "The play limit must be between 0 and 3."))
}

func nextFilePart(reader *multipart.Reader) (*multipart.Part, error) {
	for {
		part, err := reader.NextPart()
		if err != nil {
			return nil, err
		}
		if part.FileName() != "" {
			return part, nil
		}
		_ = part.Close()
	}
}

func ToAPIMediaAsset(a domain.Asset, url string) openapi.MediaAsset {
	out := openapi.MediaAsset{
		Id:               httpapi.ParseUUID(a.ID),
		Kind:             openapi.MediaKind(a.Kind),
		Url:              url,
		MimeType:         openapi.MediaAssetMimeType(a.MimeType),
		Bytes:            int(a.Bytes),
		OriginalFilename: a.OriginalFilename,
		CreatedAt:        a.CreatedAt,
	}
	if a.DurationMs != nil {
		out.DurationMs = a.DurationMs
	}
	return out
}

// ListMedia implements GET /teacher/media -- the §8 media library.
func (h Media) ListMedia(ctx context.Context, request openapi.ListMediaRequestObject) (openapi.ListMediaResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}

	in := listInput(ctx, request.Params)
	listResult, err := h.app.Queries.List.Handle(ctx, query.List{Input: in})
	assets, page := listResult.Items, listResult.Page
	if err != nil {
		return nil, err
	}
	totalBytes, err := h.app.Queries.TotalBytes.Handle(ctx, query.TotalBytes{Kind: in.Kind, Query: in.Query, Unused: in.Unused, Scope: in.Scope})
	if err != nil {
		return nil, err
	}
	facets, err := h.app.Queries.Facets.Handle(ctx, query.Facets{Input: in})
	if err != nil {
		return nil, err
	}
	usage, err := h.app.Queries.Usage.Handle(ctx, query.Usage{Scope: in.Scope})
	if err != nil {
		return nil, err
	}
	var out openapi.ListMedia200JSONResponse
	out.Body.TotalBytes = int(totalBytes)
	out.Body.Facets = openapi.MediaFacets{All: facets.All, Audio: facets.Audio, Image: facets.Image, Unused: facets.Unused}
	out.Body.Usage = openapi.MediaUsage{AudioBytes: usage.AudioBytes, ImageBytes: usage.ImageBytes, QuotaBytes: usage.QuotaBytes}
	out.Headers.CacheControl = cacheControlForSignedURLList
	out.Body.Items = make([]openapi.LibraryAsset, len(assets))
	for i, a := range assets {
		out.Body.Items[i] = ToAPILibraryAsset(a)
	}
	out.Body.Page, out.Body.PageSize, out.Body.Total = page.Number, page.Size, page.Total
	return out, nil
}

func listInput(ctx context.Context, params openapi.ListMediaParams) domain.ListInput {
	in := domain.ListInput{Scope: httpapi.ScopeFromContext(ctx).Own()}
	if params.Kind != nil {
		kind := domain.Kind(*params.Kind)
		in.Kind = &kind
	}
	if params.Q != nil {
		in.Query = *params.Q
	}
	if params.Unused != nil {
		in.Unused = *params.Unused
	}
	if params.Page != nil {
		in.Page = int(*params.Page)
	}
	if params.Limit != nil {
		in.Limit = int(*params.Limit)
	}
	return in
}

// ToAPILibraryAsset renders an asset as the library lists it, with the URL,
// the references and the question count its Asset already carries.
func ToAPILibraryAsset(a domain.Asset) openapi.LibraryAsset {
	usage := a.UsageCount
	usedIn := ToAPIReferencingTests(a.UsedIn)
	return openapi.LibraryAsset{
		Bytes:            int(a.Bytes),
		CreatedAt:        a.CreatedAt,
		DefaultMaxPlays:  a.DefaultMaxPlays,
		DisplayName:      a.DisplayName,
		DurationMs:       a.DurationMs,
		Height:           a.Height,
		Id:               httpapi.ParseUUID(a.ID),
		Kind:             openapi.LibraryAssetKind(a.Kind),
		MimeType:         openapi.LibraryAssetMimeType(a.MimeType),
		OriginalFilename: a.OriginalFilename,
		QuestionCount:    a.QuestionCount,
		Url:              a.URL,
		UsageCount:       &usage,
		UsedIn:           &usedIn,
		Width:            a.Width,
	}
}

// UpdateMedia implements PATCH /teacher/media/{id}.
func (h Media) UpdateMedia(ctx context.Context, request openapi.UpdateMediaRequestObject) (openapi.UpdateMediaResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	actor, ok := httpapi.ActorFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	in := domain.UpdateInput{
		ID:          request.Id.String(),
		ActorID:     actor.ID,
		All:         actor.Scope.All,
		DisplayName: request.Body.DisplayName,
		IP:          actor.IP,
		UserAgent:   actor.UserAgent,
	}
	if raw := request.Body.DefaultMaxPlays; len(raw) > 0 {
		in.SetDefaultMaxPlays = true
		if err := json.Unmarshal(raw, &in.DefaultMaxPlays); err != nil {
			return updateRefused(ctx, domain.ErrInvalidPlayLimit), nil
		}
	}

	asset, err := h.app.Commands.Update.Handle(ctx, command.Update{Input: in})
	switch {
	case err == nil:
		return openapi.UpdateMedia200JSONResponse(ToAPILibraryAsset(asset)), nil

	case errors.Is(err, domain.ErrNotFound):
		return openapi.UpdateMedia404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy tệp.", "The file was not found.")))}, nil

	case errors.Is(err, domain.ErrPlayLimitOnImage), errors.Is(err, domain.ErrInvalidPlayLimit),
		errors.Is(err, domain.ErrInvalidName), errors.Is(err, domain.ErrNothingToUpdate):
		return updateRefused(ctx, err), nil

	default:
		return nil, err
	}
}

func updateRefused(ctx context.Context, err error) openapi.UpdateMedia400JSONResponse {
	var body openapi.ErrorResponse
	switch {
	case errors.Is(err, domain.ErrInvalidName):
		body = httpapi.FieldError(ctx, "displayName",
			httpx.Text(ctx, "Tên tệp phải có từ 1 đến 200 ký tự.", "The file name must be 1 to 200 characters long."))
	case errors.Is(err, domain.ErrNothingToUpdate):
		body = httpapi.Error(ctx, openapi.VALIDATIONFAILED,
			httpx.Text(ctx, "Cần ít nhất một thay đổi.", "At least one change is needed."))
	default:
		body = playLimitRefused(ctx, err)
	}
	return openapi.UpdateMedia400JSONResponse{BadRequestJSONResponse: openapi.BadRequestJSONResponse(body)}
}

func ToAPIReferencingTests(refs []domain.TestRef) []openapi.ReferencingTest {
	out := make([]openapi.ReferencingTest, len(refs))
	for i, ref := range refs {
		version := ref.Version
		out[i] = openapi.ReferencingTest{Id: httpapi.ParseUUID(ref.ID), Title: ref.Title, Version: &version}
	}
	return out
}

// DeleteMedia implements DELETE /teacher/media/{id}.
func (h Media) DeleteMedia(ctx context.Context, request openapi.DeleteMediaRequestObject) (openapi.DeleteMediaResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	actor, ok := httpapi.ActorFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	_, err := h.app.Commands.Delete.Handle(ctx, command.Delete{Input: domain.DeleteInput{
		ID:        request.Id.String(),
		ActorID:   actor.ID,
		All:       actor.Scope.All,
		IP:        actor.IP,
		UserAgent: actor.UserAgent,
	}})
	switch {
	case err == nil:
		return openapi.DeleteMedia204Response{}, nil

	case errors.Is(err, domain.ErrReferenced):
		resp := httpapi.Error(ctx, openapi.MEDIAREFERENCED,
			httpx.Text(ctx, "Tệp đang được dùng trong một đề đã xuất bản nên không thể xoá.",
				"The file is used in a published test, so it cannot be deleted."))
		var blocked *domain.ReferencedError
		if errors.As(err, &blocked) {
			refs := ToAPIReferencingTests(blocked.Tests)
			details := map[string]interface{}{"tests": refs}
			if len(blocked.Groups) > 0 {
				resp.Error.Message = httpx.Text(ctx, "Tệp đang được dùng trong nhóm câu hỏi hoặc đề đã xuất bản nên không thể xoá.",
					"The file is used in a question group or a published test, so it cannot be deleted.")
				details["groups"] = groupReferences(blocked.Groups)
			}
			resp.Error.Details = &details
		}
		return openapi.DeleteMedia409JSONResponse(resp), nil

	case errors.Is(err, domain.ErrNotFound):
		return openapi.DeleteMedia404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(
			httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy tệp.", "The file was not found.")))}, nil

	default:
		return nil, err
	}
}

func groupReferences(refs []domain.GroupRef) []openapi.ReferencingGroup {
	out := make([]openapi.ReferencingGroup, len(refs))
	for i, ref := range refs {
		out[i] = openapi.ReferencingGroup{Id: httpapi.ParseUUID(ref.ID), Title: ref.Title}
		if ref.TestID != nil {
			id := httpapi.ParseUUID(*ref.TestID)
			out[i].TestId = &id
		}
	}
	return out
}

// GetMediaUrl implements GET /app/media/{assetId}/url -- a student minting a
// signed URL for a listening file (§11.2).
func (h Media) GetMediaUrl(ctx context.Context, request openapi.GetMediaUrlRequestObject) (openapi.GetMediaUrlResponseObject, error) {
	if h.app == nil {
		return nil, httpx.ErrNotImplemented
	}
	principal, ok := httpx.PrincipalFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}

	result, err := h.app.Queries.MintForStudent.Handle(ctx, query.MintForStudent{StudentID: principal.UserID, AssetID: request.AssetId.String()})
	if errors.Is(err, domain.ErrForbidden) {
		return openapi.GetMediaUrl403JSONResponse{ForbiddenJSONResponse: openapi.ForbiddenJSONResponse(
			httpapi.Error(ctx, openapi.FORBIDDEN,
				httpx.Text(ctx, "Bạn không có quyền truy cập tệp này.", "You do not have permission to access this file.")))}, nil
	}
	if err != nil {
		return nil, err
	}

	return openapi.GetMediaUrl200JSONResponse{
		Body: struct {
			ExpiresAt openapi.Timestamp `json:"expiresAt"`
			Url       string            `json:"url"`
		}{
			ExpiresAt: result.ExpiresAt,
			Url:       result.URL,
		},
		Headers: openapi.GetMediaUrl200ResponseHeaders{
			CacheControl: cacheControlForSignedURL(h.app.SignedURLTTL()),
		},
	}, nil
}

func cacheControlForSignedURL(ttl time.Duration) string {
	return fmt.Sprintf("private, max-age=%d", int(ttl.Seconds()))
}

const cacheControlForSignedURLList = "private, no-store"

// ReplaceMedia replaces a library file while preserving immutable published media.
func (h Media) ReplaceMedia(ctx context.Context, request openapi.ReplaceMediaRequestObject) (openapi.ReplaceMediaResponseObject, error) {
	if h.app == nil || request.Body == nil {
		return nil, httpx.ErrNotImplemented
	}
	actor, ok := httpapi.ActorFromContext(ctx)
	if !ok {
		return nil, httpx.ErrNotImplemented
	}
	scope := httpapi.ScopeFromContext(ctx)
	if _, err := h.app.Queries.ReplacementTarget.Handle(ctx, query.ReplacementTarget{ID: request.Id.String(), Scope: scope}); err != nil {
		return replacementRefused(ctx, err)
	}
	part, err := nextFilePart(request.Body)
	if err != nil {
		if overBodyLimit(err) {
			return openapi.ReplaceMedia413JSONResponse(fileTooLarge(ctx)), nil
		}
		return openapi.ReplaceMedia415JSONResponse(httpapi.Error(ctx, openapi.VALIDATIONFAILED, httpx.Text(ctx, "Không tìm thấy tệp trong yêu cầu tải lên.", "The upload request holds no file."))), nil
	}
	defer func() { _ = part.Close() }()
	result, err := h.app.Commands.Replace.Handle(ctx, command.Replace{ID: request.Id.String(), Scope: scope, Filename: part.FileName(), Body: part, UploaderID: actor.ID, IP: actor.IP, UserAgent: actor.UserAgent})
	if err != nil {
		return replacementRefused(ctx, err)
	}
	return openapi.ReplaceMedia201JSONResponse{Asset: ToAPILibraryAsset(result.Asset), Repointed: openapi.MediaReplacementCounts{Questions: result.Repointed.Questions, Groups: result.Repointed.Groups}, Left: openapi.MediaReplacementCounts{Questions: result.Left.Questions, Groups: result.Left.Groups}}, nil
}
func replacementRefused(ctx context.Context, err error) (openapi.ReplaceMediaResponseObject, error) {
	var cleanup *domain.ReplacementCleanupError
	var transaction *domain.ReplacementError
	if errors.As(err, &cleanup) || errors.As(err, &transaction) && (transaction.Outcome != domain.ReplacementNotCommitted || transaction.RollbackError != nil) {
		return nil, err
	}
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return openapi.ReplaceMedia404JSONResponse{NotFoundJSONResponse: openapi.NotFoundJSONResponse(httpapi.NotFound(ctx, httpx.Text(ctx, "Không tìm thấy tệp.", "The file was not found.")))}, nil
	case errors.Is(err, domain.ErrQuotaExceeded):
		return openapi.ReplaceMedia409JSONResponse(httpapi.Error(ctx, openapi.MEDIAQUOTAEXCEEDED, httpx.Text(ctx, "Thư viện đã hết dung lượng. Hãy xoá bớt tệp rồi tải lại.", "Your media library is full. Delete some files, then upload again."))), nil
	case errors.Is(err, domain.ErrImageTooLarge):
		return openapi.ReplaceMedia413JSONResponse(httpapi.Error(ctx, openapi.MEDIATOOLARGE, httpx.Text(ctx, "Ảnh vượt quá 10 MB. Vui lòng dùng ảnh nhỏ hơn.", "The image is larger than 10 MB. Please use a smaller one."))), nil
	case errors.Is(err, domain.ErrTooLarge), overBodyLimit(err):
		return openapi.ReplaceMedia413JSONResponse(fileTooLarge(ctx)), nil
	case errors.Is(err, domain.ErrKindMismatch):
		return openapi.ReplaceMedia415JSONResponse(httpapi.Error(ctx, openapi.MEDIAKINDMISMATCH, httpx.Text(ctx, "Tệp thay thế phải có cùng loại với tệp cũ.", "The replacement must have the same media kind as the old file."))), nil
	case errors.Is(err, domain.ErrTooLong):
		return openapi.ReplaceMedia415JSONResponse(httpapi.Error(ctx, openapi.MEDIATOOLONG, httpx.Text(ctx, "Tệp âm thanh dài hơn 5 phút. Vui lòng cắt ngắn.", "The audio file is longer than 5 minutes. Please shorten it."))), nil
	case errors.Is(err, domain.ErrUnmeasurable):
		return openapi.ReplaceMedia415JSONResponse(httpapi.Error(ctx, openapi.MEDIAUNREADABLE, httpx.Text(ctx, "Không đọc được tệp âm thanh này. Tệp có thể bị lỗi hoặc chưa tải lên hết.", "This audio file cannot be read. It may be damaged or not fully uploaded."))), nil
	case errors.Is(err, domain.ErrUnsupportedType):
		return openapi.ReplaceMedia415JSONResponse(httpapi.Error(ctx, openapi.MEDIATYPEUNSUPPORTED, httpx.Text(ctx, "Chỉ hỗ trợ mp3, m4a và ảnh png/jpg/webp.", "Only mp3, m4a and png, jpg or webp images are supported."))), nil
	default:
		return nil, err
	}
}
