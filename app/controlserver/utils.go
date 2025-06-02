package controlserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/labstack/gommon/log"
)

type GenericResponse[T any] struct {
	IsSuccess bool
	Message   string `json:",omitempty"`
	Data      T      `json:",omitempty"`
}

func JsonMustMarshalIndent(data any) string {
	return string(JsonMustMarshalIndentB(data))
}

func JsonMustMarshalIndentB(data any) []byte {
	var j []byte
	var err error
	j, err = json.MarshalIndent(data, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("JsonMustMarshal(%v), err: %v", data, err.Error()))
	}
	return j
}

var globalReqCount atomic.Int32

func WrapGenericResp[R any](
	fn func(w http.ResponseWriter, r *http.Request) (_ R, msg string, _ error),
) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		reqCount := globalReqCount.Add(1)
		log.Infof("[req=%d] request received: %s", reqCount, r.URL.Path)
		data, msg, err := fn(w, r)
		if err != nil {
			log.Errorf("[req=%d] request handling failed: %v", reqCount, err)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "%s", JsonMustMarshalIndent(GenericResponse[R]{
				IsSuccess: false,
				Message:   err.Error(),
			}))
		} else {
			if msg == "" {
				msg = "OK"
			}
			fmt.Fprintf(w, "%s", JsonMustMarshalIndent(GenericResponse[R]{
				IsSuccess: true,
				Message:   msg,
				Data:      data,
			}))
		}
	}
}
