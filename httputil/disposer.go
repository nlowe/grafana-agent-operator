package httputil

import (
	"io"
	"io/ioutil"
	"net/http"
)

func MakeDisposer(resp *http.Response, err error) (*http.Response, error, func()) {
	return resp, err, func() {
		if resp != nil {
			defer func() {
				_ = resp.Body.Close()
			}()
			_, _ = io.Copy(ioutil.Discard, resp.Body)
		}
	}
}
