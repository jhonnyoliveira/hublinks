// Cola do painel: CSRF nas requisições HTMX, modal nativo e toasts.
(function () {
  "use strict";

  document.addEventListener("htmx:configRequest", function (event) {
    var meta = document.querySelector('meta[name="csrf-token"]');
    if (meta) event.detail.headers["X-CSRF-Token"] = meta.content;
  });

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
  document.addEventListener("close", function (event) {
    if (event.target && event.target.id === "modal") document.getElementById("modal-body").innerHTML = "";
  }, true);

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

  // Marca o item de navegação da página atual.
  document.addEventListener("DOMContentLoaded", function () {
    document.querySelectorAll('nav[aria-label="Principal"] a').forEach(function (a) {
      if (location.pathname === a.getAttribute("href") || location.pathname.indexOf(a.getAttribute("href") + "/") === 0) a.setAttribute("aria-current", "page");
    });
  });
})();
