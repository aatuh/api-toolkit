package objectstores3

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aatuh/api-toolkit/contrib/v3/objectstore"
	"github.com/aatuh/api-toolkit/v3/endpoints/health"
)

const (
	defaultHTTPTimeout = 10 * time.Second
	amzDateFormat      = "20060102T150405Z"
	shortDateFormat    = "20060102"
	unsignedPayload    = "UNSIGNED-PAYLOAD"
)

var (
	// ErrInvalidConfig reports invalid S3 adapter configuration.
	ErrInvalidConfig = errors.New("invalid s3 object store config")
	// ErrStoreNotConfigured reports a nil or incomplete store.
	ErrStoreNotConfigured = errors.New("s3 object store not configured")
)

// Options configures an S3-compatible object store.
type Options struct {
	Endpoint            string
	Region              string
	Bucket              string
	AccessKeyID         string
	SecretAccessKey     string
	SessionToken        string
	HTTPClient          *http.Client
	Clock               func() time.Time
	MaxObjectSize       int64
	AllowedContentTypes []string
}

// Store implements objectstore.Store using the S3 HTTP API and SigV4 signing.
type Store struct {
	endpoint        *url.URL
	region          string
	defaultBucket   string
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
	client          *http.Client
	clock           func() time.Time
	policy          objectstore.Policy
}

var (
	_ objectstore.Store       = (*Store)(nil)
	_ objectstore.SignedURLer = (*Store)(nil)
)

// New creates an S3-compatible object store.
func New(opts Options) (*Store, error) {
	endpoint, err := url.Parse(strings.TrimRight(strings.TrimSpace(opts.Endpoint), "/"))
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return nil, fmt.Errorf("%w: endpoint is required", ErrInvalidConfig)
	}
	region := strings.TrimSpace(opts.Region)
	if region == "" {
		return nil, fmt.Errorf("%w: region is required", ErrInvalidConfig)
	}
	accessKeyID := strings.TrimSpace(opts.AccessKeyID)
	secretAccessKey := strings.TrimSpace(opts.SecretAccessKey)
	if accessKeyID == "" || secretAccessKey == "" {
		return nil, fmt.Errorf("%w: credentials are required", ErrInvalidConfig)
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	clock := opts.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	return &Store{
		endpoint:        endpoint,
		region:          region,
		defaultBucket:   strings.TrimSpace(opts.Bucket),
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
		sessionToken:    strings.TrimSpace(opts.SessionToken),
		client:          client,
		clock:           clock,
		policy: objectstore.Policy{
			MaxObjectSize:       opts.MaxObjectSize,
			AllowedContentTypes: append([]string(nil), opts.AllowedContentTypes...),
		},
	}, nil
}

// Put writes an object.
func (s *Store) Put(ctx context.Context, ref objectstore.Ref, body io.Reader, opts objectstore.PutOptions) error {
	if s == nil || s.endpoint == nil || s.client == nil {
		return ErrStoreNotConfigured
	}
	ref = s.withDefaultBucket(ref)
	if err := objectstore.ValidateRef(ref); err != nil {
		return err
	}
	metadata, err := objectstore.SafeMetadata(opts.Metadata)
	if err != nil {
		return err
	}
	if err := s.policy.ValidatePut(opts); err != nil {
		return err
	}
	payload, size, err := objectstore.ReadAtMost(body, s.policy.MaxObjectSize)
	if err != nil {
		return err
	}
	if opts.Size > 0 && opts.Size != size {
		return objectstore.ErrInvalidObject
	}
	opts.Size = size
	target := s.objectURL(ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, target.String(), bytes.NewReader(payload)) // #nosec G107 -- endpoint is operator-controlled object storage configuration.
	if err != nil {
		return err
	}
	req.ContentLength = size
	if opts.ContentType != "" {
		req.Header.Set("Content-Type", strings.TrimSpace(opts.ContentType))
	}
	for key, value := range metadata {
		req.Header.Set("X-Amz-Meta-"+key, value)
	}
	if err := s.signRequest(req, payloadHash(payload)); err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { closeAndDiscard(resp.Body) }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return s.statusError("put", resp.StatusCode)
	}
	return nil
}

