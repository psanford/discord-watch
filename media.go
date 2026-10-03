package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

type mediaFetcher struct {
	dir      string
	maxBytes int64
	client   *http.Client
	sem      chan struct{}
}

func newMediaFetcher(dir string, maxBytes int64) (*mediaFetcher, error) {
	for _, sub := range []string{"tmp", "ready"} {
		err := os.MkdirAll(filepath.Join(dir, sub), 0o755)
		if err != nil {
			return nil, err
		}
	}

	return &mediaFetcher{
		dir:      dir,
		maxBytes: maxBytes,
		client:   &http.Client{Timeout: 5 * time.Minute},
		sem:      make(chan struct{}, 4),
	}, nil
}

func (f *mediaFetcher) fetchAttachments(m *discordgo.Message) {
	for _, a := range m.Attachments {
		if a.Size > 0 && int64(a.Size) > f.maxBytes {
			log.Printf("media skip: attachment %s on message %s is %d bytes, over limit %d", a.ID, m.ID, a.Size, f.maxBytes)
			continue
		}
		go f.fetch(m, a)
	}
}

func (f *mediaFetcher) fetch(m *discordgo.Message, a *discordgo.MessageAttachment) {
	f.sem <- struct{}{}
	defer func() { <-f.sem }()

	ts := m.Timestamp.UTC()
	rel := filepath.Join(
		ts.Format("2006"), ts.Format("01"), ts.Format("02"),
		safeName(m.ChannelID), safeName(m.ID),
		safeName(a.ID)+"-"+safeName(a.Filename),
	)

	var err error
	for attempt := range 3 {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 10 * time.Second)
		}
		err = f.download(a.URL, rel)
		if err == nil || err == errTooLarge {
			break
		}
	}
	if err != nil {
		log.Printf("media error: attachment %s on message %s: %v", a.ID, m.ID, err)
		return
	}
	log.Printf("media saved: %s", rel)
}

var errTooLarge = fmt.Errorf("attachment exceeds size limit")

func (f *mediaFetcher) download(url, rel string) error {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	if resp.ContentLength > f.maxBytes {
		return errTooLarge
	}

	tmp, err := os.CreateTemp(filepath.Join(f.dir, "tmp"), "dl-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	n, err := io.Copy(tmp, io.LimitReader(resp.Body, f.maxBytes+1))
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if n > f.maxBytes {
		return errTooLarge
	}

	dst := filepath.Join(f.dir, "ready", rel)
	err = os.MkdirAll(filepath.Dir(dst), 0o755)
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

func safeName(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r < 0x20 {
			return '_'
		}
		return r
	}, s)
	if len(s) > 200 {
		s = strings.ToValidUTF8(s[:200], "")
	}
	if s == "" || s == "." || s == ".." {
		s = "_"
	}
	return s
}
