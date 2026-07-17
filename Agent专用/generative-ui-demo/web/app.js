const form = document.querySelector("#generator-form");
const promptInput = document.querySelector("#prompt");
const submitButton = document.querySelector("#submit");
const preview = document.querySelector("#preview");
const jsonOutput = document.querySelector("#json-output");

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  submitButton.disabled = true;
  submitButton.textContent = "MiniMax 正在设计…";

  try {
    const response = await fetch("/api/generate", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ prompt: promptInput.value.trim() }),
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || "生成失败");

    jsonOutput.textContent = JSON.stringify(data, null, 2);
    renderPage(data);
  } catch (error) {
    preview.replaceChildren(messageNode(`生成失败：${error.message}`, "error"));
  } finally {
    submitButton.disabled = false;
    submitButton.textContent = "生成 UI";
  }
});

function renderPage(page) {
  preview.classList.remove("empty");
  preview.style.setProperty("--page-bg", safeColor(page.background, "#10131a"));
  preview.style.setProperty("--accent", safeColor(page.accent, "#7c5cff"));
  preview.replaceChildren();

  const pageRoot = document.createElement("div");
  pageRoot.className = "generated-page";
  pageRoot.setAttribute("aria-label", page.title || "Generated UI");

  const renderers = {
    hero: renderHero,
    text: renderText,
    card: renderCard,
    stat: renderStat,
    button: renderButton,
  };

  for (const component of (page.components || []).slice(0, 8)) {
    const renderer = renderers[component.type];
    if (renderer) pageRoot.append(renderer(component));
  }
  preview.append(pageRoot);
}

function renderHero(component) {
  const node = document.createElement("header");
  node.className = "ui-hero";
  node.append(textElement("p", component.label, "ui-kicker"));
  node.append(textElement("h2", component.title));
  node.append(textElement("p", component.text, "ui-copy"));
  return node;
}

function renderText(component) {
  return textElement("p", component.text || component.title, "ui-text");
}

function renderCard(component) {
  const node = document.createElement("article");
  node.className = "ui-card";
  node.append(textElement("h3", component.title));
  node.append(textElement("p", component.text, "ui-copy"));
  return node;
}

function renderStat(component) {
  const node = document.createElement("article");
  node.className = "ui-stat";
  node.append(textElement("p", component.label || component.title, "ui-kicker"));
  node.append(textElement("strong", component.value));
  node.append(textElement("p", component.text, "ui-copy"));
  return node;
}

function renderButton(component) {
  const node = document.createElement("button");
  node.className = "ui-button";
  node.type = "button";
  node.textContent = component.label || component.title || "继续";
  node.addEventListener("click", () => alert(`你点击了：${node.textContent}`));
  return node;
}

function textElement(tag, value = "", className = "") {
  const node = document.createElement(tag);
  node.textContent = value;
  if (className) node.className = className;
  return node;
}

function messageNode(text, className) {
  const node = textElement("p", text, className);
  return node;
}

function safeColor(value, fallback) {
  return /^#[0-9a-f]{6}$/i.test(value || "") ? value : fallback;
}
