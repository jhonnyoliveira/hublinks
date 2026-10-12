// Proteção contra perda de dados: formulários com data-unsaved avisam antes de o
// usuário sair com alterações não salvas. Delegação de eventos, sem depender da
// ordem de carga do Alpine.
(function () {
  "use strict";

  var MESSAGE = "Há alterações não salvas. Sair desta página?";

  function dirtyForm() { return document.querySelector('form[data-unsaved="true"]'); }

  function formOf(event) {
    var el = event.target;
    return el && el.closest ? el.closest("form[data-unsaved]") : null;
  }

  // Qualquer edição marca o formulário; enviar o formulário o desmarca.
  function mark(event) { var f = formOf(event); if (f) f.dataset.unsaved = "true"; }
  document.addEventListener("input", mark);
  document.addEventListener("change", mark);
  document.addEventListener("submit", function (event) {
    var f = formOf(event);
    if (f) f.dataset.unsaved = "false";
  });

  // Fechar a aba, recarregar ou seguir um link comum.
  window.addEventListener("beforeunload", function (event) {
    if (dirtyForm()) {
      event.preventDefault();
      event.returnValue = "";
    }
  });

  // Navegação por HTMX (abrir um modal, filtrar, paginar) a partir de fora do formulário.
  document.addEventListener("htmx:beforeRequest", function (event) {
    var f = dirtyForm();
    if (!f) return;
    var source = event.detail && event.detail.elt;
    if (source && f.contains(source)) return;
    if (window.confirm(MESSAGE)) f.dataset.unsaved = "false";
    else event.preventDefault();
  });
})();