// Get reads an object. The caller owns the returned body.
func (s *Store) Get(ctx context.Context, ref objectstore.Ref) (objectstore.GetResult, error) {
	if s == nil || s.endpoint == nil || s.client == nil {
		return objectstore.GetResult{}, ErrStoreNotConfigured
	}
	ref = s.withDefaultBucket(ref)
	if err := objectstore.ValidateRef(ref); err != nil {
		return objectstore.GetResult{}, err
	}
	target := s.objectURL(ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil) // #nosec G107 -- endpoint is operator-controlled object storage configuration.
	if err != nil {
		return objectstore.GetResult{}, err
	}
	if err := s.signRequest(req, unsignedPayload); err != nil {
		return objectstore.GetResult{}, err
	}
	resp, err := s.client.Do(req) //nolint:bodyclose // successful response body ownership is returned to the caller.
	if err != nil {
		return objectstore.GetResult{}, err
	}
	if resp.StatusCode == http.StatusNotFound {
		closeAndDiscard(resp.Body)
		return objectstore.GetResult{}, objectstore.ErrObjectNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		closeAndDiscard(resp.Body)
		return objectstore.GetResult{}, s.statusError("get", resp.StatusCode)
	}
	return objectstore.GetResult{
		Body:        resp.Body,
		ContentType: resp.Header.Get("Content-Type"),
		Size:        resp.ContentLength,
		Metadata:    responseMetadata(resp.Header),
		ETag:        resp.Header.Get("ETag"),
	}, nil
}

// Delete removes an object. Missing objects are treated as successfully deleted.
func (s *Store) Delete(ctx context.Context, ref objectstore.Ref) error {
	if s == nil || s.endpoint == nil || s.client == nil {
		return ErrStoreNotConfigured
	}
	ref = s.withDefaultBucket(ref)
	if err := objectstore.ValidateRef(ref); err != nil {
		return err
	}
	target := s.objectURL(ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, target.String(), nil) // #nosec G107 -- endpoint is operator-controlled object storage configuration.
	if err != nil {
		return err
	}
	if err := s.signRequest(req, unsignedPayload); err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { closeAndDiscard(resp.Body) }()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return s.statusError("delete", resp.StatusCode)
	}
	return nil
}

// SignedURL creates a SigV4 pre-signed URL for a supported S3 object operation.
func (s *Store) SignedURL(ctx context.Context, ref objectstore.Ref, opts objectstore.SignedURLOptions) (string, error) {
	if s == nil || s.endpoint == nil {
		return "", ErrStoreNotConfigured
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	ref = s.withDefaultBucket(ref)
	if err := objectstore.ValidateRef(ref); err != nil {
		return "", err
	}
	opts, err := objectstore.NormalizeSignedURLOptions(opts)
	if err != nil {
		return "", err
	}
	target := s.objectURL(ref)
	now := s.clock().UTC()
	scope := credentialScope(now, s.region)
	query := target.Query()
	query.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	query.Set("X-Amz-Credential", s.accessKeyID+"/"+scope)
	query.Set("X-Amz-Date", now.Format(amzDateFormat))
	query.Set("X-Amz-Expires", strconv.FormatInt(int64(opts.Expires/time.Second), 10))
	signedHeaders := []string{"host"}
	headers := map[string]string{"host": target.Host}
	if opts.ContentType != "" {
		signedHeaders = append(signedHeaders, "content-type")
		headers["content-type"] = strings.TrimSpace(opts.ContentType)
	}
	sort.Strings(signedHeaders)
	query.Set("X-Amz-SignedHeaders", strings.Join(signedHeaders, ";"))
	if s.sessionToken != "" {
		query.Set("X-Amz-Security-Token", s.sessionToken)
	}
	target.RawQuery = canonicalQuery(query)
	canonical := strings.Join([]string{
		opts.Method,
		canonicalURI(target),
		target.RawQuery,
		canonicalHeadersFromMap(headers, signedHeaders),
		strings.Join(signedHeaders, ";"),
		unsignedPayload,
	}, "\n")
	signature := s.signature(now, canonical)
	query.Set("X-Amz-Signature", signature)
	target.RawQuery = canonicalQuery(query)
	return target.String(), nil
}

// HealthChecker returns a bucket-level health checker for this store.
func (s *Store) HealthChecker() health.Checker {
	return health.NewCustomChecker(
		"s3-objectstore",
		func(ctx context.Context) (health.Status, string, interface{}) {
			if s == nil || s.endpoint == nil || s.client == nil {
				return health.StatusUnhealthy, "s3 object store not configured", nil
			}
			bucket := strings.TrimSpace(s.defaultBucket)
			if bucket == "" {
				return health.StatusHealthy, "s3 object store configured without default bucket", nil
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodHead, s.objectURL(objectstore.Ref{Bucket: bucket, Key: "healthcheck"}).String(), nil) // #nosec G107 -- endpoint is operator-controlled object storage configuration.
			if err != nil {
				return health.StatusUnhealthy, "s3 object store health request failed", nil
			}
			if err := s.signRequest(req, unsignedPayload); err != nil {
				return health.StatusUnhealthy, "s3 object store signing failed", nil
			}
			resp, err := s.client.Do(req)
			if err != nil {
				return health.StatusUnhealthy, fmt.Sprintf("s3 object store health failed: %v", err), nil
			}
			defer func() {
				_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
				_ = resp.Body.Close()
			}()
			if resp.StatusCode == http.StatusNotFound || (resp.StatusCode >= 200 && resp.StatusCode < 300) {
				return health.StatusHealthy, "s3 object store healthy", nil
			}
			return health.StatusUnhealthy, fmt.Sprintf("s3 object store health status %d", resp.StatusCode), nil
		},
	)
}

func (s *Store) withDefaultBucket(ref objectstore.Ref) objectstore.Ref {
	if strings.TrimSpace(ref.Bucket) == "" {
		ref.Bucket = s.defaultBucket
	}
	ref.Bucket = strings.TrimSpace(ref.Bucket)
	ref.Key = strings.TrimSpace(ref.Key)
	return ref
}

func (s *Store) objectURL(ref objectstore.Ref) *url.URL {
	target := *s.endpoint
	basePath := strings.TrimRight(target.Path, "/")
	target.Path = basePath + "/" + ref.Bucket + "/" + ref.Key
	target.RawPath = ""
	target.RawQuery = ""
	return &target
}

func (s *Store) signRequest(req *http.Request, payloadHashValue string) error {
	if req == nil || req.URL == nil {
		return ErrStoreNotConfigured
	}
	now := s.clock().UTC()
	req.Header.Set("X-Amz-Date", now.Format(amzDateFormat))
	req.Header.Set("X-Amz-Content-Sha256", payloadHashValue)
	if s.sessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", s.sessionToken)
	}
	signedHeaders := signedHeaderNames(req)
	canonical := strings.Join([]string{
		req.Method,
		canonicalURI(req.URL),
		canonicalQuery(req.URL.Query()),
		canonicalHeaders(req, signedHeaders),
		strings.Join(signedHeaders, ";"),
		payloadHashValue,
	}, "\n")
	signature := s.signature(now, canonical)
	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.accessKeyID,
		credentialScope(now, s.region),
		strings.Join(signedHeaders, ";"),
		signature,
	))
	return nil
}

