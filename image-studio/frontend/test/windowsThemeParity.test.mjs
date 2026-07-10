import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";

function rgbToHex(r, g, b) {
  return `#${[r, g, b].map((value) => Number(value).toString(16).padStart(2, "0")).join("")}`;
}

function extractCssToken(source, selector, tokenName) {
  const pattern = new RegExp(`${selector}\\s*\\{[\\s\\S]*?${tokenName}:\\s*(#[0-9a-fA-F]{6})`, "m");
  const match = source.match(pattern);
  assert.ok(match, `missing ${tokenName} in selector ${selector}`);
  return match[1].toLowerCase();
}

function extractGoThemeColor(source, fieldName) {
  const pattern = new RegExp(`${fieldName}:\\s+wailswindows\\.RGB\\((\\d+),\\s*(\\d+),\\s*(\\d+)\\)`);
  const match = source.match(pattern);
  assert.ok(match, `missing ${fieldName} in main.go`);
  return rgbToHex(match[1], match[2], match[3]).toLowerCase();
}

test("windows fluent titlebar token matches native Wails titlebar colors", async () => {
  const css = await readFile(new URL("../src/styles/fluent/_windows-theme.css", import.meta.url), "utf8");
  const mainGo = await readFile(new URL("../../main.go", import.meta.url), "utf8");

  const lightCss = extractCssToken(
    css,
    String.raw`html\[data-platform="windows"\]\[data-ui-family="fluent"\]`,
    "--window-titlebar-bg",
  );
  const darkCss = extractCssToken(
    css,
    String.raw`html\.dark\[data-platform="windows"\]\[data-ui-family="fluent"\]`,
    "--window-titlebar-bg",
  );

  const lightGo = extractGoThemeColor(mainGo, "LightModeTitleBar");
  const darkGo = extractGoThemeColor(mainGo, "DarkModeTitleBar");

  assert.equal(lightCss, lightGo);
  assert.equal(darkCss, darkGo);
  assert.match(css, /\.app-header\s*\{[\s\S]*?background:\s*var\(--window-titlebar-bg\);/m);
  assert.match(css, /\.workspace-bar\s*\{[\s\S]*?background:\s*var\(--window-titlebar-bg\);/m);
});

test("windows appearance changes keep the native Wails titlebar in sync", async () => {
  const themeSource = await readFile(new URL("../src/state/studioStore.shared.ts", import.meta.url), "utf8");

  assert.match(themeSource, /if \(theme === "system"\) WindowSetSystemDefaultTheme\(\);/);
  assert.match(themeSource, /else if \(theme === "dark"\) WindowSetDarkTheme\(\);/);
  assert.match(themeSource, /else WindowSetLightTheme\(\);/);
});
