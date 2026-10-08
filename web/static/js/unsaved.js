document.addEventListener("input", function (event) {
  const form = event.target.closest("form[data-unsaved]");
  if (form) form.dataset.unsaved = "true";
});
window.addEventListener("beforeunload", function (event) {
  if (document.querySelector("form[data-unsaved][data-unsaved='true']")) {
    event.preventDefault();
    event.returnValue = "";
  }
});
