package httputil

import (
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockReadCloser struct {
	read   bool
	closed bool
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	m.read = true
	p[0] = 42
	return 1, io.EOF
}

func (m *mockReadCloser) Close() error {
	m.closed = true
	return nil
}

func TestMakeDisposer(t *testing.T) {
	t.Run("nil resp", func(t *testing.T) {
		resp, err, dispose := MakeDisposer(nil, nil)
		dispose()

		assert.Nil(t, resp)
		assert.Nil(t, err)
	})

	t.Run("discards and closes body", func(t *testing.T) {
		body := &mockReadCloser{}
		resp, err, dispose := MakeDisposer(&http.Response{Body: body}, nil)
		dispose()

		assert.NotNil(t, resp)
		assert.Nil(t, err)

		assert.True(t, body.read)
		assert.True(t, body.closed)
	})

	t.Run("propagates error", func(t *testing.T) {
		_, err, _ := MakeDisposer(nil, fmt.Errorf("dummy"))

		assert.EqualError(t, err, "dummy")
	})
}
