package errs

import (
	"errors"
	"log/slog"
	"net/http"

	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/xanderbilla/bi8s-go/internal/response"
)

const (
	CodeValidationFailed = "VALIDATION_FAILED"
	CodeBadRequest       = "BAD_REQUEST"
	CodeUnauthorized     = "UNAUTHORIZED"
	CodeForbidden        = "FORBIDDEN"
	CodeNotFound         = "NOT_FOUND"
	CodeConflict         = "CONFLICT"
	CodeRateLimited      = "RATE_LIMITED"
	CodeInternal         = "INTERNAL_ERROR"
)

var (
	ErrFileUploaderNotConfigured = errors.New("file uploader is not configured")
	ErrAWSRegionRequired         = errors.New("aws region is required")
	ErrS3BucketNotConfigured     = errors.New("s3 bucket is not configured")
	ErrFileEmpty                 = errors.New("file is empty")

	ErrResultTooLarge = errors.New("result set too large; add filters or pagination")

	ErrContentNotFound      = errors.New("content not found")
	ErrNoEncodingFound      = errors.New("no encoding found for this content")
	ErrNoCompletedEncoding  = errors.New("no completed encoding for this content")
	ErrPlaybackNotAvailable = errors.New("playback information not available")

	ErrAttributeNameTaken = errors.New("attribute with this name already exists")
)

func IsConditionalCheckFailed(err error) bool {
	var e *ddbtypes.ConditionalCheckFailedException
	return errors.As(err, &e)
}

func IsThrottled(err error) bool {
	var pte *ddbtypes.ProvisionedThroughputExceededException
	if errors.As(err, &pte) {
		return true
	}
	var rle *ddbtypes.RequestLimitExceeded
	return errors.As(err, &rle)
}

type PerformerNotFoundError struct {
	ID string
}

func (e *PerformerNotFoundError) Error() string {
	return "performer with id '" + e.ID + "' not found"
}

type AttributeNotFoundError struct {
	ID           string
	ExpectedType string
}

func (e *AttributeNotFoundError) Error() string {
	return "attribute with id '" + e.ID + "' not found or wrong type (expected " + e.ExpectedType + ")"
}

type APIError struct {
	Status  int
	Code    string
	Message string
	Details any
	cause   error
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *APIError) Unwrap() error { return e.cause }

func newAPIError(status int, code, msg string, details any, cause error) *APIError {
	return &APIError{Status: status, Code: code, Message: msg, Details: details, cause: cause}
}

func NewValidation(details any) *APIError {
	return newAPIError(http.StatusBadRequest, CodeValidationFailed, "Request validation failed", details, nil)
}

func NewBadRequest(msg string) *APIError {
	if msg == "" {
		msg = "The request was invalid"
	}
	return newAPIError(http.StatusBadRequest, CodeBadRequest, msg, nil, nil)
}

func NewUnauthorized() *APIError {
	return newAPIError(http.StatusUnauthorized, CodeUnauthorized, "Authentication required", nil, nil)
}

func NewForbidden() *APIError {
	return newAPIError(http.StatusForbidden, CodeForbidden, "Access denied", nil, nil)
}

func NewNotFound(resource string) *APIError {
	if resource == "" {
		resource = "resource"
	}
	return newAPIError(http.StatusNotFound, CodeNotFound, "The requested "+resource+" was not found", nil, nil)
}

func NewConflict(msg string) *APIError {
	if msg == "" {
		msg = "The resource already exists or has changed"
	}
	return newAPIError(http.StatusConflict, CodeConflict, msg, nil, nil)
}

func NewRateLimited() *APIError {
	return newAPIError(http.StatusTooManyRequests, CodeRateLimited, "Rate limit exceeded. Please try again later.", nil, nil)
}

func NewInternal(cause error) *APIError {
	return newAPIError(http.StatusInternalServerError, CodeInternal, "The server encountered a problem", nil, cause)
}

// From maps any known typed/sentinel error to a properly classified *APIError.
// If err is already an *APIError, it is returned unchanged. Unknown errors map
// to a generic 500 with cause attached (no leakage of internal details).
func From(err error) *APIError {
	if err == nil {
		return nil
	}
	var ae *APIError
	if errors.As(err, &ae) {
		return ae
	}
	var pnf *PerformerNotFoundError
	if errors.As(err, &pnf) {
		return NewBadRequest(err.Error())
	}
	var anf *AttributeNotFoundError
	if errors.As(err, &anf) {
		return NewBadRequest(err.Error())
	}
	switch {
	case errors.Is(err, ErrContentNotFound),
		errors.Is(err, ErrNoEncodingFound),
		errors.Is(err, ErrNoCompletedEncoding),
		errors.Is(err, ErrPlaybackNotAvailable):
		return NewNotFound(err.Error())
	case errors.Is(err, ErrAttributeNameTaken):
		return NewConflict(err.Error())
	case errors.Is(err, ErrFileEmpty), errors.Is(err, ErrResultTooLarge):
		return NewBadRequest(err.Error())
	case errors.Is(err, ErrFileUploaderNotConfigured),
		errors.Is(err, ErrAWSRegionRequired),
		errors.Is(err, ErrS3BucketNotConfigured):
		return NewInternal(err)
	case IsConditionalCheckFailed(err):
		return NewConflict("The resource already exists or has changed")
	case IsThrottled(err):
		return NewRateLimited()
	}
	return NewInternal(err)
}

