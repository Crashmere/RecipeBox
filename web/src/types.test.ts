import { describe, it, expect, vi, afterEach } from "vitest";
import { blank, expiryDays, expiryText, today } from "./types";
afterEach(() => vi.useRealTimers());
describe("Pantry calendar dates", () => {
  it("compares local calendar days including month rollover", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 8, 30, 23, 59));
    expect(today()).toBe("2026-09-30");
    expect(expiryDays("2026-10-01")).toBe(1);
    expect(expiryDays("2026-09-30")).toBe(0);
    expect(expiryDays("2026-09-29")).toBe(-1);
    expect(expiryDays("")).toBe(null);
  });
  it("distinguishes unknown, today, expired, and consumed stock", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 8, 19, 12));
    const e = blank("pantry");
    expect(expiryText(e)).toBe("未设置到期日");
    e.expiryDate = "2026-09-19";
    expect(expiryText(e)).toBe("今天到期");
    e.expiryDate = "2026-09-18";
    expect(expiryText(e)).toBe("已过期 1 天");
    e.quantity = 0;
    expect(expiryText(e)).toBe("已吃完");
  });
});
