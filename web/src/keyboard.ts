// Ordinary Tab never moves between page controls unless a feature explicitly owns it.
window.addEventListener("keydown", (event: KeyboardEvent) => {
  if (event.key !== "Tab" || event.ctrlKey || event.metaKey || event.altKey ||
      event.isComposing || event.keyCode === 229) return;
  event.preventDefault();
  event.stopImmediatePropagation();
}, true);
