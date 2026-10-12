package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hublinks/hublinks/internal/config"
)

func TestMaintenanceCommandValidatesArguments(t *testing.T) {
	tz, _ := time.LoadLocation("America/Sao_Paulo")
	c := config.Config{ReportTZ: tz, DatabaseURL: "postgres://invalido:invalido@127.0.0.1:1/x"}
	today := time.Now().In(tz).Format("2006-01-02")
	future := time.Now().In(tz).AddDate(0, 0, 3).Format("2006-01-02")
	for _, c2 := range []struct {
		args []string
		want string
	}{
		{nil, "uso:"},
		{[]string{"agora"}, "uso:"},
		{[]string{"run", "extra"}, "uso:"},
		{[]string{"run", "--desconhecida"}, "uso:"},
		{[]string{"run", "--day", "ontem"}, "YYYY-MM-DD"},
		{[]string{"run", "--day", today}, "ainda não terminou"},
		{[]string{"run", "--day", future}, "ainda não terminou"},
	} {
		var out bytes.Buffer
		err := runMaintenance(context.Background(), c, c2.args, &out)
		if err == nil || !strings.Contains(err.Error(), c2.want) {
			t.Errorf("%v: erro = %v, esperado conter %q", c2.args, err, c2.want)
		}
	}
}
