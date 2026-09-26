import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import {
  ALPHABET,
  clean,
  CODE_LENGTH,
  EXAMPLE_CODE,
  format,
  group,
  hasExcluded,
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
    const go = readFileSync(
      resolve(
        import.meta.dirname,
        "../../../../server/internal/modules/classes/domain/joincode.go",
      ),
      "utf8",
    );
    const declared = /const Alphabet = "([^"]+)"/.exec(go);
    expect(declared, "server Alphabet constant not found").not.toBeNull();
    expect(ALPHABET).toBe(declared?.[1]);
  });

  it("excludes the characters that read as each other", () => {
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
    expect(normalize("K7M3-P9Q1")).toBe("K7M3P9Q");
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

describe("clean", () => {
  it("upper-cases and drops everything but letters and digits", () => {
    expect(clean("k7qm-2pxa")).toBe("K7QM2PXA");
    expect(clean(" k7qm – 2pxa ")).toBe("K7QM2PXA");
  });

  it("keeps the characters no code uses, so the field can say so", () => {
    expect(clean("k7q0-2pxi")).toBe("K7Q02PXI");
  });

  it("stops at the code length", () => {
    expect(clean("K7QM2PXAZZZZ")).toBe("K7QM2PXA");
  });
});

describe("hasExcluded", () => {
  it("finds each of 0, O, 1 and I", () => {
    for (const ch of "0O1I") expect(hasExcluded(`K7Q${ch}`), ch).toBe(true);
  });

  it("passes a code spelt from the alphabet", () => {
    expect(hasExcluded("K7QM2PXA")).toBe(false);
    expect(hasExcluded("")).toBe(false);
  });
});

describe("group", () => {
  it("adds the dash with the fifth character", () => {
    expect(group("K7QM")).toBe("K7QM");
    expect(group("K7QM2")).toBe("K7QM-2");
    expect(group("K7Q02PXA")).toBe("K7Q0-2PXA");
  });
});

describe("EXAMPLE_CODE", () => {
  it("is a code the alphabet could produce", () => {
    expect(isComplete(EXAMPLE_CODE)).toBe(true);
    expect(format(EXAMPLE_CODE)).toBe(EXAMPLE_CODE);
  });
});
