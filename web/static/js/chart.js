// Gráfico de cliques por dia (uPlot). Lê o JSON ao lado do contêiner [data-chart]
// e redesenha após trocas do HTMX. Sem uPlot, a tabela alternativa segue visível.
(function () {
  "use strict";

  function css(name, fallback) {
    var v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    return v || fallback;
  }

  function draw(box) {
    var holder = box.closest(".chart-box");
    var script = holder && holder.querySelector("[data-chart-data]");
    if (!script || typeof uPlot === "undefined") return;
    if (box._plot) { box._plot.destroy(); box._plot = null; }
    box.innerHTML = "";
    var data;
    try { data = JSON.parse(script.textContent); } catch (e) { return; }
    if (!data.days || !data.days.length) return;
    var xs = data.days.map(function (d) {
      var p = d.split("-");
      return Date.UTC(+p[0], +p[1] - 1, +p[2]) / 1000;
    });
    var ink = css("--muted", "#5b6475"), line = css("--line", "#d9dde5");
    var opts = {
      width: Math.max(box.clientWidth, 280),
      height: 260,
      // Os dias são civis (REPORT_TZ): exibir em UTC evita deslocar o rótulo.
      tzDate: function (ts) { return uPlot.tzDate(new Date(ts * 1e3), "Etc/UTC"); },
      legend: { show: true },
      cursor: { drag: { x: false, y: false } },
      axes: [
        { stroke: ink, grid: { stroke: line, width: 1 }, ticks: { stroke: line } },
        { stroke: ink, grid: { stroke: line, width: 1 }, ticks: { stroke: line }, size: 48 },
      ],
      series: [
        {},
        { label: "Cliques", stroke: css("--brand", "#2563eb"), width: 2, points: { show: xs.length <= 31 } },
        { label: "Únicos", stroke: css("--ok", "#067647"), width: 2, dash: [6, 4], points: { show: xs.length <= 31 } },
      ],
    };
    box._plot = new uPlot(opts, [xs, data.clicks, data.unique], box);
  }

  function drawAll() { document.querySelectorAll("[data-chart]").forEach(draw); }

  document.addEventListener("DOMContentLoaded", drawAll);
  document.addEventListener("htmx:afterSettle", drawAll);
  window.addEventListener("resize", function () {
    document.querySelectorAll("[data-chart]").forEach(function (box) {
      if (box._plot) box._plot.setSize({ width: Math.max(box.clientWidth, 280), height: 260 });
    });
  });
  // Troca de tema (classe .dark/.light no <html>): as cores vêm de variáveis CSS.
  new MutationObserver(drawAll).observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
})();