func (s *Store) signature(now time.Time, canonicalRequest string) string {
	scope := credentialScope(now, s.region)
	canonicalHash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		now.Format(amzDateFormat),
		scope,
		hex.EncodeToString(canonicalHash[:]),
	}, "\n")
	signingKey := sigV4Key(s.secretAccessKey, now.Format(shortDateFormat), s.region)
	return hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))
}

func payloadHash(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func credentialScope(now time.Time, region string) string {
	return now.Format(shortDateFormat) + "/" + region + "/s3/aws4_request"
}

func sigV4Key(secret, date, region string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte("s3"))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func hmacSHA256(key, value []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(value)
	return mac.Sum(nil)
}

func signedHeaderNames(req *http.Request) []string {
	seen := map[string]struct{}{"host": {}}
	for name := range req.Header {
		lower := strings.ToLower(name)
		if lower == "authorization" {
			continue
		}
		seen[lower] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func canonicalHeaders(req *http.Request, names []string) string {
	values := make(map[string]string, len(names))
	for _, name := range names {
		if name == "host" {
			values[name] = req.URL.Host
			continue
		}
		values[name] = normalizeHeaderValue(strings.Join(req.Header.Values(name), ","))
	}
	return canonicalHeadersFromMap(values, names)
}

func canonicalHeadersFromMap(values map[string]string, names []string) string {
	var b strings.Builder
	for _, name := range names {
		b.WriteString(name)
		b.WriteByte(':')
		b.WriteString(normalizeHeaderValue(values[name]))
		b.WriteByte('\n')
	}
	return b.String()
}

func normalizeHeaderValue(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func canonicalURI(u *url.URL) string {
	escaped := u.EscapedPath()
	if escaped == "" {
		return "/"
	}
	return escaped
}

func canonicalQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(values))
	for _, key := range keys {
		vals := append([]string(nil), values[key]...)
		sort.Strings(vals)
		for _, value := range vals {
			parts = append(parts, awsQueryEscape(key)+"="+awsQueryEscape(value))
		}
	}
	return strings.Join(parts, "&")
}

func awsQueryEscape(value string) string {
	escaped := url.QueryEscape(value)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	escaped = strings.ReplaceAll(escaped, "%7E", "~")
	return escaped
}

func responseMetadata(header http.Header) map[string]string {
	const prefix = "X-Amz-Meta-"
	metadata := map[string]string{}
	for name, values := range header {
		if !strings.HasPrefix(http.CanonicalHeaderKey(name), prefix) || len(values) == 0 {
			continue
		}
		key := strings.TrimPrefix(http.CanonicalHeaderKey(name), prefix)
		key = strings.ToLower(key)
		metadata[key] = values[0]
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func (s *Store) statusError(operation string, status int) error {
	return fmt.Errorf("s3 object store %s failed: status %d", operation, status)
}

func closeAndDiscard(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 4<<10))
	_ = body.Close()
}
