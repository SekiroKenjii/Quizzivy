import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import {
  ALPHABET,
  CODE_LENGTH,
  format,
  isComplete,
  normalize,
} from "@/features/join/code";

/**
 * §6.1: a code is accepted "with or without the dash and in any case". A
 * student reads it off a poster, a phone message, or a QR code, and every one
 * of those produces a slightly different string.
 */

describe("the join code alphabet", () => {
  it("matches the server's, character for character", () => {
    // The client normalizer is presentation only -- the server normalizes again
    // before hashing -- but a client that DROPS a character the server keeps
    // breaks a perfectly good code. Read from the Go source rather than copied,
    // so a change on either side fails here.
    const go = readFileSync(
      resolve(import.meta.dirname, "../../../../server/internal/join/code.go"),
      "utf8",
    );
    const declared = /const Alphabet = "([^"]+)"/.exec(go);
    expect(declared, "server Alphabet constant not found").not.toBeNull();
    expect(ALPHABET).toBe(declared?.[1]);
  });

  it("excludes the characters that read as each other", () => {
    // §6.1 removes the two confusion groups. `L` stays: with `1` and `I` gone
    // there is nothing left for it to be mistaken for.
    for (const ch of "0O1I") expect(ALPHABET).not.toContain(ch);
    expect(ALPHABET).toContain("L");
    expect(ALPHABET).toHaveLength(32);
  });
});

describe("normalize", () => {
  it("reduces every spelling of one code to the same string", () => {
    const canonical = "K7M3P9QR";
    for (const typed of [
      "K7M3P9QR",
      "K7M3-P9QR",
      "k7m3-p9qr",
      "k7m3 p9qr",
      "  K7M3 - P9QR  ",
      "K7M3–P9QR", // en dash, from a phone keyboard
      "K7M3_P9QR",
      "K7M3.P9QR",
    ]) {
      expect(normalize(typed), typed).toBe(canonical);
    }
  });

  it("is idempotent", () => {
    const once = normalize("k7m3-p9qr");
    expect(normalize(once)).toBe(once);
  });

  it("drops characters outside the alphabet rather than keeping them", () => {
    // A `1` or an `O` is not in the alphabet, so it cannot be part of a code.
    // Keeping it would produce a string that matches nothing, which is the
    // same outcome with a worse-looking field.
    expect(normalize("K7M3-P9Q1")).toBe("K7M3P9Q");
    // Vietnamese text, to show what survives: diacritics go, and so do the
    // letters §6.1 removed -- the `I` of XIN and the `O` of CHÀO.
    expect(normalize("nghé xin chào")).toBe("NGHXNCH");
  });

  it("handles an empty and a junk input without throwing", () => {
    expect(normalize("")).toBe("");
    expect(normalize("!!! ---")).toBe("");
  });
});

describe("format", () => {
  it("groups a full code as XXXX-XXXX", () => {
    expect(format("k7m3p9qr")).toBe("K7M3-P9QR");
    expect(format("K7M3-P9QR")).toBe("K7M3-P9QR");
  });

  it("does not add a dash before there is a second group to separate", () => {
    // Typing forward, character by character: the dash must appear only once
    // it has something on both sides, or the caret jumps around.
    expect(format("K")).toBe("K");
    expect(format("K7M3")).toBe("K7M3");
    expect(format("K7M3P")).toBe("K7M3-P");
  });

  it("ignores anything past the code length", () => {
    expect(format("K7M3P9QRZZZZ")).toBe("K7M3-P9QR");
  });
});

describe("isComplete", () => {
  it("is true only at the full length", () => {
    expect(isComplete("K7M3-P9QR")).toBe(true);
    expect(isComplete("k7m3p9qr")).toBe(true);
    expect(isComplete("K7M3-P9Q")).toBe(false);
    expect(isComplete("")).toBe(false);
    // Eight characters, but one of them is not in the alphabet.
    expect(isComplete("K7M3-P9Q1")).toBe(false);
  });

  it("agrees with the server's code length", () => {
    expect(CODE_LENGTH).toBe(8);
  });
});
