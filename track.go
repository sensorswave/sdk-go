package sensorswave

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// ////////////////////////////////// client inner funcs

type messageQueue struct {
	pending  [][]byte
	size     int // length of pending
	bodySize int
}

func (q *messageQueue) push(msg []byte) (jsonBody []byte) {
	if q.pending == nil { // re init
		q.pending = make([][]byte, 0, maxBatchSize)
		q.size = 0
		q.bodySize = 0
	}
	q.pending = append(q.pending, msg)
	q.size++
	q.bodySize += len(msg)
	if q.size >= maxBatchSize || q.bodySize >= maxHTTPBodySize {
		jsonBody = q.flush()
	}
	return
}

func (q *messageQueue) flush() (jsonBody []byte) {
	if q.size == 0 {
		return
	}
	var buf bytes.Buffer
	buf.Grow(q.size + q.bodySize + 2)
	buf.WriteByte('[')
	for i, msg := range q.pending {
		buf.Write(msg)
		if i < q.size-1 { // last
			buf.WriteByte(',')
		}
	}
	buf.WriteByte(']')
	q.pending = nil
	jsonBody = buf.Bytes()
	return
}

func (c *client) loop() {
	defer c.wg.Done()

	tick := time.NewTicker(c.cfg.FlushInterval)
	defer tick.Stop()

	msgQue := messageQueue{}
	for {
		select {
		case msg := <-c.msgchan:
			_ = c.push(&msgQue, msg)
		case <-tick.C:
			_ = c.flush(&msgQue)
		case <-c.quit:
			c.cfg.Logger.Debugf("loop closing: draining messages")
			for {
				select {
				case msg := <-c.msgchan:
					_ = c.push(&msgQue, msg)
				default:
					_ = c.flush(&msgQue)
					return
				}
			}
		}
	}
}

func (c *client) push(msgq *messageQueue, msg []byte) (err error) {
	if jsonBody := msgq.push(msg); jsonBody != nil {
		c.send(jsonBody)
	}
	return
}

func (c *client) flush(msgq *messageQueue) (err error) {
	if jsonBody := msgq.flush(); jsonBody != nil {
		c.send(jsonBody)
	}
	return
}

func (c *client) send(jsonBody []byte) {
	if len(jsonBody) <= 2 {
		return
	}
	c.wg.Add(1)
	go func(jsonBody []byte) {
		<-c.sem
		defer func() {
			c.sem <- struct{}{}
			c.wg.Done()
		}()

		headers := map[string]string{
			"Content-Type":    "application/json",
			"User-Agent":      "", // Disable default Go User-Agent; SDK info is sent via other headers
			HeaderSourceToken: c.sourceToken,
		}
		requestBody := jsonBody
		if c.cfg.GzipThresholdBytes > 0 && len(jsonBody) > c.cfg.GzipThresholdBytes {
			c.cfg.Logger.Warnf("start gzip")
			gzipBody, err := gzipBytes(jsonBody)
			if err != nil {
				c.cfg.Logger.Errorf("gzip event body error: %v", err)
			} else {
				requestBody = gzipBody
				headers["Content-Encoding"] = "gzip"
			}
		}

		trackURL := strings.TrimRight(c.endpoint, "/") + c.cfg.TrackURIPath
		opts := newRequestOpts().
			WithMethod("POST").
			WithURL(trackURL).
			WithHeaders(headers).
			WithBody(requestBody).
			WithTimeout(c.cfg.HTTPTimeout).
			WithRetry(c.cfg.HTTPRetry)
		_, httpcode, err := c.h.Do(context.Background(), opts)
		if err != nil || httpcode != http.StatusOK {
			c.cfg.Logger.Errorf("http send event error: %v httpcode:%d", err, httpcode)
			if c.cfg.OnTrackFailHandler != nil {
				var events []Event
				if err := json.Unmarshal(jsonBody, &events); err != nil {
					c.cfg.Logger.Errorf("unmarshal fail events error: %v", err)
				}
				c.cfg.OnTrackFailHandler(events, err)
			}
			if len(jsonBody) > 100 {
				c.cfg.Logger.Debugf("http send body body  : (%s)", string(jsonBody[:100]))
			} else {
				c.cfg.Logger.Debugf("http send body body  : (%s)", string(jsonBody))
			}
		} else {
			c.cfg.Logger.Debugf("http send body length: %d ", len(jsonBody))
		}
	}(jsonBody)
}

func gzipBytes(body []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(body); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
