package respwriter

import "net/http"

type Recorder struct {
	http.ResponseWriter
	Status int
	Bytes  int
}

func New(w http.ResponseWriter) *Recorder {
	return &Recorder{ResponseWriter: w}
}

func (r *Recorder) WriteHeader(code int) {
	r.Status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *Recorder) Write(b []byte) (int, error) {
	if r.Status == 0 {
		r.Status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.Bytes += n
	return n, err
}
