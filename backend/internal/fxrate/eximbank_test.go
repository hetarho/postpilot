package fxrate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEximbankRateContract(t *testing.T) {
	for _, tc := range []struct {
		name, body         string
		status             int
		rate               int64
		published, wantErr bool
	}{
		{"USD", `[{"result":1,"cur_unit":"JPY(100)","deal_bas_r":"950.1"},{"result":1,"cur_unit":"USD","deal_bas_r":"1,392.50"}]`, 200, 13_925_000, true, false},
		{"holiday", `[]`, 200, 0, false, false},
		{"shape", `[{"result":1,"cur_unit":"EUR","deal_bas_r":"1,500"}]`, 200, 0, false, true},
		{"result", `[{"result":0,"cur_unit":"USD","deal_bas_r":"1,392.50"}]`, 200, 0, false, true},
		{"status", `{}`, 503, 0, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var query string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				query = r.URL.RawQuery
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client := NewEximbank("secret", server.Client())
			client.endpoint = server.URL
			rate, published, err := client.KRWPerUSD(context.Background(), time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC))
			if (err != nil) != tc.wantErr || rate != tc.rate || published != tc.published {
				t.Fatalf("rate=%d published=%v err=%v", rate, published, err)
			}
			if query != "authkey=secret&data=AP01&searchdate=20260904" {
				t.Fatalf("query=%s", query)
			}
		})
	}
}
