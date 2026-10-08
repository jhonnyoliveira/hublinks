package httpx

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover transforma um pânico no handler em resposta 500, registrando o erro e
// a pilha (sem dados da requisição). http.ErrAbortHandler segue o fluxo normal
// do net/http, que encerra a conexão sem registrar.
func Recover(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler {
				panic(v)
			}
			logger.Error("panic_recovered", "panic", v, "stack", string(debug.Stack()))
			// se a resposta já começou, este cabeçalho é ignorado; é o melhor possível
			http.Error(w, "erro interno", http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}
