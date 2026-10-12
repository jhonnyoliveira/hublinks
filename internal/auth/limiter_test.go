package auth

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync"
	"testing"
	"time"
)

const pepper = "12345678901234567890123456789012"

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestLimiter(max int, window, lockout time.Duration) (*LoginLimiter, *clock) {
	c := &clock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	l := NewLoginLimiter(pepper, max, window, lockout)
	l.now = c.now
	return l, c
}

func from(ip string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	r.RemoteAddr = ip + ":4000"
	return r
}

func TestFiveFailuresFromOneIPBlockForTheLockout(t *testing.T) {
	l, c := newTestLimiter(5, 15*time.Minute, 15*time.Minute)
	r := from("198.51.100.8")
	for i := 0; i < 4; i++ {
		l.Fail(r, "a@example.test")
		if _, blocked := l.Blocked(r, "a@example.test"); blocked {
			t.Fatalf("bloqueou na falha %d", i+1)
		}
	}
	l.Fail(r, "a@example.test")
	wait, blocked := l.Blocked(r, "a@example.test")
	if !blocked || wait != 15*time.Minute {
		t.Fatalf("5ª falha deveria bloquear por 15 min: %v %v", blocked, wait)
	}
	// o bloqueio vale para qualquer e-mail vindo do mesmo IP
	if _, blocked = l.Blocked(r, "outro@example.test"); !blocked {
		t.Fatal("o IP bloqueado deveria recusar outros e-mails")
	}
	c.advance(14 * time.Minute)
	if wait, blocked = l.Blocked(r, "a@example.test"); !blocked || wait != time.Minute {
		t.Fatalf("restava 1 min: %v %v", blocked, wait)
	}
	c.advance(time.Minute)
	if _, blocked = l.Blocked(r, "a@example.test"); blocked {
		t.Fatal("depois do prazo volta a aceitar")
	}
}

func TestFailuresFromManyIPsBlockTheEmail(t *testing.T) {
	l, _ := newTestLimiter(5, 15*time.Minute, 15*time.Minute)
	for i := 0; i < 5; i++ {
		l.Fail(from("198.51.100."+string(rune('1'+i))), "Alvo@Example.test ")
	}
	// e-mail normalizado: maiúsculas e espaços não escapam do limite
	if _, blocked := l.Blocked(from("203.0.113.77"), "alvo@example.test"); !blocked {
		t.Fatal("o e-mail deveria estar bloqueado, mesmo vindo de outro IP")
	}
	if _, blocked := l.Blocked(from("203.0.113.77"), "outra-pessoa@example.test"); blocked {
		t.Fatal("outro e-mail, de outro IP, não deveria ser afetado")
	}
}

func TestSuccessBeforeTheLimitResetsTheEmailCounterOnly(t *testing.T) {
	l, _ := newTestLimiter(5, 15*time.Minute, 15*time.Minute)
	r := from("198.51.100.8")
	for i := 0; i < 4; i++ {
		l.Fail(r, "a@example.test")
	}
	l.Success(r, "a@example.test")
	// o contador do e-mail zerou: mais 4 falhas por IPs diferentes não bloqueiam o e-mail
	for i := 0; i < 4; i++ {
		l.Fail(from("203.0.113."+string(rune('1'+i))), "a@example.test")
	}
	if _, blocked := l.Blocked(from("192.0.2.200"), "a@example.test"); blocked {
		t.Fatal("o sucesso deveria ter zerado o contador do e-mail")
	}
	// o contador do IP não zera: a 5ª falha do IP original o bloqueia
	l.Fail(r, "b@example.test")
	if _, blocked := l.Blocked(r, "qualquer@example.test"); !blocked {
		t.Fatal("o contador do IP deveria ter continuado em 4")
	}
}

func TestFailuresOutsideTheWindowDoNotAccumulate(t *testing.T) {
	l, c := newTestLimiter(3, time.Minute, time.Minute)
	r := from("198.51.100.8")
	for i := 0; i < 5; i++ {
		l.Fail(r, "a@example.test")
		c.advance(2 * time.Minute) // cada falha cai numa janela nova
	}
	if _, blocked := l.Blocked(r, "a@example.test"); blocked {
		t.Fatal("falhas espaçadas não deveriam bloquear")
	}
}

func TestCounterRestartsAfterLockoutEvenWhenLockoutIsShorterThanWindow(t *testing.T) {
	l, c := newTestLimiter(3, 10*time.Minute, time.Minute)
	r := from("198.51.100.8")
	for i := 0; i < 3; i++ {
		l.Fail(r, "a@example.test")
	}
	c.advance(2 * time.Minute) // bloqueio venceu, mas a janela de falhas ainda não
	l.Fail(r, "a@example.test")
	if _, blocked := l.Blocked(r, "a@example.test"); blocked {
		t.Fatal("uma única falha após o bloqueio não deveria bloquear de novo")
	}
}

func TestTrustedProxyKeepsClientsApart(t *testing.T) {
	proxy := netip.MustParsePrefix("10.0.0.0/8")
	viaProxy := func(client string) *http.Request {
		r := from("10.0.0.5")
		r.Header.Set("X-Forwarded-For", client)
		return r
	}
	// sem confiar no proxy, todos compartilham a chave do proxy
	l, _ := newTestLimiter(3, time.Minute, time.Minute)
	for i := 0; i < 3; i++ {
		l.Fail(viaProxy("198.51.100.1"), "a@example.test")
	}
	if _, blocked := l.Blocked(viaProxy("198.51.100.2"), "b@example.test"); !blocked {
		t.Fatal("sem proxy confiável, o IP do proxy é a chave")
	}
	// confiando, cada cliente tem a sua
	l, _ = newTestLimiter(3, time.Minute, time.Minute)
	l.WithTrustedProxies([]netip.Prefix{proxy})
	for i := 0; i < 3; i++ {
		l.Fail(viaProxy("198.51.100.1"), "a@example.test")
	}
	if _, blocked := l.Blocked(viaProxy("198.51.100.1"), "c@example.test"); !blocked {
		t.Fatal("o cliente que errou deveria estar bloqueado")
	}
	if _, blocked := l.Blocked(viaProxy("198.51.100.2"), "b@example.test"); blocked {
		t.Fatal("outro cliente atrás do mesmo proxy não deveria ser bloqueado")
	}
}

func TestSweepDropsExpiredEntries(t *testing.T) {
	l, c := newTestLimiter(5, time.Minute, time.Minute)
	for i := 0; i < 50; i++ {
		l.Fail(from("198.51.100."+string(rune('0'+i%10))), "u"+string(rune('a'+i%26))+"@example.test")
	}
	if l.Tracked() == 0 {
		t.Fatal("deveria haver entradas")
	}
	c.advance(5 * time.Minute)
	l.Blocked(from("203.0.113.1"), "novo@example.test") // dispara a varredura
	if l.Tracked() != 0 {
		t.Fatalf("entradas expiradas deveriam sair do mapa: %d", l.Tracked())
	}
}

func TestLimiterIsSafeForConcurrentUse(t *testing.T) {
	l := NewLoginLimiter(pepper, 5, time.Minute, time.Minute)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := from("198.51.100.8")
			for i := 0; i < 200; i++ {
				l.Fail(r, "a@example.test")
				l.Blocked(r, "a@example.test")
				l.Success(r, "a@example.test")
			}
		}()
	}
	wg.Wait()
}
