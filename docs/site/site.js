"use strict";

const input = document.getElementById("site-search");
const results = document.getElementById("search-results");
const statusLine = document.getElementById("search-status");
let entries = [];

function normalize(value) {
  return String(value || "").toLowerCase();
}

function tokens(value) {
  return normalize(value).split(/\s+/).filter(Boolean);
}

function clearResults() {
  while (results.firstChild) {
    results.removeChild(results.firstChild);
  }
}

function renderResult(entry) {
  const item = document.createElement("li");
  const category = document.createElement("span");
  const link = document.createElement("a");
  const summary = document.createElement("p");

  category.textContent = entry.category;
  link.href = entry.url;
  link.textContent = entry.title;
  summary.textContent = String(entry.text || "").slice(0, 220);

  item.appendChild(category);
  item.appendChild(link);
  item.appendChild(summary);
  results.appendChild(item);
}

function render(query) {
  clearResults();
  const queryTokens = tokens(query);
  const matches = entries.filter((entry) => {
    if (queryTokens.length === 0) {
      return entry.category === "api" || entry.category === "package-status" || entry.category === "migration";
    }
    const haystack = normalize([entry.title, entry.category, entry.text].join(" "));
    return queryTokens.every((token) => haystack.includes(token));
  }).slice(0, 30);

  statusLine.textContent = matches.length + " result" + (matches.length === 1 ? "" : "s");
  matches.forEach(renderResult);
}

fetch("search-index.json", { credentials: "same-origin" })
  .then((response) => {
    if (!response.ok) {
      throw new Error("search index request failed");
    }
    return response.json();
  })
  .then((data) => {
    entries = Array.isArray(data) ? data : [];
    render("");
  })
  .catch(() => {
    statusLine.textContent = "Search index unavailable";
  });

input.addEventListener("input", () => render(input.value));
