import { describe, expect, it } from "vitest";
import {
  bytesOf,
  limitsSchema,
  limitsValues,
  typedLimit,
  unitFor,
} from "@/features/organization/limits";
import { SAVED } from "./support";

const MIB = 1024 ** 2;
const GIB = 1024 ** 3;

describe("the usage limits fields", () => {
  it("read a size in the unit it fits, and an empty field in the default's unit", () => {
    const values = limitsValues(
      { ...SAVED.limits, storageBytes: 20 * GIB, importBytes: 1536 * MIB, seats: 300 },
      SAVED.limitDefaults,
    );
    expect(values.storageBytes).toEqual({ amount: "20", unit: "GB" });
    expect(values.importBytes).toEqual({ amount: "1536", unit: "MB" });
    expect(values.importPerTeacherBytes).toEqual({ amount: "", unit: "MB" });
    expect(values.databaseBytes).toEqual({ amount: "", unit: "GB" });
    expect(values.seats).toBe("300");
    expect(values.teachers).toBe("");
  });

  it("round a typed size to whole megabytes and take a comma as the decimal mark", () => {
    expect(bytesOf({ amount: "2,5", unit: "GB" })).toBe(2560 * MIB);
    expect(bytesOf({ amount: "0.3", unit: "GB" })).toBe(307 * MIB);
    expect(bytesOf({ amount: "12", unit: "MB" })).toBe(12 * MIB);
    expect(bytesOf({ amount: "2 GB", unit: "GB" })).toBeNaN();
    expect(unitFor(3 * GIB)).toBe("GB");
    expect(unitFor(300 * MIB)).toBe("MB");
    expect(unitFor(null)).toBe("MB");
  });

  it("send null for an empty field and refuse what the server would", () => {
    const empty = limitsValues(SAVED.limits, SAVED.limitDefaults);
    expect(limitsSchema.parse(empty)).toEqual(SAVED.limits);
    for (const bad of [
      { ...empty, seats: "0" },
      { ...empty, seats: "1.5" },
      { ...empty, warnAtPercent: "49" },
      { ...empty, emailsPerDay: "100001" },
      { ...empty, storageBytes: { amount: "0.0001", unit: "MB" as const } },
    ]) {
      expect(limitsSchema.safeParse(bad).success).toBe(false);
    }
    expect(limitsSchema.parse({ ...empty, warnAtPercent: " 90 " }).warnAtPercent).toBe(
      90,
    );
  });

  it("read the value a field holds while it is typed, null until it is one", () => {
    expect(typedLimit("40")).toBe(40);
    expect(typedLimit("")).toBeNull();
    expect(typedLimit("4x")).toBeNull();
    expect(typedLimit({ amount: "1", unit: "GB" })).toBe(GIB);
    expect(typedLimit({ amount: "", unit: "GB" })).toBeNull();
  });
});
