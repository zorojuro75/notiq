package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func newAuthRouter(user, pass string) *gin.Engine {
	r := gin.New()
	r.GET("/protected", BasicAuth(user, pass), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

func TestBasicAuth(t *testing.T) {
	const user, pass = "admin", "s3cret"
	r := newAuthRouter(user, pass)

	cases := []struct {
		name     string
		setAuth  bool
		user     string
		pass     string
		wantCode int
	}{
		{"valid credentials", true, user, pass, http.StatusOK},
		{"wrong password", true, user, "nope", http.StatusUnauthorized},
		{"wrong username", true, "root", pass, http.StatusUnauthorized},
		{"no auth header", false, "", "", http.StatusUnauthorized},
		{"empty credentials", true, "", "", http.StatusUnauthorized},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if c.setAuth {
				req.SetBasicAuth(c.user, c.pass)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != c.wantCode {
				t.Errorf("got %d, want %d", w.Code, c.wantCode)
			}
			if c.wantCode == http.StatusUnauthorized {
				if h := w.Header().Get("WWW-Authenticate"); h == "" {
					t.Error("expected WWW-Authenticate header on 401")
				}
			}
		})
	}
}