func Write(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}
	ae := From(err)
	logAPIError(r, ae)
	_ = response.Error(w, r, ae.Status, ae.Code, ae.Message, ae.Details)
}

func logAPIError(r *http.Request, ae *APIError) {
	attrs := []any{
		"request_id", reqID(r),
		"method", method(r),
		"path", path(r),
		"code", ae.Code,
		"status", ae.Status,
	}
	if ae.cause != nil {
		attrs = append(attrs, "error", ae.cause.Error())
	}
	switch {
	case ae.Status >= 500:
		slog.Error(ae.Code, attrs...)
	case ae.Status == http.StatusUnauthorized,
		ae.Status == http.StatusForbidden,
		ae.Status == http.StatusConflict,
		ae.Status == http.StatusTooManyRequests:
		slog.Warn(ae.Code, attrs...)
	default:
		slog.Info(ae.Code, attrs...)
	}
}

func reqID(r *http.Request) string {
	if r == nil {
		return ""
	}
	return middleware.GetReqID(r.Context())
}

func method(r *http.Request) string {
	if r == nil {
		return ""
	}
	return r.Method
}

func path(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	return r.URL.Path
}

func InternalServerError(w http.ResponseWriter, r *http.Request, err error) {
	Write(w, r, NewInternal(err))
}

// safeUserMessage returns a message safe to expose to clients. It returns
// (msg, true) only for *APIError, known typed errors, and known sentinels.
// Unknown errors return ("", false) to prevent leakage of internal details.
func safeUserMessage(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	var ae *APIError
	if errors.As(err, &ae) {
		return ae.Message, true
	}
	var pnf *PerformerNotFoundError
	if errors.As(err, &pnf) {
		return err.Error(), true
	}
	var anf *AttributeNotFoundError
	if errors.As(err, &anf) {
		return err.Error(), true
	}
	switch {
	case errors.Is(err, ErrFileEmpty),
		errors.Is(err, ErrContentNotFound),
		errors.Is(err, ErrNoEncodingFound),
		errors.Is(err, ErrNoCompletedEncoding),
		errors.Is(err, ErrPlaybackNotAvailable),
		errors.Is(err, ErrResultTooLarge),
		errors.Is(err, ErrAttributeNameTaken):
		return err.Error(), true
	}
	return "", false
}

func logUnclassified(r *http.Request, tag string, err error) {
	if err == nil {
		return
	}
	slog.Warn(tag,
		"request_id", reqID(r),
		"method", method(r),
		"path", path(r),
		"error", err.Error(),
	)
}

// classifyOr writes err using the matching status. If err is already an
// *APIError it is forwarded verbatim. If err is a known safe type, its
// message is used; otherwise a generic message is sent and the cause is
// logged separately to avoid leaking internal details.
func classifyOr(w http.ResponseWriter, r *http.Request, err error, defaultStatus int, defaultCode, genericMsg, tag string) {
	if err == nil {
		return
	}
	var ae *APIError
	if errors.As(err, &ae) {
		Write(w, r, ae)
		return
	}
	msg, ok := safeUserMessage(err)
	if !ok {
		logUnclassified(r, tag, err)
		msg = genericMsg
	}
	out := newAPIError(defaultStatus, defaultCode, msg, nil, err)
	Write(w, r, out)
}

func BadRequestError(w http.ResponseWriter, r *http.Request, err error) {
	classifyOr(w, r, err, http.StatusBadRequest, CodeBadRequest, "The request was invalid", "BAD_REQUEST_UNCLASSIFIED")
}

func NotFoundError(w http.ResponseWriter, r *http.Request, err error) {
	classifyOr(w, r, err, http.StatusNotFound, CodeNotFound, "The requested resource was not found", "NOT_FOUND_UNCLASSIFIED")
}

func ConflictError(w http.ResponseWriter, r *http.Request, err error) {
	classifyOr(w, r, err, http.StatusConflict, CodeConflict, "The resource already exists or has changed", "CONFLICT_UNCLASSIFIED")
}

func UnauthorizedError(w http.ResponseWriter, r *http.Request) {
	Write(w, r, NewUnauthorized())
}
