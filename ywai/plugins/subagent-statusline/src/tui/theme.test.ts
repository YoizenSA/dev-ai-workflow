import { describe, expect, test } from "bun:test";

import { FALLBACK_COLORS, resolveTuiTheme } from "./theme.js";

describe("resolveTuiTheme", () => {
  test("returns neutral defaults for missing or non-object input", () => {
    for (const input of [undefined, null, 42, "nope", []]) {
      const theme = resolveTuiTheme(input);
      expect(theme.success).toBe(FALLBACK_COLORS.success);
      expect(theme.error).toBe(FALLBACK_COLORS.error);
      expect(theme.warning).toBe(FALLBACK_COLORS.warning);
      expect(theme.text).toBe(FALLBACK_COLORS.text);
      expect(theme.textMuted).toBe(FALLBACK_COLORS.textMuted);
      expect(theme.accent).toBe(FALLBACK_COLORS.accent);
      expect(theme.backgroundPanel).toBeUndefined();
      expect(theme.backgroundElement).toBeUndefined();
    }
  });

  test("maps matching v2 fields directly", () => {
    const theme = resolveTuiTheme({
      success: "#0f0",
      error: "#f00",
      warning: "#ff0",
      text: "#fff",
      textMuted: "#888",
      accent: "#abc",
    });

    expect(theme).toEqual({
      success: "#0f0",
      error: "#f00",
      warning: "#ff0",
      text: "#fff",
      textMuted: "#888",
      accent: "#abc",
      backgroundPanel: undefined,
      backgroundElement: undefined,
    });
  });

  test("falls back from accent to primary", () => {
    expect(resolveTuiTheme({ primary: "#123456" }).accent).toBe("#123456");
    // An explicit accent wins over primary.
    expect(
      resolveTuiTheme({ accent: "#abcdef", primary: "#123456" }).accent,
    ).toBe("#abcdef");
  });

  test("falls back backgroundElement → backgroundPanel → background", () => {
    expect(
      resolveTuiTheme({ backgroundPanel: "#111", background: "#000" })
        .backgroundElement,
    ).toBe("#111");
    expect(resolveTuiTheme({ background: "#000" }).backgroundElement).toBe(
      "#000",
    );
    expect(
      resolveTuiTheme({
        backgroundElement: "#222",
        backgroundPanel: "#111",
        background: "#000",
      }).backgroundElement,
    ).toBe("#222");
  });

  test("ignores empty and non-string color values", () => {
    const theme = resolveTuiTheme({
      success: "",
      accent: 42,
      primary: "",
      background: null,
    });
    expect(theme.success).toBe(FALLBACK_COLORS.success);
    expect(theme.accent).toBe(FALLBACK_COLORS.accent);
    expect(theme.backgroundElement).toBeUndefined();
  });
});
