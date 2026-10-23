// Package netutil provides shared utility networking functions.
package netutil

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"codeberg.org/puregotk/puregotk/v4/glib"
	"codeberg.org/puregotk/puregotk/v4/gtk"
)

// Client is the HTTP client used for all downloads. Connections are
// bounded by a timeout to avoid hanging forever on a stalled mirror;
// the body itself may take as long as it needs.
var Client = &http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ResponseHeaderTimeout: 30 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConnsPerHost:   8,
		ForceAttemptHTTP2:     true,
	},
}

type progressCounter struct {
	current atomic.Uint64
}

func (pc *progressCounter) Write(p []byte) (int, error) {
	pc.current.Add(uint64(len(p)))
	return len(p), nil
}

// ErrBadStatus is the error returned by Download and Body
// if the returned HTTP status code is not http.StatusOK.
var ErrBadStatus = errors.New("bad status")

func get(url string) (*http.Response, error) {
	resp, err := Client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%w: %s", ErrBadStatus, resp.Status)
	}
	return resp, nil
}

// DownloadProgress downloads the named url to the named file, reporting
// the progress to the given progress bar. No retry will be checked here.
// On failure, the partially downloaded file is removed.
func DownloadProgress(url, file string, pbar *gtk.ProgressBar) (err error) {
	resp, err := get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(file)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(file)
		}
	}()

	pc := &progressCounter{}
	total := resp.ContentLength
	var done atomic.Bool

	var tcb glib.SourceFunc
	tcb = func(uintptr) bool {
		if done.Load() {
			glib.UnrefCallback(&tcb)
			return false
		}
		if total > 0 {
			pbar.SetFraction(float64(pc.current.Load()) / float64(total))
		} else {
			pbar.Pulse()
		}
		return true
	}
	glib.TimeoutAdd(16, &tcb, 0)
	defer done.Store(true)

	_, err = io.Copy(out, io.TeeReader(resp.Body, pc))
	return err
}

// Download downloads the named url to the named file. If an error
// occurs when downloading the file. Download will retry 3 times before
// returning a final error.
func Download(url, file string) error {
	const retries = 3
	var err error
	for i := range retries {
		err = download(url, file)
		if err == nil {
			return nil
		}
		os.Remove(file)

		// File and status errors will not be resolved by retrying.
		var perr *os.PathError
		if errors.As(err, &perr) || errors.Is(err, ErrBadStatus) {
			return err
		}

		slog.Warn("Download failed, retrying...", "url", url, "err", err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	return err
}

func download(url, file string) error {
	resp, err := get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(file)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
