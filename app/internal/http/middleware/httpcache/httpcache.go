package httpcache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type Entry struct {
	Status      int
	ContentType string
	Body        []byte
	StoredAt    time.Time
	TTL         time.Duration
}

func (e Entry) Expired(now time.Time) bool {
	return e.TTL > 0 && now.Sub(e.StoredAt) >= e.TTL
}

type Store interface {
	Get(key string) (Entry, bool)
	Set(key string, entry Entry)
}

type MemoryStore struct {
	mu       sync.RWMutex
	entries  map[string]Entry
	order    []string
	capacity int
}

func NewMemoryStore(capacity int) *MemoryStore {
	if capacity <= 0 {
		capacity = 1024
	}
	return &MemoryStore{
		entries:  make(map[string]Entry, capacity),
		order:    make([]string, 0, capacity),
		capacity: capacity,
	}
}

func (m *MemoryStore) Get(key string) (Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	e, ok := m.entries[key]
	if !ok {
		return Entry{}, false
	}
	if e.Expired(time.Now()) {
		return Entry{}, false
	}
	return e, true
}

func (m *MemoryStore) Set(key string, entry Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.entries[key]; !exists {
		if len(m.order) >= m.capacity {
			oldest := m.order[0]
			m.order = m.order[1:]
			delete(m.entries, oldest)
		}
		m.order = append(m.order, key)
	}
	m.entries[key] = entry
}

type Options struct {
	TTL time.Duration

	VaryHeaders []string
}

func Middleware(store Store, opts Options) func(http.Handler) http.Handler {
	if store == nil {
		store = NewMemoryStore(0)
	}
	vary := make([]string, len(opts.VaryHeaders))
	for i, h := range opts.VaryHeaders {
		vary[i] = http.CanonicalHeaderKey(h)
	}
	sort.Strings(vary)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}
			if hasNoStore(r.Header.Get("Cache-Control")) {
				next.ServeHTTP(w, r)
				return
			}

			key := makeKey(r, vary)

			if entry, ok := store.Get(key); ok {
				replay(w, entry)
				return
			}

			rec := newRecorder(w)
			next.ServeHTTP(rec, r)

			if !rec.cacheable() {
				return
			}
			store.Set(key, Entry{
				Status:      rec.status,
				ContentType: rec.Header().Get("Content-Type"),
				Body:        bytes.Clone(rec.body.Bytes()),
				StoredAt:    time.Now(),
				TTL:         opts.TTL,
			})
		})
	}
}

func hasNoStore(cc string) bool {
	cc = strings.ToLower(cc)
	for _, part := range strings.Split(cc, ",") {
		if strings.TrimSpace(part) == "no-store" {
			return true
		}
	}
	return false
}

func makeKey(r *http.Request, vary []string) string {
	h := sha256.New()
	h.Write([]byte(r.Method))
	h.Write([]byte{'\x1f'})
	h.Write([]byte(r.URL.Path))
	h.Write([]byte{'\x1f'})

	if r.URL.RawQuery != "" {

		q := r.URL.Query()
		keys := make([]string, 0, len(q))
		for k := range q {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			vals := append([]string(nil), q[k]...)
			sort.Strings(vals)
			h.Write([]byte(k))
			h.Write([]byte{'='})
			h.Write([]byte(strings.Join(vals, ",")))
			h.Write([]byte{'&'})
		}
	}
	h.Write([]byte{'\x1f'})

	for _, name := range vary {
		h.Write([]byte(name))
		h.Write([]byte{':'})
		h.Write([]byte(r.Header.Get(name)))
		h.Write([]byte{'\x1f'})
	}

	return hex.EncodeToString(h.Sum(nil))
}

func replay(w http.ResponseWriter, e Entry) {
	if e.ContentType != "" {
		w.Header().Set("Content-Type", e.ContentType)
	}
	w.Header().Set("X-Cache", "HIT")
	if e.Status == 0 {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(e.Status)
	}
	_, _ = w.Write(e.Body)
}

type recorder struct {
	http.ResponseWriter
	status      int
	body        bytes.Buffer
	wroteHeader bool
}

func newRecorder(w http.ResponseWriter) *recorder {
	return &recorder{ResponseWriter: w}
}

func (r *recorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.status = code
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *recorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *recorder) cacheable() bool {
	if r.status < 200 || r.status >= 300 {
		return false
	}
	h := r.Header()
	if h.Get("Set-Cookie") != "" {
		return false
	}
	cc := strings.ToLower(h.Get("Cache-Control"))
	for _, part := range strings.Split(cc, ",") {
		switch strings.TrimSpace(part) {
		case "no-store", "private":
			return false
		}
	}
	return true
}
