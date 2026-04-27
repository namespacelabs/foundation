// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"

	"github.com/cenkalti/backoff/v4"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/ctxio"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

// Maximum total time we'll spend retrying transient download failures.
const downloadMaxElapsed = 2 * time.Minute

func URL(ref artifacts.Reference) compute.Computable[bytestream.ByteStream] {
	return &downloadUrl{url: ref.URL, digest: &ref.Digest}
}

// Must only be used when it's guaranteed that the output does not change based on the presence of Namespace headers.
func NamespaceURL(ref artifacts.Reference, values url.Values) compute.Computable[bytestream.ByteStream] {
	return &downloadUrl{url: ref.URL, digest: &ref.Digest, useNamespaceHeaders: true, additionalValues: values}
}

func UnverifiedURL(url string) compute.Computable[bytestream.ByteStream] {
	return &downloadUrl{url: url}
}

type downloadUrl struct {
	url                 string
	digest              *schema.Digest
	useNamespaceHeaders bool       // Does not affect the output.
	additionalValues    url.Values // Does not affect the output.

	compute.LocalScoped[bytestream.ByteStream]
}

func (dl *downloadUrl) Action() *tasks.ActionEvent {
	action := tasks.Action("artifact.download").Arg("url", dl.url)
	if dl.digest != nil {
		return action.Arg("digest", dl.digest.String())
	}
	return action
}

func (dl *downloadUrl) Inputs() *compute.In {
	inputs := compute.Inputs().Str("url", dl.url)
	if dl.digest != nil {
		return inputs.Digest("digest", dl.digest)
	} else {
		return inputs.Indigestible("digest", nil) // Don't cache.
	}
}

func (dl *downloadUrl) Compute(ctx context.Context, _ compute.Resolved) (bytestream.ByteStream, error) {
	url := dl.url

	if query := dl.additionalValues.Encode(); query != "" {
		url += "?" + query
	}

	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 500 * time.Millisecond
	b.RandomizationFactor = 0.5
	b.Multiplier = 1.5
	b.MaxInterval = 5 * time.Second
	b.MaxElapsedTime = downloadMaxElapsed
	b.Reset()

	var result bytestream.ByteStream
	attempt := 0
	err := backoff.Retry(func() error {
		attempt++
		bs, finalURL, err := dl.attempt(ctx, url)
		if err == nil {
			result = bs
			return nil
		}
		if isRetryableDownloadError(err) {
			fmt.Fprintf(console.Warnings(ctx), "artifact.download: %s: attempt %d failed with transient error, retrying (final url=%s): %v\n", dl.url, attempt, finalURL, err)
			return err
		}
		fmt.Fprintf(console.Warnings(ctx), "artifact.download: %s: attempt %d failed with permanent error (final url=%s): %v\n", dl.url, attempt, finalURL, err)
		return backoff.Permanent(err)
	}, backoff.WithContext(b, ctx))
	if err != nil {
		return nil, err
	}
	return result, nil
}

// attempt performs a single download attempt. It always returns the final URL it observed
// (after following redirects); on error this lets callers log where the failure actually
// originated rather than just the user-supplied URL.
func (dl *downloadUrl) attempt(ctx context.Context, requestURL string) (bytestream.ByteStream, string, error) {
	finalURL := requestURL

	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, finalURL, err
	}

	if dl.useNamespaceHeaders {
		fnapi.AddJsonNamespaceHeaders(ctx, req.Header)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// *url.Error.URL is the URL of the request that failed, which reflects the post-redirect
		// URL when the failure happens after a redirect was followed.
		var urlErr *url.Error
		if errors.As(err, &urlErr) && urlErr.URL != "" {
			finalURL = urlErr.URL
		}
		return nil, finalURL, err
	}

	defer resp.Body.Close()

	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}

	if resp.StatusCode != http.StatusOK {
		return nil, finalURL, newHTTPStatusError(dl.url, resp.StatusCode)
	}

	bsw, err := compute.NewByteStream(ctx)
	if err != nil {
		return nil, finalURL, err
	}

	defer bsw.Close()

	var p artifacts.ProgressWriter
	if resp.ContentLength >= 0 {
		p = artifacts.NewProgressWriter(uint64(resp.ContentLength), nil)
	} else {
		p = artifacts.NewProgressWriter(0, nil)
	}

	tasks.Attachments(ctx).SetProgress(p)

	w := io.MultiWriter(bsw, p)

	if _, err := io.Copy(ctxio.WriterWithContext(ctx, w, nil), resp.Body); err != nil {
		return nil, finalURL, err
	}

	bs, err := bsw.Complete()
	if err != nil {
		return nil, finalURL, err
	}

	if dl.digest != nil {
		resultDigest, err := bytestream.Digest(ctx, bs)
		if err != nil {
			return nil, finalURL, err
		}

		if !resultDigest.Equals(*dl.digest) {
			// Treat as transient: a truncated/corrupted body may not surface as a transport-level
			// EOF (e.g. with a known Content-Length the server can still deliver short bytes that
			// are mistakenly framed as a clean close). Retrying recovers from those cases.
			return nil, finalURL, newDigestMismatchError(dl.url, resultDigest.String(), dl.digest.String())
		}
	}

	// XXX support returning a io.Reader here so we don't need to buffer the download.
	return bs, finalURL, nil
}

type httpStatusError struct {
	url    string
	status int
	err    error
}

func (e *httpStatusError) Error() string { return e.err.Error() }
func (e *httpStatusError) Unwrap() error { return e.err }

func newHTTPStatusError(url string, status int) *httpStatusError {
	return &httpStatusError{
		url:    url,
		status: status,
		err:    fnerrors.InvocationError("http", "failed to download %s: got status %d", url, status),
	}
}

type digestMismatchError struct {
	err error
}

func (e *digestMismatchError) Error() string { return e.err.Error() }
func (e *digestMismatchError) Unwrap() error { return e.err }

func newDigestMismatchError(url, got, want string) *digestMismatchError {
	return &digestMismatchError{
		err: fnerrors.InternalError("artifact.download: %s: digest didn't match, got %s expected %s", url, got, want),
	}
}

// isRetryableDownloadError returns true if err is likely to be transient and worth retrying.
func isRetryableDownloadError(err error) bool {
	if err == nil {
		return false
	}

	// Don't retry on cancellation/deadline of the caller context.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Mid-body truncation — the symptom we're trying to fix here.
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return true
	}

	// Connection reset, broken pipe, etc.
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}

	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		// net/http surfaces transient transport errors wrapped in *url.Error; if the underlying
		// error wasn't already matched above, treat any url.Error other than a context error as
		// retryable.
		if temp, ok := urlErr.Err.(interface{ Temporary() bool }); ok && temp.Temporary() {
			return true
		}
		return urlErr.Timeout()
	}

	var statusErr *httpStatusError
	if errors.As(err, &statusErr) {
		switch statusErr.status {
		case http.StatusRequestTimeout, // 408
			http.StatusTooManyRequests,     // 429
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout:      // 504
			return true
		}
		return false
	}

	var digestErr *digestMismatchError
	if errors.As(err, &digestErr) {
		return true
	}

	return false
}
