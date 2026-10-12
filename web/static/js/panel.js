// Cola do painel: CSRF nas requisições HTMX, modal nativo e toasts.
(function () {
  "use strict";

  // O token CSRF vai em hx-headers no <body> (layouts/admin.html).

  function dialog() { return document.getElementById("modal"); }

  // Conteúdo carregado em #modal-body abre o <dialog> (foco preso e Esc nativos).
  document.addEventListener("htmx:afterSwap", function (event) {
    var d = dialog();
    if (d && event.detail.target && event.detail.target.id === "modal-body" && !d.open) d.showModal();
  });
  document.addEventListener("modal-close", function () {
    var d = dialog();
    if (d && d.open) d.close();
  });
  document.addEventListener("click", function (event) {
    if (event.target.closest("[data-modal-close]")) {
      var d = dialog();
      if (d && d.open) d.close();
    }
  });
  // "close" é assíncrono: se o modal já foi reaberto, não apaga o conteúdo novo.
  document.addEventListener("close", function (event) {
    var d = event.target;
    if (d && d.id === "modal" && !d.open) document.getElementById("modal-body").innerHTML = "";
  }, true);

  // Seletor de data: o campo de texto (dd/mm/aaaa) é o valor enviado; o botão abre
  // o calendário nativo (input date escondido) e devolve o dia escolhido ao texto.
  function brToIso(v) {
    var m = /^(\d{2})\/(\d{2})\/(\d{4})$/.exec(v.trim());
    return m ? m[3] + "-" + m[2] + "-" + m[1] : "";
  }
  document.addEventListener("click", function (event) {
    var btn = event.target.closest("[data-date-picker]");
    if (!btn) return;
    var box = btn.closest(".date-input");
    var native = box.querySelector("[data-date-native]");
    native.value = brToIso(box.querySelector("input[type=text]").value);
    if (native.showPicker) { try { native.showPicker(); } catch (e) { native.focus(); } }
    else native.focus();
  });
  document.addEventListener("change", function (event) {
    var native = event.target.closest && event.target.closest("[data-date-native]");
    if (!native || !native.value) return;
    var p = native.value.split("-");
    var text = native.closest(".date-input").querySelector("input[type=text]");
    text.value = p[2] + "/" + p[1] + "/" + p[0];
    text.dispatchEvent(new Event("input", { bubbles: true }));
    text.focus();
  });

  document.addEventListener("alpine:init", function () {
    var seq = 0;
    Alpine.data("toasts", function () {
      return {
        items: [],
        add: function (detail) {
          if (!detail || !detail.message) return;
          var item = { id: ++seq, message: detail.message, kind: detail.kind || "success" };
          this.items.push(item);
          var self = this;
          setTimeout(function () { self.items = self.items.filter(function (t) { return t.id !== item.id; }); }, 4000);
        },
      };
    });
    // Botão "Copiar": o texto vem de data-copy (nunca interpolado em JavaScript);
    // mostra toast e confirma no próprio botão.
    Alpine.data("copy", function () {
      return {
        copied: false,
        run: function () {
          var self = this;
          var text = this.$el.dataset.copy;
          var done = function () {
            self.copied = true;
            window.dispatchEvent(new CustomEvent("toast", { detail: { message: "Copiado para a área de transferência." } }));
            setTimeout(function () { self.copied = false; }, 2000);
          };
          var fail = function () {
            window.dispatchEvent(new CustomEvent("toast", { detail: { message: "Não foi possível copiar. Selecione e copie manualmente.", kind: "error" } }));
          };
          if (navigator.clipboard && navigator.clipboard.writeText) navigator.clipboard.writeText(text).then(done, fail);
          else fail();
        },
      };
    });
  });

  // Tema: a classe .dark/.light no <html> é definida antes da pintura (theme_init);
  // aqui só alternamos e lembramos a escolha. Sem escolha salva, seguimos o sistema.
  function isDark() { return document.documentElement.classList.contains("dark"); }
  function syncThemeButtons() {
    document.querySelectorAll("[data-theme-toggle]").forEach(function (b) { b.setAttribute("aria-pressed", String(isDark())); });
  }
  function setTheme(dark, persist) {
    var c = document.documentElement.classList;
    c.toggle("dark", dark);
    c.toggle("light", !dark);
    if (persist) { try { localStorage.setItem("theme", dark ? "dark" : "light"); } catch (e) { /* sem armazenamento: vale só nesta página */ } }
    syncThemeButtons();
  }
  document.addEventListener("click", function (event) {
    if (event.target.closest("[data-theme-toggle]")) setTheme(!isDark(), true);
  });
  if (window.matchMedia) {
    window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", function (event) {
      var saved = null;
      try { saved = localStorage.getItem("theme"); } catch (e) { /* ignora */ }
      if (!saved) setTheme(event.matches, false);
    });
  }
  document.addEventListener("DOMContentLoaded", syncThemeButtons);

  // Marca o item de navegação da página atual.
  document.addEventListener("DOMContentLoaded", function () {
    document.querySelectorAll('nav[aria-label="Principal"] a').forEach(function (a) {
      if (location.pathname === a.getAttribute("href") || location.pathname.indexOf(a.getAttribute("href") + "/") === 0) a.setAttribute("aria-current", "page");
    });
  });
})();
